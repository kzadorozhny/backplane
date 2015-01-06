// +build !race

package swim

import (
	"testing"
	"time"
)

//.Close() is known to cause a data race condition on the socket.
//I would prefer to aviod overhead and complexity associated with race-free implementation
func TestCloseCauseServingExit(t *testing.T) {
	var err error
	s, err := NewSwim("127.0.0.1:0")
	if err != nil {
		t.Error("Unable to initialize Swim ", err)
	}
	defer s.Close()
	exited := false
	go func() {
		err = s.Serve()
		exited = true
	}()
	time.Sleep(10 * time.Millisecond) //wait until s.Serve() starts serving
	s.Close()
	time.Sleep(10 * time.Millisecond) //wait until s.Serve() exits
	if err == nil {
		t.Error("err expected to be set")
	}
	if !exited {
		t.Error("s.Serve should exit")
	}
	t.Log("s.Serve returned err ", err)
}
