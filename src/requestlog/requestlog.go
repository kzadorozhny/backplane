package requestlog

import (
	"errors"
	"flag"
	"log"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/apesternikov/backplane/src/backoff"

	"github.com/golang/glog"

	influx "github.com/influxdb/influxdb/client"
)

//go:generate protoc --go_out=. requestlog.proto

var (
	influxdb_url      = flag.String("influxdb_url", "http://localhost:8086", "URL of influxdb")
	influxdb_database = flag.String("influxdb_database", "backplane", "influxdb database for access log")
	influxdb_user     = flag.String("influxdb_user", "", "influxdb user")
	influxdb_pass     = flag.String("influxdb_pass", "", "influxdb password")

	cli *influx.Client

	logbuf   chan *Item = make(chan *Item, 10000)
	hostname string     = "unknown"
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
	hostname, err = os.Hostname()
	if err != nil {
		glog.Error("Unable to obtain hostname ", err)
	}
	go LogWriter()
}

var BufferOverflow = errors.New("accesslog buffer overflow")

func SubmitLog(logitem *Item) error {
	select {
	case logbuf <- logitem:
		return nil
	default:
		glog.Error("accesslog buffer overflow, dropped %s", logitem)
		return BufferOverflow
	}
}

func itemToInfluxPoint(it *Item, pt *influx.Point) {
	if len(pt.Measurement) == 0 {
		pt.Measurement = "accesslog"
	}
	pt.Tags = map[string]string{
		"hostname": hostname,
	}
	if it.IsTls {
		pt.Tags["IsTls"] = ""
	}
	pt.Fields = map[string]interface{}{
		"BackendName":     it.BackendName,
		"ClientIp":        it.ClientIp,
		"Frontend":        it.Frontend,
		"HandlerPath":     it.HandlerPath,
		"HttpVersion":     it.HttpVersion,
		"Method":          it.Method,
		"Referrer":        it.Referrer,
		"RequestUri":      it.RequestUri,
		"ResponseSize":    strconv.Itoa(int(it.ResponseSize)),
		"ServerAddress":   it.ServerAddress,
		"StatusCode":      strconv.Itoa(int(it.StatusCode)),
		"UserAgent":       it.UserAgent,
		"Vhost":           it.Vhost,
		"FrontendLatency": it.FrontendLatencyNs,
		"ServerLatency":   it.ServerLatencyNs,
	}
	pt.Time = time.Unix(0, it.TimeTNs)
}

func LogWriter() {
	// we are writing in batches of up to 100
	var pointsbuf [100]influx.Point
	for {
		//read 1 item
		item := <-logbuf
		itemToInfluxPoint(item, &pointsbuf[0])
		//try to read up to 99 records in the next 100 ms
		t := time.NewTimer(time.Millisecond * 300)
		l := 1
	OuterLoop:
		for i := 1; i <= 100; i++ {
			select {
			case <-t.C:
				break OuterLoop
			case item = <-logbuf:
				itemToInfluxPoint(item, &pointsbuf[i])
				l = i
			}
		}
		pts := pointsbuf[0:l]
		glog.V(1).Infof("LogWriter write %d log items", l)
		bps := influx.BatchPoints{
			Points:          pts,
			Database:        *influxdb_database,
			RetentionPolicy: "default",
			Precision:       "n",
		}
		for i := 0; ; i++ {
			resp, err := cli.Write(bps)
			if err == nil {
				break
			}
			// error, log it and retry
			glog.Errorf("Error writing to influxdb: %s %s", err, resp)
			time.Sleep(backoff.Default.Duration(i))
		}
	}
}
