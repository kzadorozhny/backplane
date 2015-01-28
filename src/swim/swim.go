// Copyright 2014 The Backplane Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This package contains implementation of discovery and dissemination protocol described in
// http://www.cs.cornell.edu/~asdas/research/dsn02-swim.pdf
// with several differences:
// - down nodes are not removed from the list but marked as down to handle netsplits
//
// Dissemination:
// All incoming updates are applied to local in-memory state.
// If the incoming update is new to the system it is updated to the outbound log.
// Each item in the outbound log has local sequence number. Each node stores last seen sequence numbers from all remote nodes
// as well as last local seq confirmed by remote node.
// Each ping request contains remote log fetch request after particluar (last seen) seq

package swim

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/apesternikov/backplane/src/gen"
	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
)

var protoPreiod = 1000 * time.Millisecond
var rtt = 200 * time.Millisecond

var now func() int64 = func() int64 {
	return time.Now().UnixNano()
}

type node struct {
	addr        *net.UDPAddr
	name        string
	lastChanged time.Time
	update      *gen.DisseminationUpdateMsg
}

func NewNode(hostport string) (n *node, err error) {
	a, err := parseIpPort(hostport)
	if err != nil {
		return nil, err
	}
	return &node{addr: a, name: hostport}, nil
}

func (n *node) setFromUpdate(update *gen.DisseminationUpdateMsg) (updated bool) {
	if n.update == nil || update.Alive != n.update.Alive {
		n.update = update
		n.lastChanged = time.Now()
		glog.V(1).Info("node remote state ", n)
		return true
	}
	return false
}

//mark node as up and (re)generate update message if needed
func (n *node) setUp(isUp bool) (updated bool) {
	if n.update == nil || isUp != n.update.Alive {
		update := &gen.DisseminationUpdateMsg{
			Timestamp: now(),
			NodeName:  n.name,
			Alive:     isUp,
		}
		n.update = update
		n.lastChanged = time.Now()
		glog.V(1).Info("node local state ", n)
		return true
	}
	return false
}

func (n *node) Up() bool {
	if n.update != nil {
		return n.update.Alive
	}
	return false
}

func (n *node) String() string {
	var state string
	if n.Up() {
		state = "UP"
	} else {
		state = "DOWN"
	}
	return fmt.Sprintf("Node %s %s changed %s %s", n.addr, state, n.lastChanged.Format(time.RFC1123), n.update)
}

type Swim struct {
	name       string       //swim node id
	Addr       *net.UDPAddr //local udp address
	serverConn *net.UDPConn
	client     *swimmer //this client is used by server side to execute ping_req

	mu       sync.Mutex
	nodes    []*node //all nodes in the network excluding itself. TODO: split into dclocal and dcremote
	nodesmap map[string]*node
	updates  []*gen.DisseminationUpdateMsg //current implementation send all updates.
}

func (s *Swim) HandleStatus(rw http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(rw, "name: %s\n", s.name)
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, n := range s.nodes {
		fmt.Fprintf(rw, "%s\n", n)
	}
}

var badip = errors.New("Unable to parse IP")

func parseIpPort(hostport string) (addr *net.UDPAddr, err error) {
	host, port, err := net.SplitHostPort(hostport)
	if err != nil {
		return
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return nil, badip
	}
	p, err := strconv.Atoi(port)
	if err != nil {
		return
	}
	return &net.UDPAddr{IP: ip, Port: p}, nil
}

func NewSwim(localhostport string) (s *Swim, err error) {
	s = new(Swim)
	s.Addr, err = parseIpPort(localhostport)
	if err != nil {
		return
	}
	s.nodesmap = make(map[string]*node)
	s.serverConn, err = net.ListenUDP(s.Addr.Network(), s.Addr)
	if err != nil {
		glog.Errorf("Swim: Unable to listen on local udp %s: %s", s.Addr, err)
		return nil, err
	}
	s.name = s.serverConn.LocalAddr().String()
	glog.Info("Swim: serving ", s.name)
	s.client, err = newSwimmer(s)
	return
}

//add nodes in down state
func (s *Swim) AddHosts(hosts ...string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, host := range hosts {
		if _, ok := s.nodesmap[host]; !ok {
			n, err := NewNode(host)
			if err != nil {
				glog.Errorf("Unable to create node %s: %s", host, err)
				continue
			}
			s.nodes = append(s.nodes, n)
			s.nodesmap[host] = n
		}
	}
}

