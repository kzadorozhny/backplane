package main

import (
	"flag"
	"net/http"

	"github.com/golang/glog"

	"github.com/apesternikov/backplane/src/config"
	"github.com/apesternikov/backplane/src/swim"
)

func main() {
	flag.Parse()
	config.Configure()
	glog.Infof("node id %s", config.NodeId)
	s, err := swim.NewSwim(config.NodeId)
	if err != nil {
		glog.Fatal("Unable to create swim: ", err)
	}
	if *config.Seed != "" {
		s.AddHosts(*config.Seed)
	}
	http.HandleFunc("/swim", s.HandleStatus)
	go s.Serve()
	glog.Infof("status page http://%s/swim", config.NodeId)
	http.ListenAndServe(config.NodeId, nil)
}
