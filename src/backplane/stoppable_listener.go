package backplane

import (
	"errors"
	"net"
	"sync/atomic"
	"time"

	"github.com/golang/glog"

	"github.com/apesternikov/backplane/src/backplane/stats"
)

// tcpKeepAliveListener sets TCP keep-alive timeouts on accepted
// connections. It's used by ListenAndServe and ListenAndServeTLS so
// dead TCP connections (e.g. closing laptop mid-download) eventually
// go away.

type StoppableListener struct {
	*net.TCPListener
	stop        chan int //Channel used only to indicate listener should shutdown. listener will close it after the shutdown
	AcceptedCnt int64
	ActiveCnt   int64
	RateLimiter stats.RateLimiter
	Limiter     stats.Limiter
}

func NewStoppableListener(l *net.TCPListener, maxrate float64, maxallowed int64) *StoppableListener {
	if maxrate == 0 {
		maxrate = 999999
	}
	return &StoppableListener{
		l,
		make(chan int),
		0,
		0,
		stats.NewRateLimiter(maxrate),
		stats.NewLimiter(int(maxallowed)),
	}
}

var StoppedError = errors.New("Stopped")

type conn struct {
	net.Conn
	closed bool
	parent *StoppableListener
}

func (c *conn) Close() error {
	if !c.closed {
		c.closed = true
		c.parent.Limiter.Release(nil)
		atomic.AddInt64(&c.parent.ActiveCnt, -1)
	}
	return c.Conn.Close()
}

func (sl *StoppableListener) Accept() (net.Conn, error) {
	sl.Limiter.Acquire(nil) //TODO: acquiring this will delay stop until sema is available
	for {
		//Wait up to one second for a new connection
		sl.SetDeadline(time.Now().Add(time.Second))

		tc, err := sl.TCPListener.AcceptTCP()

		//Check for the channel being closed
		select {
		case <-sl.stop:
			close(sl.stop)
			sl.Limiter.Release(nil)
			return nil, StoppedError
		default:
			//If the channel is still open, continue as normal
		}

		if err != nil {
			netErr, ok := err.(net.Error)

			//If this is a timeout, then continue to wait for
			//new connections
			if ok && netErr.Timeout() && netErr.Temporary() {
				continue
			}
		}
		if !sl.RateLimiter.Accepted() {
			glog.Error("Acceptor QPS is too high, dropping connection")
			tc.Close()
			continue
		}
		atomic.AddInt64(&sl.ActiveCnt, 1)
		atomic.AddInt64(&sl.AcceptedCnt, 1)
		tc.SetKeepAlive(true)
		tc.SetKeepAlivePeriod(3 * time.Minute)

		return &conn{Conn: tc, parent: sl}, err
	}
}

func (sl *StoppableListener) Stop(wait bool) {
	sl.stop <- 1
	if wait {
		<-sl.stop
	}
}
