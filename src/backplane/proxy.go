package backplane

import (
	"html/template"
	"net/http"
	"os"
	"sync"
	"syscall"
	"time"

	"golang.org/x/net/trace"

	"github.com/apesternikov/backplane/src/backplane/static/tpls"

	"github.com/apesternikov/backplane/src/config"
	"github.com/golang/glog"
)

type Backplane struct {
	Backends  []*Backend
	Frontends []*Frontend
}

func (bp *Backplane) Configure(cf *config.Config) error {
	backends := make(map[string]http.Handler)
	Backends := make([]*Backend, 0, len(cf.HttpBackend)+1)
	for _, cf := range cf.HttpBackend {
		newb, err := NewBackend(cf)
		if err != nil {
			glog.Errorf("Unable to create new backend %s: %s", cf.Name, err)
			continue
		}
		backends[cf.Name] = newb
		Backends = append(Backends, newb)
	}

	backends["internalstats"] = http.HandlerFunc(bp.handleStats)

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
		err := f.Listen()
		if err != nil {
			glog.Errorf("Unable to listen: %s", err)
		}
		//TODO: handle listen errors (no permission on port 80 && !root)
		go f.Serve()
	}
	bp.Backends = Backends
	bp.Frontends = frontends
	return nil
}

var funcMap = template.FuncMap{
	"age": func(t time.Time) time.Duration { return time.Now().Sub(t) },
}

var (
	statstpl *template.Template
	tplmux   sync.Mutex
)

func StatsTemplate() *template.Template {
	tplmux.Lock()
	defer tplmux.Unlock()
	changed, err := tpls.Stats_html.Refresh()
	if err != nil {
		panic(err) //only possible in dev mode
	}
	//TODO: race condition
	if statstpl == nil || changed {
		t, err := template.New(tpls.Stats_html.Filename).Funcs(funcMap).Parse(string(tpls.Stats_html.Data))
		if err != nil {
			glog.Errorf("Error parsing template %s: %s", tpls.Stats_html.Filename, err)
			return statstpl //return old value
		}
		statstpl = t
	}
	return statstpl
}

var starttime time.Time

func init() {
	starttime = time.Now()
}

func (bp *Backplane) handleStats(w http.ResponseWriter, req *http.Request) {
	tr := trace.New("backend.internalstats", req.RequestURI)
	defer tr.Finish()
	hostname, err := os.Hostname()
	if err != nil {
		glog.Error("Unable to obtain hostname: ", err)
		tr.LazyPrintf("Unable to obtain hostname: ", err)
	}
	var data = struct {
		Backends                         []*Backend
		Frontends                        []*Frontend
		Pid                              int
		Hostname                         string
		Uptime                           time.Duration
		LimitAs, LimitFsize, LimitNofile syscall.Rlimit
	}{
		Backends:  bp.Backends,
		Frontends: bp.Frontends,
		Pid:       os.Getpid(),
		Hostname:  hostname,
		Uptime:    time.Since(starttime),
	}
	syscall.Getrlimit(syscall.RLIMIT_AS, &data.LimitAs)
	syscall.Getrlimit(syscall.RLIMIT_FSIZE, &data.LimitFsize)
	syscall.Getrlimit(syscall.RLIMIT_NOFILE, &data.LimitNofile)

	err = StatsTemplate().Execute(w, &data)
	if err != nil {
		glog.Errorf("unable to execute template: %s", err)
		tr.LazyPrintf("unable to execute template: %s", err)
	}
}
