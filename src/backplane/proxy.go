package backplane

import (
	"html/template"
	"net/http"
	"time"

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

func (bp *Backplane) handleStats(w http.ResponseWriter, req *http.Request) {
	t, err := template.New("stats.html").Funcs(funcMap).ParseFiles("stats.html")
	if err != nil {
		glog.Errorf("unable to parse template: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	err = t.Execute(w, bp)
	if err != nil {
		glog.Errorf("unable to execute template: %s", err)
	}
}
