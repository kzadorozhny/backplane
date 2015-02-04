package backplane

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHostswitchNoDefault(t *testing.T) {
	hs := HostSwitch{handlers: make(map[string]http.Handler)}
	var c1, c2 bool
	hs.handlers["one"] = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		t.Log("one")
		c1 = true
	})
	hs.handlers["two"] = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		t.Log("two")
		c2 = true
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "http://one/a/b", nil)
	hs.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expecting status %v", http.StatusOK)
	}
	if !(c1 && !c2) {
		t.Error("handler one should be called")
	}
	c1 = false

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "http://two/a/b", nil)
	hs.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expecting status %v", http.StatusOK)
	}
	if !(!c1 && c2) {
		t.Error("handler two should be called")
	}
	c2 = false

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "http://three/a/b", nil)
	hs.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("expecting status %v", http.StatusForbidden)
	}
	if !(!c1 && !c2) {
		t.Error("handler default should be called")
	}

}

func TestHostswitchWithDefault(t *testing.T) {
	hs := HostSwitch{handlers: make(map[string]http.Handler)}
	var cd, c1, c2 bool
	hs.defaultHandler = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		t.Log("default")
		cd = true
	})
	hs.handlers["one"] = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		t.Log("one")
		c1 = true
	})
	hs.handlers["two"] = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		t.Log("two")
		c2 = true
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "http://one/a/b", nil)
	hs.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expecting status %v", http.StatusOK)
	}
	if !(c1 && !c2 && !cd) {
		t.Error("handler one should be called")
	}
	c1 = false

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "http://two/a/b", nil)
	hs.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expecting status %v", http.StatusOK)
	}
	if !(!c1 && c2 && !cd) {
		t.Error("handler two should be called")
	}
	c2 = false

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "http://three/a/b", nil)
	hs.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expecting status %v", http.StatusOK)
	}
	if !(!c1 && !c2 && cd) {
		t.Error("handler default should be called")
	}
	cd = false

}
