package config

import (
	"fmt"
	"testing"

	"github.com/golang/protobuf/proto"
)

func TestValidate(t *testing.T) {
	invalid := []string{
		`asdf`,
		`http_frontend<>,http_backend<>,http_frontend<>`,
		`http_backend<>`,                                                                //backend with empty name
		`http_frontend: <bind_address: ":80" handler: <path: "/">>`,                     //handler without backend
		`http_frontend: <bind_address: ":80" handler: <path: "/" backend_name: "be1">>`, //no such backend
	}

	valid := []string{
		``,
		`http_frontend<>,http_frontend<>`,
		`http_frontend<>,http_backend<name:"be1">,http_frontend<>,http_backend<name:"be2">`,
		`http_frontend<bind_address: ":80" host: < handler: <path: "/" backend_name: "be1"> > >,http_backend<name:"be1">,http_frontend<>,http_backend<name:"be2">`,
	}
	for i, cs := range invalid {
		_, err := FromText(cs)
		if err == nil {
			t.Errorf("config %d expected to be invalid: '%s'", i+1, cs)
		}
	}

	for i, cs := range valid {
		_, err := FromText(cs)
		if err != nil {
			t.Errorf("config %d expected to be valid, got error '%s': '%s'", i+1, err, cs)
		}
	}
}

func ExampleFormat() {
	cs := &Config{
		HttpFrontend: []*HttpFrontend{
			&HttpFrontend{
				BindAddress: ":80",
				Host: []*HttpFrontendVhost{&HttpFrontendVhost{
					Domain: []string{"host.com", "www.host.com"},
					Handler: []*HttpHandler{
						&HttpHandler{Path: "/", BackendName: "be"},
						&HttpHandler{Path: "/api", BackendName: "api"},
					}},
				},
			},
			&HttpFrontend{
				BindAddress: "127.0.0.1:8080",
			},
		},
		HttpBackend: []*HttpBackend{
			&HttpBackend{
				Name: "be",
				Server: []*Server{
					&Server{
						Address: "10.0.0.1:1234",
						Weight:  10,
						Maxconn: 100,
					},
					&Server{
						Address: "10.0.0.2:1234",
						Weight:  1,
						Maxconn: 10,
					},
				},
			},
		},
	}
	fmt.Println(proto.MarshalTextString(cs))
	// Output:
	// http_frontend: <
	//   bind_address: ":80"
	//   host: <
	//     domain: "host.com"
	//     domain: "www.host.com"
	//     handler: <
	//       path: "/"
	//       backend_name: "be"
	//     >
	//     handler: <
	//       path: "/api"
	//       backend_name: "api"
	//     >
	//   >
	// >
	// http_frontend: <
	//   bind_address: "127.0.0.1:8080"
	// >
	// http_backend: <
	//   name: "be"
	//   server: <
	//     address: "10.0.0.1:1234"
	//     weight: 10
	//     maxconn: 100
	//   >
	//   server: <
	//     address: "10.0.0.2:1234"
	//     weight: 1
	//     maxconn: 10
	//   >
	// >
}
