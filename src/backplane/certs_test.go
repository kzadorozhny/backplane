package backplane

import "testing"

func TestNoSuchDir(t *testing.T) {
	certs, err := LoadCertsByMask("testdata/nosuchdir/*.pem")
	if err != nil {
		t.Errorf("Expected no error, returned %v", err)
	}
	if len(certs) != 0 {
		t.Errorf("Expected no certs, returned %v", certs)
	}
}

func TestSingleEmptyCertFile(t *testing.T) {
	certs, err := LoadCertsByMask("testdata/empty*")
	if err != nil {
		t.Errorf("Expected no error, returned %v", err)
	}
	if len(certs) != 0 {
		t.Error("Expected error, returned nil")
	}
}

func TestEmptyCertFile(t *testing.T) {
	_, err := X509KeyPairFromFile("testdata/emptycertfile.pem")
	if err != NO_PRIVATE_KEY {
		t.Errorf("Expected NO_PRIVATE_KEY, got %v", err)
	}
}

func TestCertOnlyFile(t *testing.T) {
	_, err := X509KeyPairFromFile("testdata/certonly.pem")
	if err != NO_PRIVATE_KEY {
		t.Errorf("Expected NO_PRIVATE_KEY, got %v", err)
	}
}

func TestKeyOnlyFile(t *testing.T) {
	_, err := X509KeyPairFromFile("testdata/keyonly.pem")
	if err != NO_PUBLIC_CERT {
		t.Errorf("Expected NO_PUBLIC_CERT, got %v", err)
	}
}

func TestTwoCerts(t *testing.T) {
	certs, err := LoadCertsByMask("testdata/certpem*.pem")
	if err != nil {
		t.Errorf("Expected no error, returned %v", err)
	}
	if len(certs) != 2 {
		t.Error("Expected 2 certificates, got ", certs)
	}
}
