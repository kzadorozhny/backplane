package backplane

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/golang/protobuf/proto"

	"github.com/apesternikov/backplane/src/config"
)

func makeMockBackends(t *testing.T) HandlersMap {
	return func(name string) http.Handler {
		f := func(w http.ResponseWriter, r *http.Request) {
			t.Logf("backend %s", name)
			fmt.Fprintf(w, "calling backend %s", name)
		}
		return http.HandlerFunc(f)
	}
}

//build frontend config
func mustFEFromText(textcf string) *config.HttpFrontend {
	cf := &config.HttpFrontend{}
	err := proto.UnmarshalText(textcf, cf)
	if err != nil {
		panic(err)
	}
	return cf
}

type urlTestCase struct {
	Url string
	//returned values
	Code         int
	BodyContains string
}

func (tc *urlTestCase) run(t *testing.T, b http.Handler, i int) {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", tc.Url, nil)
	b.ServeHTTP(w, req)
	if w.Code != tc.Code {
		t.Errorf("testcase %d: expected status %v received %v", i+1, tc.Code, w.Code)
	}
	if !strings.Contains(w.Body.String(), tc.BodyContains) {
		t.Errorf("testcase %d: Unexpected response %v expected %v", i+1, w.Body, tc.BodyContains)
	}
}

func TestDomainsNoDefault(t *testing.T) {
	//TODO: check counters
	b, err := NewFrontend(mustFEFromText(`
		bind_http: ":80"
		host: <
			domain: "one.com"
			domain: "one.net"
			handler: <
				path: "/"
				backend_name: "be1"
				> 
			 >
		`), makeMockBackends(t))
	if err != nil {
		t.Fatal("Unexpected error: ", err)
	}
	testcases := []urlTestCase{
		urlTestCase{"http://one.com/a/b", http.StatusOK, "calling backend be1"},
		urlTestCase{"http://one.net/a/b", http.StatusOK, "calling backend be1"},
		urlTestCase{"http://www.one.net/a/b", http.StatusForbidden, ""},
	}
	for i, tc := range testcases {
		tc.run(t, b, i)
	}
}

func TestDomainsWithDefault(t *testing.T) {
	//TODO: check counters
	b, err := NewFrontend(mustFEFromText(`
		bind_http: ":80"
		host: <
			default: true
			handler: <
				path: "/"
				backend_name: "be1"
				> 
			 >
		`), makeMockBackends(t))
	if err != nil {
		t.Fatal("Unexpected error: ", err)
	}

	testcases := []urlTestCase{
		urlTestCase{"http://one.com/a/b", http.StatusOK, "calling backend be1"},
		urlTestCase{"http://one.net/a/b", http.StatusOK, "calling backend be1"},
		urlTestCase{"http://www.one.net/a/b", http.StatusOK, "calling backend be1"},
	}
	for i, tc := range testcases {
		tc.run(t, b, i)
	}
}

func TestPaths(t *testing.T) {
	//TODO: check counters
	b, err := NewFrontend(mustFEFromText(`
		bind_http: ":80"
		host: <
			default: true
			handler: <
				path: "/"
				backend_name: "be1"
				> 
			handler: <
				path: "/api/"
				backend_name: "be2"
				> 
			handler: <
				path: "/static/"
				backend_name: "be3"
				> 
			 >
		`), makeMockBackends(t))
	if err != nil {
		t.Fatal("Unexpected error: ", err)
	}

	testcases := []urlTestCase{
		urlTestCase{"http://whatever/", http.StatusOK, "calling backend be1"},
		urlTestCase{"http://whatever/somepath", http.StatusOK, "calling backend be1"},
		urlTestCase{"http://whatever/api/call", http.StatusOK, "calling backend be2"},
		urlTestCase{"http://whatever/api", http.StatusMovedPermanently, "/api/"},
		urlTestCase{"http://whatever/static", http.StatusMovedPermanently, "/static/"},
		urlTestCase{"http://whatever/static/file.bin", http.StatusOK, "calling backend be3"},
	}
	for i, tc := range testcases {
		tc.run(t, b, i)
	}
}
