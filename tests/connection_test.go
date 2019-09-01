package tests

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"log"
	"testing"

	"github.com/streadway/amqp"
)

func TestBasicConnection(t *testing.T) {

}

func TestReconnection(t *testing.T) {

}

func TestPoolConnection(t *testing.T) {

}

func TestConnectionPool(t *testing.T) {

}

func TestChannelPool(t *testing.T) {

}

func TestTLSConnection(t *testing.T) {
	// https://github.com/streadway/amqp/blob/master/examples_test.go
	// This example assume you have a RabbitMQ node running on localhost
	// with TLS enabled.
	//
	// The easiest way to create the CA, certificates and keys required for these
	// examples is by using tls-gen: https://github.com/michaelklishin/tls-gen
	//
	// A comprehensive RabbitMQ TLS guide can be found at
	// http://www.rabbitmq.com/ssl.html
	//
	// Once you have the required TLS files in place, use the following
	// rabbitmq.config example for the RabbitMQ node that you will run on
	// localhost:
	//
	//   [
	//   {rabbit, [
	//     {tcp_listeners, []},     % listens on 127.0.0.1:5672
	//     {ssl_listeners, [5671]}, % listens on 0.0.0.0:5671
	//     {ssl_options, [{cacertfile,"/path/to/your/testca/cacert.pem"},
	//                    {certfile,"/path/to/your/server/cert.pem"},
	//                    {keyfile,"/path/to/your/server/key.pem"},
	//                    {verify,verify_peer},
	//                    {fail_if_no_peer_cert,true}]}
	//     ]}
	//   ].
	//
	//
	// In the above rabbitmq.config example, we are disabling the plain AMQP port
	// and verifying that clients and fail if no certificate is presented.
	//
	// The self-signing certificate authority's certificate (cacert.pem) must be
	// included in the RootCAs to be trusted, otherwise the server certificate
	// will fail certificate verification.
	//
	// Alternatively to adding it to the tls.Config. you can add the CA's cert to
	// your system's root CAs.  The tls package will use the system roots
	// specific to each support OS.  Under OS X, add (drag/drop) cacert.pem
	// file to the 'Certificates' section of KeyChain.app to add and always
	// trust.  You can also add it via the command line:
	//
	//   security add-certificate testca/cacert.pem
	//   security add-trusted-cert testca/cacert.pem
	//
	// If you depend on the system root CAs, then use nil for the RootCAs field
	// so the system roots will be loaded instead.
	//
	// Server names are validated by the crypto/tls package, so the server
	// certificate must be made for the hostname in the URL.  Find the commonName
	// (CN) and make sure the hostname in the URL matches this common name.  Per
	// the RabbitMQ instructions (or tls-gen) for a self-signed cert, this defaults to the
	// current hostname.
	//
	//   openssl x509 -noout -in /path/to/certificate.pem -subject
	//
	// If your server name in your certificate is different than the host you are
	// connecting to, set the hostname used for verification in
	// ServerName field of the tls.Config struct.
	cfg := new(tls.Config)

	// see at the top
	cfg.RootCAs = x509.NewCertPool()

	if ca, err := ioutil.ReadFile("testca/cacert.pem"); err == nil {
		cfg.RootCAs.AppendCertsFromPEM(ca)
	}

	// Move the client cert and key to a location specific to your application
	// and load them here.

	if cert, err := tls.LoadX509KeyPair("client/cert.pem", "client/key.pem"); err == nil {
		cfg.Certificates = append(cfg.Certificates, cert)
	}

	// see a note about Common Name (CN) at the top
	conn, err := amqp.DialTLS("amqps://server-name-from-certificate/", cfg)

	log.Printf("conn: %v, err: %v", conn, err)
}
