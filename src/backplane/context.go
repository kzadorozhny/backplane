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

type ctx struct {
	it *requestlog.Item
	tr trace.Trace
}

// GetRequestLog returns a value for request log associated with http request
func GetRequestLogAndTrace(r *http.Request) (rl *requestlog.Item, tr trace.Trace) {
	if rv := context.Get(r, &mykey); rv != nil {
		c := rv.(*ctx)
		return c.it, c.tr
	}
	return nil, nil
}

// AppendRequestLog attaches request log record to http request
func AppendRequestLogAndTrace(r *http.Request, tr trace.Trace) *requestlog.Item {
	c := &ctx{it: &requestlog.Item{}, tr: tr}
	context.Set(r, &mykey, c)
	return c.it
}

var NoSuchContext = errors.New("No such context")

func LinkContext(from, to *http.Request) error {
	if rv := context.Get(from, &mykey); rv != nil {
		context.Set(to, &mykey, rv)
		return nil
	}
	return NoSuchContext
}
