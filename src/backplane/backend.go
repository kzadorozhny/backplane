package backplane

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/apesternikov/backplane/src/backplane/stats"
	"github.com/apesternikov/backplane/src/config"
	"github.com/golang/glog"
	"golang.org/x/net/trace"
)

// transport used by backends. test could set a mock implementation
var transportForBackend func(backendaddr string) http.RoundTripper = func(backendaddr string) http.RoundTripper {
	dialer := &net.Dialer{
		Timeout:   3 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	return &http.Transport{
		Dial: func(network, addr string) (net.Conn, error) {
			//ignore address, always connect to the configured backend host
			return dialer.Dial("tcp", backendaddr)
		},
		TLSHandshakeTimeout:   10 * time.Second, //we are not using TLS here, but keep it to avoid surprises later
		ResponseHeaderTimeout: 30 * time.Second, //backend server timeout. TODO: make configurable
	}
}

type Backend struct {
	Cf       *config.HttpBackend
	proxy    http.Handler
	balancer *Balancer
	stats.Counting
	RateLimiter *stats.EMARateLimiter
	Servers     []*Server
}

func (b *Backend) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tr := trace.New("backend."+b.Cf.Name, r.RequestURI)
	tr.LazyPrintf("Request: %#v", r)
	defer tr.Finish()

	glog.V(3).Infof("Backend %s serving %s %s", b.Cf.Name, r.Host)
	log, fetr := GetRequestLogAndTrace(r)
	log.BackendName = b.Cf.Name
	fetr.LazyPrintf("using backend %s", b.Cf.Name)
	defer fetr.LazyPrintf("backend done")

	b.proxy.ServeHTTP(w, r)
	if wr, ok := w.(*stats.StatsCollectingResponseWriter); ok {
		tr.LazyPrintf("Response %d", wr.ResponseCode)
		tr.LazyPrintf("Response headers %v", wr.Header())
		if wr.IsErrorResponse() {
			tr.SetError()
		}
	}
}

func NewBackend(cf *config.HttpBackend) (*Backend, error) {
	balancer, servers := NewBalancer(cf)
	proxy := &ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = req.Host
		},
		Transport: balancer,
	}
	ch := &stats.CountersCollectingHandler{
		Handler:     proxy,
		RateLimiter: stats.NewEMARateLimiter(FIXME_RATE_LIMIT),
	}
	b := &Backend{
		Cf:          cf,
		proxy:       ch,
		balancer:    balancer,
		Counting:    ch,
		RateLimiter: ch.RateLimiter,
		Servers:     servers,
	}
	return b, nil
}

func (b *Backend) Stop() {
	//TODO: drain and close all transports
}

// Balancer implements http.RoundTripper and routes requests to configured backend servers
type Balancer struct {
	cf             *config.HttpBackend
	handlers       []*Server
	mux            sync.Mutex //TODO: profile and decide if atomic ops are feasible
	activeHandlers []*Server
	idx            int64
}

func (b *Balancer) rebuildActive() {
	activeHandlers := make([]*Server, 0, len(b.handlers))
	for _, handler := range b.handlers {
		if handler.IsHealthy() {
			activeHandlers = append(activeHandlers, handler)
		}
	}
	b.mux.Lock()
	b.activeHandlers = activeHandlers
	b.mux.Unlock()
}

var NoHealthyBackendAvailable = errors.New("No healthy backend server available")

func (b *Balancer) RoundTrip(r *http.Request) (*http.Response, error) {
	rlog, tr := GetRequestLogAndTrace(r)
	starttime := time.Now().UnixNano()
	tr.LazyPrintf("balancer")
	defer tr.LazyPrintf("balancer done")
	idx := atomic.AddInt64(&b.idx, 1)
	glog.V(3).Infof("Balancer serving %v using %d", r.URL, idx%int64(len(b.handlers)))
	glog.V(3).Infof("Request %v", r)
	b.mux.Lock()
	if len(b.activeHandlers) == 0 {
		b.mux.Unlock()
		tr.LazyPrintf("No healthy backend server available")
		tr.SetError()
		return nil, NoHealthyBackendAvailable
	}
	h := b.activeHandlers[idx%int64(len(b.activeHandlers))]
	b.mux.Unlock()
	//TODO: handle error and redispatch to another server
	resp, err := h.RoundTrip(r)
	glog.V(3).Infof("Response %v", resp)
	rlog.ServerLatencyNs = time.Now().UnixNano() - starttime
	return resp, err
}

