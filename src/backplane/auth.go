package backplane

import (
	"errors"
	"net/http"

	"github.com/apesternikov/backplane/src/config"
)

type basicAuthWrapper struct {
	Config  *config.AuthHttpBasicT
	Handler http.Handler
}

func (b *basicAuthWrapper) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	username, password, ok := r.BasicAuth()
	if !ok || b.Config.Userpass[username] != password {
		w.Header().Set("WWW-Authenticate", `Basic realm="`+b.Config.Realm+`"`)
		w.WriteHeader(401)
		w.Write([]byte("401 Unauthorized\n"))
		return
	}
	b.Handler.ServeHTTP(w, r)
}

func AuthWrapper(cf *config.Auth, h http.Handler) (http.Handler, error) {
	switch {
	case cf.HttpBasic != nil:
		return &basicAuthWrapper{Config: cf.HttpBasic, Handler: h}, nil
	default:
		return nil, errors.New("Auth config error")
	}
}
