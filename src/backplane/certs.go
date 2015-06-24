package backplane

import (
	"bytes"
	"crypto/tls"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/golang/glog"
)

// Load ssl key/cert pairs from mask like /etc/certs/*.pem
// File should have contactenated key/cert pairs
func LoadCertsByMask(mask string) (certs []tls.Certificate, err error) {
	files, err := filepath.Glob(mask)
	// The only possible returned error is ErrBadPattern, when pattern is malformed
	if err != nil {
		glog.Errorf("Unable to list cert files with mask '%s': %s", mask, err)
		return nil, err
	}
	for _, file := range files {
		glog.V(2).Infof("opeining cert file %s", file)
		cert, err := X509KeyPairFromFile(file)
		if err != nil {
			glog.Errorf("unable to read cert from file %s: %s", file, err)
			// TODO: we are ignoring error here. should we handle it somehow?
			continue
		}
		certs = append(certs, cert)
	}
	return certs, nil
}

var (
	NO_PRIVATE_KEY = errors.New("No private key present")
	NO_PUBLIC_CERT = errors.New("No public cert present")
)

func X509KeyPairFromMem(mem []byte) (cert tls.Certificate, err error) {
	var pub bytes.Buffer
	var priv []byte
	var derblock *pem.Block
	for {
		derblock, mem = pem.Decode(mem)
		if derblock == nil {
			break
		}
		switch {
		case derblock.Type == "CERTIFICATE":
			pem.Encode(&pub, derblock)
		case derblock.Type == "PRIVATE KEY" || strings.HasSuffix(derblock.Type, " PRIVATE KEY"):
			priv = pem.EncodeToMemory(derblock)
		default:
			err = fmt.Errorf("Unknown block type %s in cert file %s", derblock.Type)
			return
		}
	}
	if len(priv) == 0 {
		err = NO_PRIVATE_KEY
		return
	}
	if pub.Len() == 0 {
		err = NO_PUBLIC_CERT
		return
	}
	return tls.X509KeyPair(pub.Bytes(), priv)
}

// Load a pair of private key / public cert from a file
// File should have contactenated key/cert pairs
func X509KeyPairFromFile(pairfile string) (cert tls.Certificate, err error) {
	pemblock, err := ioutil.ReadFile(pairfile)
	if err != nil {
		return
	}
	return X509KeyPairFromMem(pemblock)
}
