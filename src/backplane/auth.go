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
	if ok {
		storedpass, ok := b.Config.Userpass[username]
		if ok && storedpass == password {
			b.Handler.ServeHTTP(w, r)
			return
		}
	}
	w.Header().Set("WWW-Authenticate", `Basic realm="`+b.Config.Realm+`"`)
	w.WriteHeader(401)
	w.Write([]byte("401 Unauthorized\n"))
	return
}

func AuthWrapper(cf *config.Auth, h http.Handler) (http.Handler, error) {
	switch {
	case cf.HttpBasic != nil:
		return &basicAuthWrapper{Config: cf.HttpBasic, Handler: h}, nil
	default:
		return nil, errors.New("Auth config error")
	}
}
