package backplane

import (
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/apesternikov/backplane/src/backplane/stats"
	"github.com/apesternikov/backplane/src/config"
	"github.com/golang/glog"
)

// transport used by backends. test could set a mock implementation
var transportForBackend func(addr string) http.RoundTripper = func(addr string) http.RoundTripper {
	proxyurl := &url.URL{Scheme: "http", Host: addr}
	return &http.Transport{
		Proxy: http.ProxyURL(proxyurl),
		Dial: (&net.Dialer{
			Timeout:   3 * time.Second,
			KeepAlive: 30 * time.Second,
		}).Dial,
		TLSHandshakeTimeout:   10 * time.Second, //we are not using TLS here, but keep it to avoid surprises later
		ResponseHeaderTimeout: 30 * time.Second, //backend server timeout. TODO: make configurable
	}
}

type Backend struct {
	Cf       *config.HttpBackend
	proxy    http.Handler
	balancer *Balancer
	Servers  []*Server
}

func (b *Backend) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	glog.V(3).Infof("Backend %s serving %s %s", b.Cf.Name, r.Host, r.URL)
	b.proxy.ServeHTTP(w, r)
}

func NewBackend(cf *config.HttpBackend) (*Backend, error) {
	balancer, servers := NewBalancer(cf)
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = req.Host
		},
		Transport: balancer,
	}
	b := &Backend{Cf: cf, proxy: proxy, balancer: balancer, Servers: servers}
	return b, nil
}

func (b *Backend) Stop() {
	//TODO: drain and close all transports
}

// Balancer implements http.RoundTripper and routes requests to configured backend servers
type Balancer struct {
	cf       *config.HttpBackend
	handlers []http.RoundTripper
	idx      int64
}

func (b *Balancer) RoundTrip(r *http.Request) (*http.Response, error) {
	idx := atomic.AddInt64(&b.idx, 1)
	glog.V(3).Infof("Balancer serving %v using %d", r.URL, idx%int64(len(b.handlers)))
	h := b.handlers[idx%int64(len(b.handlers))]
	//TODO: handle error and redirect to another server
	return h.RoundTrip(r)
}

func NewBalancer(cf *config.HttpBackend) (b *Balancer, servers []*Server) {
	b = &Balancer{cf: cf}
	servers = make([]*Server, 0, len(cf.Server))
	for _, scf := range cf.Server {
		s := NewServer(scf)
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
}

func NewServer(cf *config.Server) *Server {
	t := transportForBackend(cf.Address)
	ct := &stats.CountersCollectingRoundTripper{RoundTripper: t}
	//TODO: insert limiters here
	return &Server{
		Cf:           cf,
		RoundTripper: ct,
		Counting:     ct,
	}
}
