// Code generated by protoc-gen-go.
// source: config.proto
// DO NOT EDIT!

/*
Package config is a generated protocol buffer package.

It is generated from these files:
	config.proto

It has these top-level messages:
	HttpHandler
	HttpFrontend
	Server
	HttpBackend
	Config
*/
package config

import proto "github.com/golang/protobuf/proto"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal

type HttpHandler struct {
	// path matching rules are explained here http://golang.org/pkg/net/http/#ServeMux
	Path        string `protobuf:"bytes,1,opt,name=path" json:"path,omitempty"`
	BackendName string `protobuf:"bytes,2,opt,name=backend_name" json:"backend_name,omitempty"`
}

func (m *HttpHandler) Reset()         { *m = HttpHandler{} }
func (m *HttpHandler) String() string { return proto.CompactTextString(m) }
func (*HttpHandler) ProtoMessage()    {}

type HttpFrontend struct {
	Name        string               `protobuf:"bytes,1,opt,name=name" json:"name,omitempty"`
	BindAddress string               `protobuf:"bytes,2,opt,name=bind_address" json:"bind_address,omitempty"`
	Host        []*HttpFrontendVhost `protobuf:"bytes,3,rep,name=host" json:"host,omitempty"`
}

func (m *HttpFrontend) Reset()         { *m = HttpFrontend{} }
func (m *HttpFrontend) String() string { return proto.CompactTextString(m) }
func (*HttpFrontend) ProtoMessage()    {}

func (m *HttpFrontend) GetHost() []*HttpFrontendVhost {
	if m != nil {
		return m.Host
	}
	return nil
}

type HttpFrontendVhost struct {
	Default bool           `protobuf:"varint,1,opt,name=default" json:"default,omitempty"`
	Domain  []string       `protobuf:"bytes,2,rep,name=domain" json:"domain,omitempty"`
	Handler []*HttpHandler `protobuf:"bytes,3,rep,name=handler" json:"handler,omitempty"`
}

func (m *HttpFrontendVhost) Reset()         { *m = HttpFrontendVhost{} }
func (m *HttpFrontendVhost) String() string { return proto.CompactTextString(m) }
func (*HttpFrontendVhost) ProtoMessage()    {}

func (m *HttpFrontendVhost) GetHandler() []*HttpHandler {
	if m != nil {
		return m.Handler
	}
	return nil
}

type Server struct {
	Address string `protobuf:"bytes,1,opt,name=address" json:"address,omitempty"`
	Weight  int64  `protobuf:"varint,2,opt,name=weight" json:"weight,omitempty"`
	Maxconn int64  `protobuf:"varint,3,opt,name=maxconn" json:"maxconn,omitempty"`
}

func (m *Server) Reset()         { *m = Server{} }
func (m *Server) String() string { return proto.CompactTextString(m) }
func (*Server) ProtoMessage()    {}

type HttpBackend struct {
	Name   string    `protobuf:"bytes,1,opt,name=name" json:"name,omitempty"`
	Server []*Server `protobuf:"bytes,2,rep,name=server" json:"server,omitempty"`
}

func (m *HttpBackend) Reset()         { *m = HttpBackend{} }
func (m *HttpBackend) String() string { return proto.CompactTextString(m) }
func (*HttpBackend) ProtoMessage()    {}

func (m *HttpBackend) GetServer() []*Server {
	if m != nil {
		return m.Server
	}
	return nil
}

type Config struct {
	HttpFrontend []*HttpFrontend `protobuf:"bytes,1,rep,name=http_frontend" json:"http_frontend,omitempty"`
	HttpBackend  []*HttpBackend  `protobuf:"bytes,2,rep,name=http_backend" json:"http_backend,omitempty"`
}

func (m *Config) Reset()         { *m = Config{} }
func (m *Config) String() string { return proto.CompactTextString(m) }
func (*Config) ProtoMessage()    {}

func (m *Config) GetHttpFrontend() []*HttpFrontend {
	if m != nil {
		return m.HttpFrontend
	}
	return nil
}

func (m *Config) GetHttpBackend() []*HttpBackend {
	if m != nil {
		return m.HttpBackend
	}
	return nil
}

func init() {
}
