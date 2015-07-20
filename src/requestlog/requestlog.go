package requestlog

import (
	"flag"
	"log"
	"net/url"

	influx "github.com/influxdb/influxdb/client"
)

//go:generate protoc --go_out=. requestlog.proto

var (
	influxdb_url  = flag.String("influxdb_url", "http://localhost:8086", "URL of influxdb")
	influxdb_user = flag.String("influxdb_user", "", "influxdb user")
	influxdb_pass = flag.String("influxdb_pass", "", "influxdb password")

	cli *influx.Client
)

// We can not run this during init since we need flags to be initialized
func AfterInit() {
	host, err := url.Parse(*influxdb_url)
	if err != nil {
		log.Fatal("Unable to parse influxdb address: ", err)
	}
	conf := influx.Config{
		URL:      *host,
		Username: *influxdb_user,
		Password: *influxdb_pass,
	}
	conn, err := influx.NewClient(conf)
	if err != nil {
		log.Fatal("Unable to create influxdb client: ", err)
	}
	cli = conn
}
