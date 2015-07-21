package requestlog

import (
	"bytes"
	"errors"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/apesternikov/backplane/src/backoff"

	"github.com/golang/glog"
)

//go:generate protoc --go_out=. requestlog.proto

var (
	influxdb_url      = flag.String("influxdb_url", "http://localhost:8086", "URL of influxdb")
	influxdb_database = flag.String("influxdb_database", "backplane", "influxdb database for access log")
	influxdb_user     = flag.String("influxdb_user", "", "influxdb user")
	influxdb_pass     = flag.String("influxdb_pass", "", "influxdb password")

	cli *http.Client

	logbuf   chan *Item = make(chan *Item, 10000)
	hostname string     = "unknown"
)

// We can not run this during init since we need flags to be initialized
func AfterInit() {
	_, err := url.Parse(*influxdb_url)
	if err != nil {
		log.Fatal("Unable to parse influxdb address: ", err)
	}
	cli = &http.Client{Timeout: time.Second * 10}
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

type line struct {
	bytes.Buffer
}

func (l *line) WriteEscaped(s string) {
	for _, ch := range s {
		switch ch {
		case ' ', ',', '\\':
			l.WriteByte('\\')
		}
		l.WriteRune(ch)
	}
}

func (l *line) WriteQuoted(s string) {
	l.WriteByte('"')
	for _, ch := range s {
		if ch == '"' {
			l.WriteByte('\\')
		}
		l.WriteRune(ch)
	}
	l.WriteByte('"')
}

func itemToInfluxPoint(it *Item, l *line) {
	l.WriteString("accesslog,")
	l.WriteString("hostname=")
	l.WriteEscaped(hostname)
	l.WriteString(",BackendName=")
	l.WriteEscaped(it.BackendName)
	l.WriteString(",ServerAddress=")
	l.WriteEscaped(it.ServerAddress)
	l.WriteString(",Frontend=")
	l.WriteEscaped(it.Frontend)
	l.WriteString(",HttpVersion=")
	l.WriteEscaped(it.HttpVersion)
	l.WriteString(",Method=")
	l.WriteEscaped(it.Method)
	l.WriteString(",StatusCode=")
	l.WriteString(strconv.Itoa(int(it.StatusCode)))
	l.WriteString(",IsTls=")
	l.WriteString(strconv.FormatBool(it.IsTls))

	l.WriteByte(' ')
	l.WriteString("ClientIp=")
	l.WriteQuoted(it.ClientIp)
	l.WriteString(",Referrer=")
	l.WriteQuoted(it.Referrer)
	l.WriteString(",RequestUri=")
	l.WriteQuoted(it.RequestUri)
	l.WriteString(",UserAgent=")
	l.WriteQuoted(it.UserAgent)
	l.WriteString(",ResponseSize=")
	l.WriteString(strconv.FormatInt(it.ResponseSize, 10))
	l.WriteString(",FrontendLatencyMs=")
	l.WriteString(strconv.FormatInt(it.FrontendLatencyNs/1000000, 10))
	l.WriteString(",ServerLatencyMs=")
	l.WriteString(strconv.FormatInt(it.ServerLatencyNs/1000000, 10))

	l.WriteByte(' ')

	l.WriteString(strconv.FormatInt(it.TimeTNs, 10))

	l.WriteByte('\n')
}

func LogToServer(b io.Reader) error {
	req, err := http.NewRequest("POST", *influxdb_url, b)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "")
	req.Header.Set("User-Agent", "backplane/0.1")
	if *influxdb_user != "" {
		req.SetBasicAuth(*influxdb_user, *influxdb_pass)
	}
	req.URL.Path = "/write"
	params := req.URL.Query()
	params.Add("db", *influxdb_database)
	// params.Add("rp", bp.RetentionPolicy)
	// params.Add("precision", bp.Precision)
	params.Add("consistency", "one")
	req.URL.RawQuery = params.Encode()

	resp, err := cli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil && err.Error() != "EOF" {
		return err
	}

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return errors.New(string(body))
	}
	return nil
}

func LogWriter() {
	// we are writing in batches of up to 100
	var l line
	for {
		l.Reset()
		//read 1 item
		item := <-logbuf
		itemToInfluxPoint(item, &l)
		//try to read up to 99 records in the next 100 ms
		t := time.NewTimer(time.Millisecond * 300)
	OuterLoop:
		for i := 1; i <= 100; i++ {
			select {
			case <-t.C:
				break OuterLoop
			case item = <-logbuf:
				itemToInfluxPoint(item, &l)
			}
		}
		glog.V(1).Infof("LogWriter write %d log bytes", len(l.Bytes()))
		for i := 0; ; i++ {
			err := LogToServer(&l)
			if err == nil {
				break
			}
			// error, log it and retry
			glog.Errorf("Error writing to influxdb: %s", err)
			time.Sleep(backoff.Default.Duration(i))
		}
	}
}
