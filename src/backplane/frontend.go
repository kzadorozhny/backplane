package backplane

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/bradfitz/http2"

	"github.com/apesternikov/backplane/src/backplane/stats"

	"github.com/apesternikov/backplane/src/config"
	"github.com/golang/glog"
	"github.com/gorilla/context"
)

const FIXME_RATE_LIMIT = 100000

type HandlersMap func(name string) http.Handler

type Vhost struct {
	Cf *config.HttpFrontendVhost
	stats.Counting
	RateLimiter *stats.EMARateLimiter
	Routes      []*Route
}

type Route struct {
	Cf *config.HttpHandler
	stats.Counting
	RateLimiter *stats.EMARateLimiter
}

type Frontend struct {
	Handler     http.Handler
	Cf          *config.HttpFrontend
	srv         *http.Server
	Sln, TlsSln *StoppableListener
	tlsListener net.Listener
	//for stats display only
	stats.Counting
	RateLimiter *stats.EMARateLimiter
	Vhosts      []*Vhost
	tlsconf     *tls.Config
}

type statsCollectingResponseWriter struct {
	wrapped      http.ResponseWriter
	ResponseCode int
	ResponseSize int
}

func (s *statsCollectingResponseWriter) Header() http.Header {
	return s.wrapped.Header()
}
func (s *statsCollectingResponseWriter) Write(data []byte) (int, error) {
	sz, err := s.wrapped.Write(data)
	s.ResponseSize = s.ResponseSize + sz
	return sz, err
}
func (s *statsCollectingResponseWriter) WriteHeader(code int) {
	s.ResponseCode = code
	s.wrapped.WriteHeader(code)
}

func (f *Frontend) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	resp := statsCollectingResponseWriter{wrapped: w}
	log := AppendRequestLog(req)
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err == nil {
		log.ClientIp = host
	} else {
		log.ClientIp = req.RemoteAddr
	}
	log.TimeTNs = time.Now().UnixNano()
	log.Method = req.Method
	log.RequestUri = req.RequestURI
	log.HttpVersion = req.Proto
	log.Referrer = req.Referer()
	log.UserAgent = req.UserAgent()
	log.Frontend = f.Cf.BindHttp
	log.IsTls = (req.TLS != nil)

	f.Handler.ServeHTTP(&resp, req)

	log.StatusCode = int64(resp.ResponseCode)
	log.ResponseSize = int64(resp.ResponseSize)
	endtime := time.Now().UnixNano()
	log.FrontendLatencyNs = endtime - log.TimeTNs

	context.Clear(req)
}

func NewFrontend(cf *config.HttpFrontend, backends HandlersMap) (*Frontend, error) {
	var err error
	hs := &HostSwitch{handlers: make(map[string]http.Handler)}
	chs := &stats.CountersCollectingHandler{Handler: hs, RateLimiter: stats.NewEMARateLimiter(FIXME_RATE_LIMIT)}
	f := &Frontend{Cf: cf, Handler: chs, Counting: chs, RateLimiter: chs.RateLimiter}

	if cf.BindHttp == "" {
		return nil, fmt.Errorf("frontend %s: Bind address is empty", cf.Name)
	}
	for i, vh := range cf.Host {
		vhost := &Vhost{Cf: vh}
		f.Vhosts = append(f.Vhosts, vhost)
		mux := http.NewServeMux()
		cmux := &stats.CountersCollectingHandler{Handler: mux, RateLimiter: stats.NewEMARateLimiter(FIXME_RATE_LIMIT)}
		vhost.Counting = cmux
		vhost.RateLimiter = cmux.RateLimiter
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
			if hc.Auth != nil {
				h, err = AuthWrapper(hc.Auth, h)
				if err != nil {
					return nil, err
				}
			}
			ch := &stats.CountersCollectingHandler{Handler: h, RateLimiter: stats.NewEMARateLimiter(FIXME_RATE_LIMIT)}
			mux.Handle(hc.Path, ch)
			r := &Route{Cf: hc, Counting: ch, RateLimiter: ch.RateLimiter}
			vhost.Routes = append(vhost.Routes, r)
		}
	}
	f.srv = &http.Server{Handler: f}
	//TODO: handle error (raised if l.Accept errors)
	if len(f.Cf.SslCert) != 0 || f.Cf.SslCertMask != "" {
		f.tlsconf = &tls.Config{
			// NextProtos:   []string{"http/1.1"}, //should be updated after the http/2.0 config
			Certificates: nil, //[]tls.Certificate{},
			MinVersion:   tls.VersionTLS10,
		}
		if f.Cf.SslCertMask != "" {
			f.tlsconf.Certificates, err = LoadCertsByMask(f.Cf.SslCertMask)
			if err != nil {
				return nil, err
			}
		}
		for _, inlinecert := range f.Cf.SslCert {
			cert, err := X509KeyPairFromMem([]byte(inlinecert))
			if err != nil {
				return nil, err
			}
			f.tlsconf.Certificates = append(f.tlsconf.Certificates, cert)
		}
		f.tlsconf.BuildNameToCertificate()
		glog.V(1).Infof("configured TLS certificates: %v", f.tlsconf.NameToCertificate)
		f.srv.TLSConfig = f.tlsconf
		http2.ConfigureServer(f.srv, nil)
		f.tlsconf.NextProtos = append(f.tlsconf.NextProtos, "http/1.1")
	}
	return f, nil
}

func (f *Frontend) Listen() error {
	if f.Cf.BindHttp != "" {
		glog.V(2).Infof("frontend listening on http://%s/", f.Cf.BindHttp)
		ln, err := net.Listen("tcp", f.Cf.BindHttp)
		if err != nil {
			return err
		}
		f.Sln = NewStoppableListener(ln.(*net.TCPListener))
	}

	if f.tlsconf != nil {
		addr := f.Cf.BindHttps
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
		f.TlsSln = sln
		f.tlsListener = tls.NewListener(sln, f.tlsconf)

	}
	return nil
}

func (f *Frontend) Serve() {
	if f.tlsListener != nil {
		go f.srv.Serve(f.tlsListener)
	}
	if f.Sln != nil {
		f.srv.Serve(f.Sln)
	}
}

func (f *Frontend) Stop() {
	f.Sln.Stop(false)
}

// We need an object that implements the http.Handler interface.
// Therefore we need a type for which we implement the ServeHTTP method.
// We just use a map here, in which we map host names (with port) to http.Handlers
// TODO: how about uppercase hosts?
type HostSwitch struct {
	handlers       map[string]http.Handler
	defaultHandler http.Handler
}

//TODO: log and count host not found (and not default)

// Implement the ServerHTTP method on our new type
func (hs *HostSwitch) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	glog.V(3).Infof("HostSwitch serving request %+v", r)
	var host string
	if r.TLS != nil {
		host = r.TLS.ServerName
	} else {
		host = r.Host
	}
	sepidx := strings.Index(host, ":")
	if sepidx > 0 {
		host = host[0:sepidx]
	}
	handler := hs.handlers[host]
	switch {
	case handler != nil:
		handler.ServeHTTP(w, r)

	case hs.defaultHandler != nil:
		hs.defaultHandler.ServeHTTP(w, r)
	default:
		http.Error(w, "Forbidden", 403) // Or Redirect?
	}
}
