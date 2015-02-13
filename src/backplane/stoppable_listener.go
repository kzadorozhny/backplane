package backplane

import (
	"errors"
	"net"
	"sync/atomic"
	"time"
)

// tcpKeepAliveListener sets TCP keep-alive timeouts on accepted
// connections. It's used by ListenAndServe and ListenAndServeTLS so
// dead TCP connections (e.g. closing laptop mid-download) eventually
// go away.

type StoppableListener struct {
	*net.TCPListener
	stop        chan int //Channel used only to indicate listener should shutdown. listener will close it after the shutdown
	AcceptedCnt int64
}

func NewStoppableListener(l *net.TCPListener) *StoppableListener {
	return &StoppableListener{l, make(chan int), 0}
}

var StoppedError = errors.New("Stopped")

func (sl *StoppableListener) Accept() (net.Conn, error) {
	for {
		//Wait up to one second for a new connection
		sl.SetDeadline(time.Now().Add(time.Second))

		tc, err := sl.TCPListener.AcceptTCP()

		//Check for the channel being closed
		select {
		case <-sl.stop:
			close(sl.stop)
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
		atomic.AddInt64(&sl.AcceptedCnt, 1)
		tc.SetKeepAlive(true)
		tc.SetKeepAlivePeriod(3 * time.Minute)

		return tc, err
	}
}

func (sl *StoppableListener) Stop(wait bool) {
	sl.stop <- 1
	if wait {
		<-sl.stop
	}
}
