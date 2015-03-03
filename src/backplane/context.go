package backplane

import (
	"net/http"

	"github.com/apesternikov/backplane/src/requestlog"
	"github.com/gorilla/context"
)

type key int

var mykey key

// GetContext returns a value for this package from the request values.
func GetRequestLog(r *http.Request) *requestlog.Item {
	if rv := context.Get(r, &mykey); rv != nil {
		return rv.(*requestlog.Item)
	}
	return nil
}

// SetMyKey sets a value for this package in the request values.
func AppendRequestLog(r *http.Request) *requestlog.Item {
	it := &requestlog.Item{}
	context.Set(r, &mykey, it)
	return it
}
