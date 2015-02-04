package backplane

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

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
	cf       *config.HttpBackend
	proxy    http.Handler
	balancer *Balancer
}

func (b *Backend) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	glog.V(3).Infof("Backend %s serving %s %s", b.cf.Name, r.Host, r.URL)
	b.proxy.ServeHTTP(w, r)
}

func NewBackend(cf *config.HttpBackend) (*Backend, error) {
	balancer := NewBalancer(cf)
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = req.Host
		},
		Transport: balancer,
	}
	b := &Backend{cf: cf, proxy: proxy, balancer: balancer}
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

func NewBalancer(cf *config.HttpBackend) *Balancer {
	b := &Balancer{cf: cf}
	for _, scf := range cf.Server {
		b.handlers = append(b.handlers, transportForBackend(scf.Address))
	}
	return b
}

type HandlersMap func(name string) http.Handler

type Frontend struct {
	hs  HostSwitch
	cf  *config.HttpFrontend
	srv *http.Server
	sln *StoppableListener
}

func (f *Frontend) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.hs.ServeHTTP(w, r)
}

func NewFrontend(cf *config.HttpFrontend, backends HandlersMap) (*Frontend, error) {
	f := &Frontend{cf: cf, hs: HostSwitch{handlers: make(map[string]http.Handler)}}

	if cf.BindAddress == "" {
		return nil, fmt.Errorf("frontend %s: Bind address is empty", cf.Name)
	}
	for i, vh := range cf.Host {
		mux := http.NewServeMux()
		if vh.Default {
			if f.hs.defaultHandler != nil {
				return nil, fmt.Errorf("frontend %s host %d: default is already defined", cf.Name, i+1)
			}
			f.hs.defaultHandler = mux
		}
		for _, domain := range vh.Domain {
			f.hs.handlers[strings.ToLower(domain)] = mux
		}
		for _, hc := range vh.Handler {
			h := backends(hc.BackendName)
			if h == nil {
				return nil, fmt.Errorf("Unknown backend %s", hc.BackendName)
			}
			mux.Handle(hc.Path, h)
		}
	}
	//install routes here
	// mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {})
	f.srv = &http.Server{Handler: &f.hs}
	//TODO: configure all backends and routes before serving
	//TODO: handle error (raised if l.Accept errors)
	return f, nil
}

func (f *Frontend) Listen() error {
	glog.V(2).Infof("frontend listening on %s", f.cf.BindAddress)
	ln, err := net.Listen("tcp", f.cf.BindAddress)
	if err != nil {
		return err
	}
	f.sln = NewStoppableListener(ln.(*net.TCPListener))
	return nil
}

func (f *Frontend) Serve() {
	f.srv.Serve(f.sln)
}

func (f *Frontend) Stop() {
	f.sln.Stop(false)
}

// We need an object that implements the http.Handler interface.
// Therefore we need a type for which we implement the ServeHTTP method.
// We just use a map here, in which we map host names (with port) to http.Handlers
// TODO: how about uppercase hosts?
type HostSwitch struct {
	handlers       map[string]http.Handler
	defaultHandler http.Handler
}

// Implement the ServerHTTP method on our new type
func (hs *HostSwitch) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	glog.V(3).Infof("HostSwitch serving request %+v", r)
	handler := hs.handlers[r.Host]
	switch {
	case handler != nil:
		handler.ServeHTTP(w, r)

	case hs.defaultHandler != nil:
		hs.defaultHandler.ServeHTTP(w, r)
	default:
		http.Error(w, "Forbidden", 403) // Or Redirect?
	}
}

type Backplane struct {
	mu        sync.Mutex
	backends  map[string]*Backend
	frontends []*Frontend
}

func (bp *Backplane) Configure(cf *config.Config) error {
	backends := make(map[string]*Backend)
	for _, cf := range cf.HttpBackend {
		newb, err := NewBackend(cf)
		if err != nil {
			glog.Errorf("Unable to create new backend %s: %s", cf.Name, err)
			continue
		}
		backends[cf.Name] = newb
	}

	frontends := make([]*Frontend, 0, len(cf.HttpFrontend))
	for _, cf := range cf.HttpFrontend {
		newf, err := NewFrontend(cf, func(name string) http.Handler { return backends[name] })
		if err != nil {
			glog.Errorf("Unable to create new frontend %s: %s", cf.Name, err)
			continue
		}
		frontends = append(frontends, newf)
	}

	for _, f := range frontends {
		f.Listen()
		go f.Serve()
	}
	bp.backends = backends
	bp.frontends = frontends
	return nil
}

func (bp *Backplane) handleStats(w http.ResponseWriter, req *http.Request) {

}
