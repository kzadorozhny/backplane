package backplane

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/apesternikov/backplane/src/backplane/stats"

	"github.com/apesternikov/backplane/src/config"
	"github.com/golang/glog"
)

type HandlersMap func(name string) http.Handler

type Vhost struct {
	Cf *config.HttpFrontendVhost
	stats.Counting
	Routes []*Route
}

type Route struct {
	Cf *config.HttpHandler
	stats.Counting
}

type Frontend struct {
	http.Handler
	Cf          *config.HttpFrontend
	srv         *http.Server
	sln         *StoppableListener
	tlsListener net.Listener
	//for stats display only
	stats.Counting
	Vhosts  []*Vhost
	tlsconf *tls.Config
}

func NewFrontend(cf *config.HttpFrontend, backends HandlersMap) (*Frontend, error) {
	hs := &HostSwitch{handlers: make(map[string]http.Handler)}
	chs := &stats.CountersCollectingHandler{Handler: hs}
	f := &Frontend{Cf: cf, Handler: chs, Counting: chs}

	if cf.BindAddress == "" {
		return nil, fmt.Errorf("frontend %s: Bind address is empty", cf.Name)
	}
	for i, vh := range cf.Host {
		vhost := &Vhost{Cf: vh}
		f.Vhosts = append(f.Vhosts, vhost)
		mux := http.NewServeMux()
		cmux := &stats.CountersCollectingHandler{Handler: mux}
		vhost.Counting = cmux
		if vh.Default {
			if hs.defaultHandler != nil {
				return nil, fmt.Errorf("frontend %s host %d: default is already defined", cf.Name, i+1)
			}
			hs.defaultHandler = cmux
		}
		for _, domain := range vh.Domain {
			hs.handlers[strings.ToLower(domain)] = cmux
		}
		for _, hc := range vh.Handler {
			h := backends(hc.BackendName)
			if h == nil {
				return nil, fmt.Errorf("Unknown backend %s", hc.BackendName)
			}
			ch := &stats.CountersCollectingHandler{Handler: h}
			mux.Handle(hc.Path, ch)
			vhost.Routes = append(vhost.Routes, &Route{Cf: hc, Counting: ch})
		}
	}
	f.srv = &http.Server{Handler: f}
	//TODO: configure all backends and routes before serving
	//TODO: handle error (raised if l.Accept errors)

	if f.Cf.SslKey != "" {
		cert, err := tls.X509KeyPair([]byte(f.Cf.SslCert), []byte(f.Cf.SslKey))
		if err != nil {
			return nil, err
		}
		f.tlsconf = &tls.Config{
			NextProtos:   []string{"http/1.1"},
			Certificates: []tls.Certificate{cert},
		}
		f.tlsconf.BuildNameToCertificate()
	}
	return f, nil
}

func (f *Frontend) Listen() error {
	if f.Cf.BindAddress != "" {
		glog.V(2).Infof("frontend listening on http://%s/", f.Cf.BindAddress)
		ln, err := net.Listen("tcp", f.Cf.BindAddress)
		if err != nil {
			return err
		}
		f.sln = NewStoppableListener(ln.(*net.TCPListener))
	}

	if f.tlsconf != nil {
		addr := f.Cf.BindSsl
		if addr == "" {
			addr = ":https"
		}
		glog.V(2).Infof("frontend listening on SSL https://%s/", addr)

		ln, err := net.Listen("tcp", addr)
		if err != nil {
			return err
		}

		//TODO: put it in the struct so it could be actually stopped
		sln := NewStoppableListener(ln.(*net.TCPListener))
		f.tlsListener = tls.NewListener(sln, f.tlsconf)

	}
	return nil
}

func (f *Frontend) Serve() {
	if f.tlsListener != nil {
		go f.srv.Serve(f.tlsListener)
	}
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
