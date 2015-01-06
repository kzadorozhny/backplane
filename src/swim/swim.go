// Copyright 2014 The Backplane Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This package contains implementation of discovery and dissemination protocol described in
// http://www.cs.cornell.edu/~asdas/research/dsn02-swim.pdf
// with several differences:
// - down nodes are not removed from the list but marked as down to handle netsplits

//go:generate protoc --go_out=. swim.proto

package swim

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
)

var protoPreiod = 200 * time.Millisecond
var rtt = 20 * time.Millisecond

type node struct {
	addr *net.UDPAddr
	name string
	isUp bool
}

func (n *node) String() string {
	var state string
	if n.isUp {
		state = "UP"
	} else {
		state = "DOWN"
	}
	return fmt.Sprintf("Node %s %s", n.addr, state)
}

type Swim struct {
	name       string       //swim node id
	Addr       *net.UDPAddr //local udp address
	serverConn *net.UDPConn
	client     *swimmer //this client is used by server side to execute ping_req

	mu       sync.Mutex
	nodes    []*node //all nodes in the network excluding itself. TODO: split into dclocal and dcremote
	nodesmap map[string]*node
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
			a, err := parseIpPort(host)
			if err != nil {
				glog.Errorf("Unable to parse host %s", host)
				continue
			}
			n := &node{addr: a, name: host}
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
		a, err := parseIpPort(host)
		if err != nil {
			return nil, err
		}
		n := &node{addr: a, name: host}
		s.nodes = append(s.nodes, n)
		s.nodesmap[host] = n
		return n, nil
	} else {
		return n, nil
	}
}

func (s *Swim) Serve() (err error) {
	for {
		if err = s.serveOnce(); err != nil {
			return
		}
	}
}

func (s *Swim) serveOnce() error {
	var in, out SwimMessage
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
	out.Seq = in.Seq
	switch {
	case in.Ping != nil:
		out.Ack = s.servePing(in.Ping)
	case in.PingReq != nil:
		out.Ack = s.servePingReq(in.PingReq)
	}
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

func (s *Swim) servePing(pkt *Ping) *Ack {
	// first, ping means the source node exists and alive
	n, err := s.getHost(pkt.SourceNode)
	if err != nil {
		glog.Error("getHost: ", err)
		return nil
	}
	n.isUp = true
	return &Ack{Alive: true}
}

func (s *Swim) servePingReq(pkt *PingReq) *Ack {
	// first, ping means the source node exists and alive
	src, err := s.getHost(pkt.SourceNode)
	if err != nil {
		glog.Error("getHost: ", err)
		return nil
	}
	src.isUp = true
	//get the dest node
	dst, err := s.getHost(pkt.DestNode)
	if err != nil {
		glog.Error("getHost: ", err)
		return nil
	}
	ack, err := s.client.pingack(dst, &Ping{s.name})
	if err != nil {
		glog.Error("pingack: ", err)
		return &Ack{Alive: false}
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
