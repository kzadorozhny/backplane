package main

import (
	"flag"
	"io/ioutil"

	"github.com/golang/glog"

	"github.com/apesternikov/backplane/src/backplane"
	"github.com/apesternikov/backplane/src/config"
)

var cf = flag.String("c", "backplaned.conf", "Config file location")

func main() {
	flag.Parse()
	glog.Infof("using config file %s", *cf)
	textcf, err := ioutil.ReadFile(*cf)
	if err != nil {
		glog.Fatalf("Unable to read config file: %s", err)
	}
	cfg, err := config.FromText(string(textcf))
	if err != nil {
		glog.Fatalf("Unable to parse config file: %s", err)
	}
	b := &backplane.Backplane{}
	err = b.Configure(cfg)
	if err != nil {
		glog.Fatalf("Unable to create backplane: %s", err)
	}

	var done chan struct{}
	<-done
}
