package backplane

import (
	"net/http"

	"github.com/apesternikov/backplane/src/requestlog"
	"github.com/gorilla/context"
)

type key int

var mykey key

// GetRequestLog returns a value for request log associated with http request
func GetRequestLog(r *http.Request) *requestlog.Item {
	if rv := context.Get(r, &mykey); rv != nil {
		return rv.(*requestlog.Item)
	}
	return nil
}

// AppendRequestLog attaches request log record to http request
func AppendRequestLog(r *http.Request) *requestlog.Item {
	it := &requestlog.Item{}
	context.Set(r, &mykey, it)
	return it
}