func NewBalancer(cf *config.HttpBackend) (b *Balancer, servers []*Server) {
	b = &Balancer{cf: cf}
	servers = make([]*Server, 0, len(cf.Server))
	for _, scf := range cf.Server {
		s := NewServer(cf.Name, scf, b.rebuildActive)
		b.handlers = append(b.handlers, s)
		servers = append(servers, s)
	}
	return
}

//server is a transport. several servers are balanced by Balancer
type Server struct {
	Cf *config.Server
	//TODO: consider optimizing other places by preconverting interfaces?
	http.RoundTripper
	stats.Counting
	RateLimiter *stats.EMARateLimiter
	HealthChecker
}

func NewServer(backendName string, cf *config.Server, onStateUpdate func()) *Server {
	t := transportForBackend(cf.Address)
	ct := &stats.CountersCollectingRoundTripper{
		RoundTripper: t,
		RateLimiter:  stats.NewEMARateLimiter(FIXME_RATE_LIMIT),
		TraceFamily:  "server." + backendName + "." + cf.Address,
	}
	//TODO: insert limiters here
	//TODO: make prober url configurable
	proberUrl := fmt.Sprintf("http://%s/", cf.Address)
	prober := &HttpHealthChecker{Transport: t, Url: proberUrl, onStateUpdate: onStateUpdate}
	prober.Run()
	return &Server{
		Cf:            cf,
		RoundTripper:  ct,
		Counting:      ct,
		RateLimiter:   ct.RateLimiter,
		HealthChecker: prober,
	}
}

type HealthChecker interface {
	IsHealthy() bool
	HealthStatus() string
	LastStatusChange() time.Time
}

type HttpHealthChecker struct {
	Transport     http.RoundTripper
	Url           string
	ticker        *time.Ticker
	client        *http.Client
	mux           sync.Mutex
	isHealthy     bool
	status        string
	lastChange    time.Time
	onStateUpdate func() // called when server status changed with mutex UNlocked
}

func (h *HttpHealthChecker) IsHealthy() bool {
	h.mux.Lock()
	defer h.mux.Unlock()
	return h.isHealthy
}

func (h *HttpHealthChecker) HealthStatus() string {
	h.mux.Lock()
	defer h.mux.Unlock()
	return h.status
}
func (h *HttpHealthChecker) LastStatusChange() time.Time {
	h.mux.Lock()
	defer h.mux.Unlock()
	return h.lastChange
}

func (h *HttpHealthChecker) runOnce() {
	//TODO: make method configurable
	glog.V(2).Infof("Healthcheck request %s", h.Url)
	starttime := time.Now()
	resp, err := h.client.Head(h.Url)
	endtime := time.Now()
	d := endtime.Sub(starttime)
	glog.V(2).Infof("Healthcheck %s in %s response %s err %s", h.Url, d, resp, err)
	h.mux.Lock()
	oldIsHealthy := h.isHealthy
	switch {
	case err != nil:
		h.status = fmt.Sprintf("error: %s in %s", err, d)
		h.isHealthy = false
	case resp.StatusCode != 200:
		h.status = fmt.Sprintf("error: status %s in %s", resp.Status, d)
		h.isHealthy = false
	default:
		h.status = fmt.Sprintf("status %s in %s", resp.Status, d)
		h.isHealthy = true
	}
	var changed bool
	if oldIsHealthy != h.isHealthy || h.lastChange.IsZero() {
		h.lastChange = endtime
		changed = true
	}
	h.mux.Unlock()
	if changed && h.onStateUpdate != nil {
		h.onStateUpdate()
	}
}

func (h *HttpHealthChecker) Run() {
	//TODO: make prober timeout configurable
	h.client = &http.Client{Transport: h.Transport, Timeout: 5 * time.Second}
	h.ticker = time.NewTicker(10 * time.Second)
	go func() {
		h.runOnce()
		for now := range h.ticker.C {
			glog.V(2).Infof("Healthcheck request at %v", now)
			h.runOnce()
		}
	}()
}

func (h *HttpHealthChecker) Stop() {
	h.ticker.Stop()
}
