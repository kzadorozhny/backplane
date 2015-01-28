package swim

import (
	"net"

	"github.com/apesternikov/backplane/src/gen"

	"testing"
)

func TestParseIpPort(t *testing.T) {
	a, e := parseIpPort("127.0.0.1:1234")
	if e != nil {
		t.Error("expected no error, got ", e)
	}
	if !a.IP.Equal(net.ParseIP("127.0.0.1")) {
		t.Error("unexpected ip ", a.IP)
	}
	if a.Port != 1234 {
		t.Error("unexpected port ", a.Port)
	}
}

func TestParseBadIpPort(t *testing.T) {
	//test bad ip
	_, e := parseIpPort("bs:1234")
	if e == nil {
		t.Error("expected error, got nil")
	}
	//make sure we are not resolving
	_, e = parseIpPort("localhost:1234")
	if e == nil {
		t.Error("expected error, got nil")
	}
}

func TestAddHosts(t *testing.T) {
	s, err := NewSwim("127.0.0.1:1234")
	if err != nil {
		t.Error("Unable to initialize Swim ", err)
	}
	defer s.Close()
	if len(s.nodes) != 0 || len(s.nodesmap) != 0 {
		t.Error("Unexpected nodes lengths", len(s.nodes), len(s.nodesmap))
	}
	s.AddHosts("127.0.0.1:1235", "127.0.0.1:1236")
	if len(s.nodes) != 2 || len(s.nodesmap) != 2 {
		t.Error("Unexpected nodes lengths", len(s.nodes), len(s.nodesmap))
	}
	// no new nodes here
	s.AddHosts("127.0.0.1:1235", "127.0.0.1:1236")
	if len(s.nodes) != 2 || len(s.nodesmap) != 2 {
		t.Error("Unexpected nodes lengths", len(s.nodes), len(s.nodesmap))
	}
	//add one that is not correct
	s.AddHosts("bs:1235")
	if len(s.nodes) != 2 || len(s.nodesmap) != 2 {
		t.Error("Unexpected nodes lengths", len(s.nodes), len(s.nodesmap))
	}
	//one more
	s.AddHosts("127.0.0.1:1237")
	if len(s.nodes) != 3 || len(s.nodesmap) != 3 {
		t.Error("Unexpected nodes lengths", len(s.nodes), len(s.nodesmap))
	}
}

func TestPing(t *testing.T) {
	var err error
	s1, err := NewSwim("127.0.0.1:1234")
	if err != nil {
		t.Error("Unexpected error ", err)
	}
	defer s1.Close()

	s2, err := NewSwim("127.0.0.1:1235")
	if err != nil {
		t.Error("Unexpected error ", err)
	}
	defer s2.Close()

	s2.AddHosts(s1.name)
	if len(s2.nodes) != 1 || len(s2.nodesmap) != 1 {
		t.Fatal("Unexpected sizes: ", len(s2.nodes), len(s2.nodesmap))
	}

	err = s2.client.sendRequest(s2.nodes[0].addr, &gen.SwimMessage{Seq: 1, Ping: &gen.Ping{SourceNode: s2.name}})
	if err != nil {
		t.Error("Unexpected error ", err)
	}
	s1.serveOnce()

	msg, err := s2.client.receiveResponse(1, &gen.SwimMessage{})
	if msg == nil || err != nil {
		t.Error("Unexpected error ", err)
	}
	if msg.Ack == nil || !msg.Ack.Alive {
		t.Error("Unexpected response ", msg)
	}
}

func TestPingReqToUpNode(t *testing.T) {
	var err error
	//proxy
	s1, err := NewSwim("127.0.0.1:1234")
	if err != nil {
		t.Error("Unexpected error ", err)
	}
	defer s1.Close()

	//source
	s2, err := NewSwim("127.0.0.1:1235")
	if err != nil {
		t.Error("Unexpected error ", err)
	}
	defer s2.Close()

	//target
	s3, err := NewSwim("127.0.0.1:1236")
	if err != nil {
		t.Error("Unexpected error ", err)
	}
	defer s3.Close()

	s2.AddHosts(s1.name)
	if len(s2.nodes) != 1 || len(s2.nodesmap) != 1 {
		t.Fatal("Unexpected sizes: ", len(s2.nodes), len(s2.nodesmap))
	}

	err = s2.client.sendRequest(s2.nodes[0].addr, &gen.SwimMessage{Seq: 2, PingReq: &gen.PingReq{SourceNode: s2.name, DestNode: s3.name}})
	if err != nil {
		t.Error("Unexpected error ", err)
	}
	go s1.serveOnce()
	go s3.serveOnce()

	msg, err := s2.client.receiveResponse(2, &gen.SwimMessage{})
	if msg == nil || err != nil {
		t.Error("Unexpected error ", err)
	}

	if msg.Ack == nil || !msg.Ack.Alive {
		t.Error("Unexpected response ", msg)
	}
}

func TestPingReqToDownNode(t *testing.T) {
	var err error
	//proxy
	s1, err := NewSwim("127.0.0.1:1234")
	if err != nil {
		t.Error("Unexpected error ", err)
	}
	defer s1.Close()

	//source
	s2, err := NewSwim("127.0.0.1:1235")
	if err != nil {
		t.Error("Unexpected error ", err)
	}
	defer s2.Close()

	//target
	// s3, err := NewSwim("127.0.0.1:1236")
	// if err != nil {
	// 	t.Error("Unexpected error ", err)
	// }
	// defer s3.Close()

	s2.AddHosts(s1.name)
	if len(s2.nodes) != 1 || len(s2.nodesmap) != 1 {
		t.Fatal("Unexpected sizes: ", len(s2.nodes), len(s2.nodesmap))
	}

	err = s2.client.sendRequest(s2.nodes[0].addr, &gen.SwimMessage{Seq: 2, PingReq: &gen.PingReq{SourceNode: s2.name, DestNode: "127.0.0.1:1236"}})
	if err != nil {
		t.Error("Unexpected error ", err)
	}
	go s1.serveOnce()
	// go s3.serveOnce()

	msg, err := s2.client.receiveResponse(2, &gen.SwimMessage{})
	if msg == nil || err != nil {
		t.Error("Unexpected error ", err)
	}

	if msg.Ack == nil || msg.Ack.Alive {
		t.Error("Unexpected response ", msg)
	}
}