//find the node or add node in down state and return it
func (s *Swim) getHost(host string) (n *node, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n, ok := s.nodesmap[host]; !ok {
		n, err := NewNode(host)
		if err != nil {
			return nil, err
		}
		s.nodes = append(s.nodes, n)
		s.nodesmap[host] = n
		return n, nil
	} else {
		return n, nil
	}
}

func (s *Swim) Serve() (err error) {
	go s.client.protoLoop()
	for {
		if err = s.serveOnce(); err != nil {
			return
		}
	}
}

func (s *Swim) serveOnce() error {
	var in, out gen.SwimMessage
	// out.Reset()
	n, addr, err := s.serverConn.ReadFromUDP(s.client.buf[0:])
	if err != nil {
		glog.Error("error serving swim: ", err)
		return err
	}
	glog.V(2).Infof("received pkt len %d form %s", n, addr)

	err = proto.Unmarshal(s.client.buf[0:n], &in)
	if err != nil {
		glog.Error("error unmarshalling swim: ", err)
		return nil //ignore error, continue loop
	}
	glog.V(2).Info("swim serving inpkt ", &in)
	//first, apply all db updates
	s.onUpdatePkts(in.DisseminationUpdates)
	out.Seq = in.Seq
	switch {
	case in.Ping != nil:
		out.Ack = s.servePing(in.Ping)
	case in.PingReq != nil:
		out.Ack = s.servePingReq(in.PingReq)
	}
	//add db
	out.DisseminationUpdates = s.updates
	bv, err := proto.Marshal(&out)
	if err != nil {
		glog.Error("error marshalling swim response: ", err)
		return nil //ignore error, continue loop
	}
	if len(bv) > len(s.client.buf) {
		glog.Error("packet is too big to send over UDP")
		return nil //ignore error, continue loop
	}
	glog.V(2).Infof("sending %d bytes packet to node %s: %v", len(bv), addr, &out)
	sent, err := s.serverConn.WriteToUDP(bv, addr) //responding to the original peer address
	if err != nil {
		glog.Error("error sending swim response: ", err)
		return nil //ignore error, continue loop
	}
	if sent != len(bv) {
		return fmt.Errorf("Unexpected number of bytes sent. expected %d sent %d", len(bv), sent)
	}
	return nil
}

func (s *Swim) servePing(pkt *gen.Ping) *gen.Ack {
	// first, ping means the source node exists and alive
	n, err := s.getHost(pkt.SourceNode)
	if err != nil {
		glog.Error("getHost: ", err)
		return nil
	}
	if n.setUp(true) {
		s.genUpdates()
	}
	return &gen.Ack{Alive: true}
}

func (s *Swim) servePingReq(pkt *gen.PingReq) *gen.Ack {
	// first, ping means the source node exists and alive
	src, err := s.getHost(pkt.SourceNode)
	if err != nil {
		glog.Error("getHost: ", err)
		return nil
	}
	if src.setUp(true) {
		s.genUpdates()
	}
	//get the dest node
	dst, err := s.getHost(pkt.DestNode)
	if err != nil {
		glog.Error("getHost: ", err)
		return nil
	}
	ack, err := s.client.pingack(dst, &gen.Ping{s.name})
	if err != nil {
		glog.Error("pingack: ", err)
		return &gen.Ack{Alive: false}
	}
	return ack
}

//close connections
func (s *Swim) Close() {
	if s.serverConn != nil {
		s.serverConn.Close()
	}
	if s.client != nil {
		s.client.close()
	}
}

//db functions

//update single node. assumes s is locked
//returns true if data is updated
func (s *Swim) onUpdatePkt(pkt *gen.DisseminationUpdateMsg) bool {
	var err error
	n, ok := s.nodesmap[pkt.NodeName]
	if !ok {
		n, err = NewNode(pkt.NodeName)
		if err != nil {
			glog.Error("Unable to update node with '%s': %s", pkt, err)
			return false
		}
		s.nodes = append(s.nodes, n)
		s.nodesmap[pkt.NodeName] = n
	}
	if n.setFromUpdate(pkt) {
		// the info in update is new, update the record
		s.genUpdates()
		return true
	}
	return false
}

//regenerate outbound updates
//assumes mutex is locked
func (s *Swim) genUpdates() {
	s.updates = s.updates[0:0]
	for _, n := range s.nodes {
		if n.update != nil {
			s.updates = append(s.updates, n.update)
		}
	}

}

func (s *Swim) onUpdatePkts(pkts []*gen.DisseminationUpdateMsg) (updated bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, pkt := range pkts {
		updated = updated || s.onUpdatePkt(pkt)
	}
	if updated {
		s.genUpdates()
	}
	return
}
