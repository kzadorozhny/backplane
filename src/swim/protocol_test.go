package swim

import (
	"net"
	"testing"
	"time"

	"github.com/apesternikov/backplane/src/gen"
	"github.com/golang/protobuf/proto"
)

func TestSendReceive(t *testing.T) {
	s1, err := NewSwim("127.0.0.1:1245")
	if err != nil {
		t.Fatal("Unable to create swim1: ", err)
	}
	defer s1.Close()
	s2, err := NewSwim("127.0.0.1:1246")
	if err != nil {
		t.Fatal("Unable to create swim2: ", err)
	}
	defer s2.Close()
	p1, err := newSwimmer(s1)
	if err != nil {
		t.Fatal("Unable to create swimmer1: ", err)
	}
	defer p1.close()
	p2, err := newSwimmer(s2)
	if err != nil {
		t.Fatal("Unable to create swimmer2: ", err)
	}
	defer p2.close()
	deadline := time.Now().Add(rtt)
	p1.clientConn.SetDeadline(deadline)
	p2.clientConn.SetDeadline(deadline)
	//addr1, err := parseIpPort("127.0.0.1:1245")
	addr2 := p2.clientConn.LocalAddr().(*net.UDPAddr)
	pb1 := &gen.SwimMessage{Seq: 1, Ping: &gen.Ping{}}
	pb2 := &gen.SwimMessage{}
	err = p1.sendRequest(addr2, pb1)
	if err != nil {
		t.Fatal("Unable to send request: ", err)
	}
	resp, err := p2.receiveResponse(1, pb2)
	if err != nil {
		t.Fatal("Unable to receive response: ", err)
	}
	if resp != pb2 {
		t.Fatal("nexpected value of resp")
	}
	if !proto.Equal(pb1, pb2) {
		t.Fatal("Expecting equal protos, got %s %s", pb1, pb2)
	}
	if pb2.GetPing() == nil {
		t.Error("Unexpected value of ping")
	}
}
