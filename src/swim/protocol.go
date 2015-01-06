package swim

import (
	"errors"
	"fmt"
	"math/rand"
	"net"
	"time"

	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
)

// this object hold all belongings of a single protocol executor.
// We will have at least two (local and wan)
type swimmer struct {
	s          *Swim
	clientConn *net.UDPConn
	seq        int64
	buf        [1500]byte
	pb         SwimMessage
}

func newSwimmer(s *Swim) (ret *swimmer, err error) {
	ret = &swimmer{s: s}
	cliAddr := net.UDPAddr{IP: s.Addr.IP, Port: 0}
	ret.clientConn, err = net.ListenUDP(cliAddr.Network(), &cliAddr)
	if err != nil {
		glog.Errorf("Swim: Unable to listen on client udp %s: %s", &cliAddr, err)
		return nil, err
	}
	glog.V(2).Infof("Client on %s", ret.clientConn.LocalAddr())
	return
}

func (s *swimmer) close() {
	if s.clientConn != nil {
		s.clientConn.Close()
	}
}

//shuffle nodes list using Satollo's Fisher-Yates
//http://en.wikipedia.org/wiki/Fisher%E2%80%93Yates_shuffle#Sattolo.27s_algorithm
func shuffle(a []*node) {
	for i := range a {
		j := rand.Intn(i + 1)
		a[i], a[j] = a[j], a[i]
	}
}

var noShuffleForTest = false

//select random peer and perform communication with it
func (s *swimmer) protoCycle() {
	//select all nodes and randomize the list
	var nodes []*node
	s.s.mu.Lock()
	copy(nodes, s.s.nodes)
	s.s.mu.Unlock()
	if !noShuffleForTest {
		shuffle(nodes)
	}
	for _, n := range nodes {
		s.protoOnce(n)
	}
}

//run protocol with specified node once.
func (s *swimmer) protoOnce(target *node) {
	ack, err := s.pingack(target, &Ping{SourceNode: s.s.name})
	if err != nil {
		glog.Errorf("Ping to %s error: %s", target, err)
		//select nodes in up state
		s.s.mu.Lock()
		upnodes := make([]*node, 0, len(s.s.nodes))
		for _, n := range s.s.nodes {
			if n != target && n.isUp {
				upnodes = append(upnodes, n)
			}
		}
		s.s.mu.Unlock()
		if len(upnodes) == 0 {
			glog.Error("ping failed with '%s' and no nodes to proxy ping request", err)
			return
		}
		//select 2 nodes
		proxy1 := upnodes[rand.Intn(len(upnodes))]
		proxy2 := upnodes[rand.Intn(len(upnodes))]
		ack, err = s.pingreqack(proxy1, proxy2, &PingReq{SourceNode: s.s.name, DestNode: target.name})
		if err != nil {
			glog.Errorf("Unable to proxy ping to %s: %s", target, err)
			return
		}
	}
	target.isUp = ack.Alive
	//process ack
}

//set sequence in req packet, marshal and send it to specified node

func (s *swimmer) sendRequest(addr *net.UDPAddr, req *SwimMessage) error {
	bv, err := proto.Marshal(req)
	if err != nil {
		return err
	}
	if len(bv) > len(s.buf) {
		return errors.New("packet too big to send over UDP")
	}
	glog.V(2).Infof("sending %d bytes packet to node %s: %v", len(bv), addr, req)
	sent, err := s.clientConn.WriteToUDP(bv, addr)
	if err != nil {
		return err
	}
	if sent != len(bv) {
		return fmt.Errorf("Unexpected number of bytes sent. expected %d sent %d", len(bv), sent)
	}
	return nil
}

// receive response with certain sequence id. drop any responses with wring seq id
//caller must set a deadline on the socket
// TODO: consider processing of pkts with out of order seq id
func (s *swimmer) receiveResponse(seq int64, to *SwimMessage) (resp *SwimMessage, err error) {
	for {
		glog.V(2).Infof("waiting for pkt")
		n, _, err := s.clientConn.ReadFromUDP(s.buf[0:])
		if err != nil {
			return nil, err
		}
		glog.V(2).Infof("received pkt len %d", n)

		err = proto.Unmarshal(s.buf[0:n], to)
		if err != nil {
			return nil, err
		}
		glog.V(2).Info("received pkt ", to)

		if to.Seq == seq {
			return to, nil
		}
		glog.V(1).Infof("expected seq %d receiver %d", seq, to.Seq)
	}
}

//send ping and await ack
func (s *swimmer) pingack(n *node, pkt *Ping) (resp *Ack, err error) {
	s.clientConn.SetDeadline(time.Now().Add(rtt))
	s.seq = s.seq + 1
	s.pb = SwimMessage{Seq: s.seq, Ping: pkt}
	err = s.sendRequest(n.addr, &s.pb)
	if err != nil {
		return
	}
	r, err := s.receiveResponse(s.seq, &s.pb)
	if err != nil {
		return
	}
	return r.Ack, nil
}

//send pingreq to 2 nodes and await first ack. second packet would be skipped
//by receiver as oos on the next read
func (s *swimmer) pingreqack(n1, n2 *node, pkt *PingReq) (resp *Ack, err error) {
	s.clientConn.SetDeadline(time.Now().Add(rtt * 2))
	s.seq = s.seq + 1
	s.pb = SwimMessage{Seq: s.seq, PingReq: pkt}
	var err1, err2 error
	err1 = s.sendRequest(n1.addr, &s.pb)
	if err1 != nil {
		glog.Error("Unable to send packet to n1: ", err1)
	}
	if n2 != nil && n2 != n1 {
		err2 = s.sendRequest(n2.addr, &s.pb)
		if err2 != nil {
			glog.Error("Unable to send packet to n2: ", err2)
		}
	}
	r, err := s.receiveResponse(s.seq, &s.pb)
	if err != nil {
		return
	}
	return r.Ack, nil
}
