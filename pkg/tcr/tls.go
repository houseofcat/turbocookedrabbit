package tcr

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
)

// CreateTLSConfig creates a x509 TLS Config for use in TLS-based communication.
func CreateTLSConfig(pemLocation string, localLocation string) (*tls.Config, error) {
	cfg := new(tls.Config)
	cfg.RootCAs = x509.NewCertPool()

	ca, err := ioutil.ReadFile(pemLocation)
	if err != nil {
		return nil, err
	}

	cfg.RootCAs.AppendCertsFromPEM(ca)

	cert, err := tls.LoadX509KeyPair(
		localLocation,
		localLocation)
	if err != nil {
		return nil, err
	}

	cfg.Certificates = append(cfg.Certificates, cert)
	return cfg, nil
}
