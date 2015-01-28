package config

import (
	"flag"
	"fmt"
	"log"
	"net"
	"strings"
)

var (
	Port      = flag.Int("port", 17888, "Port for node")
	PrivateIp = flag.String("node_ip", autoconfPrivateIp(), "Private IP if autoconf override is required")
	Seed      = flag.String("seed", "", "ip:port of a seed node")

	//filled out by Configure()
	NodeId string
)

func Configure() {
	NodeId = fmt.Sprintf("%s:%d", *PrivateIp, *Port)
}

// Naive implementation of private network determination
//TODO: replace with something more sane
func isPrivateIp(ip string) bool {
	return strings.HasPrefix(ip, "192.168.") || strings.HasPrefix(ip, "10.")
}

//configure node id from
func autoconfPrivateIp() string {
	ifv, err := net.InterfaceAddrs()
	if err != nil {
		log.Printf("Unable to obtain interface addrs: %s", err)
		return "unknown"
	}
	for _, iface := range ifv {
		if !isPrivateIp(iface.String()) {
			continue
		}
		ip, _, err := net.ParseCIDR(iface.String())
		if err != nil {
			log.Printf("Unable to parse network interface address '%s'", iface.String())
			continue
		}
		return ip.String()
	}
	return "unknown"
}
