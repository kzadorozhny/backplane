package backplane

import (
	"errors"
	"net/http"

	"golang.org/x/net/trace"

	"github.com/apesternikov/backplane/src/requestlog"
	"github.com/gorilla/context"
)

type key int

var mykey key

type RequestContext struct {
	Log *requestlog.Item
	Tr  trace.Trace
}

// NewRequestContext attaches request context to http request
func NewRequestContext(r *http.Request, ctx *RequestContext) *RequestContext {
	context.Set(r, &mykey, ctx)
	return ctx
}

// GetRequestContext returns a pointer to context associated with http request
func GetRequestContext(r *http.Request) (ctx *RequestContext) {
	if rv := context.Get(r, &mykey); rv != nil {
		return rv.(*RequestContext)
	}
	return nil
}

var NoSuchContext = errors.New("No such context")

func LinkContext(from, to *http.Request) error {
	if rv := context.Get(from, &mykey); rv != nil {
		context.Set(to, &mykey, rv)
		return nil
	}
	return NoSuchContext
}
