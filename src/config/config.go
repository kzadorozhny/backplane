package config

import (
	"fmt"

	"github.com/golang/protobuf/proto"
)

var staticbackends = map[string]bool{"internalstats": true}

//go:generate protoc --go_out=. config.proto
//TODO: fail on duplicates
func Validate(cf *Config) error {
	backends := make(map[string]*HttpBackend)
	for _, b := range cf.HttpBackend {
		if b.Name == "" {
			return fmt.Errorf("backend has empty name")
		}
		backends[b.Name] = b
	}
	for _, f := range cf.HttpFrontend {
		for _, h := range f.Host {
			for _, b := range h.Handler {
				if b.Path == "" {
					return fmt.Errorf("binding with empty path")
				}
				if len(b.BackendName) == 0 {
					return fmt.Errorf("binding %s has no backends configured", b.Path)
				}
				if backends[b.BackendName] == nil && !staticbackends[b.BackendName] {
					return fmt.Errorf("binding %s: unknown backend %s", b.Path, b.BackendName)
				}
			}

		}
	}
	return nil
}

func FromText(textcf string) (*Config, error) {
	cf := new(Config)
	err := proto.UnmarshalText(textcf, cf)
	if err != nil {
		return nil, err
	}
	err = Validate(cf)
	if err != nil {
		return nil, err
	}
	return cf, nil
}
