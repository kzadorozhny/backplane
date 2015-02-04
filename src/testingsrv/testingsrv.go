package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
)

func handler1(w http.ResponseWriter, req *http.Request) {
	r, err := httputil.DumpRequest(req, true)
	if err != nil {
		log.Print("error dumping request")
	}
	log.Print(string(r))
	fmt.Fprintf(w, "HANDLER 1\n")
	_, err = w.Write(r)
	if err != nil {
		log.Print("error writing response\n")
	}
}

func handler2(w http.ResponseWriter, req *http.Request) {
	r, err := httputil.DumpRequest(req, true)
	if err != nil {
		log.Print("error dumping request")
	}
	log.Print(string(r))
	fmt.Fprintf(w, "HANDLER 2\n")
	_, err = w.Write(r)
	if err != nil {
		log.Print("error writing response\n")
	}

}

func main() {
	s1 := &http.Server{Handler: http.HandlerFunc(handler1), Addr: "127.0.0.1:9080"}
	s2 := &http.Server{Handler: http.HandlerFunc(handler2), Addr: "127.0.0.1:9081"}
	fmt.Print("Listening on http://127.0.0.1:9080/ and http://127.0.0.1:9081/\n")
	go s1.ListenAndServe()
	s2.ListenAndServe()
}
