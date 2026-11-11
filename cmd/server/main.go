// Command grimcraft-server: authoritative multiplayer backend for the
// grimcraft-threejs frontend.
//
// Transports (one room per lobby, many lobbies):
//   - WebSocket  /join?lobby=<id>      (local dev, bots, tests)
//   - WebTransport /grimcraft?lobby=<id> (QUIC/UDP, what the game uses)
//   - HTTP lobby browser: GET/POST /api/lobbies
//
// WebTransport needs a TLS certificate. On first start the server
// generates a self-signed cert in --cert-dir and prints its SHA-256 hash:
// paste that hash into SERVER_CERT_HASH in src/net/multiplayer.ts.
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/quic-go/quic-go/http3"
	"github.com/quic-go/webtransport-go"

	"grimcraft-go/internal/hub"
	"grimcraft-go/internal/server"
	"grimcraft-go/internal/transport"
)

func main() {
	httpAddress := flag.String("http", ":8080", "HTTP/WebSocket/lobby listen address")
	webTransportAddress := flag.String("wt", ":4433", "WebTransport (QUIC) listen address")
	certificateDirectory := flag.String("cert-dir", "certs", "directory for the self-signed WT certificate")
	skipWebTransport := flag.Bool("no-wt", false, "disable WebTransport, serve HTTP/WS only")
	flag.Parse()

	gameHub := hub.NewHub()

	router := http.NewServeMux()
	connection := &server.ConnectionInfo{}
	router.Handle("/api/", server.API(gameHub, connection))
	router.HandleFunc("/join", transport.WSHandler(gameHub))
	router.HandleFunc("/grimcraft", func(writer http.ResponseWriter, request *http.Request) {
		http.Error(writer, "use WebTransport (QUIC) for this endpoint", http.StatusBadRequest)
	})
	router.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Access-Control-Allow-Origin", "*")
		fmt.Fprintln(writer, "grimcraft-go multiplayer server. WS /join, API /api/lobbies, WT /grimcraft (QUIC).")
	})

	if *skipWebTransport {
		log.Printf("HTTP/WS+Lobbies on %s (WebTransport disabled)", *httpAddress)
		log.Fatal(http.ListenAndServe(*httpAddress, router))
	}

	certificate, certificateDER, operationError := ensureCert(*certificateDirectory)
	if operationError != nil {
		log.Fatalf("cert: %v", operationError)
	}
	fingerprint := sha256.Sum256(certificateDER)
	connection.Enabled = true
	connection.WebTransportURL = listenURL("https", *webTransportAddress) + "/grimcraft"
	connection.CertificateHash = hex.EncodeToString(fingerprint[:])
	fmt.Printf(`
══════════════════════════════════════════════════════════════
  GRIMCRAFT-GO MULTIPLAYER SERVER
  HTTP/WS+lobbies : %s
  WebTransport    : %s
  Lobbies         : %d seeded (each = one room)

  FRONTEND WIRING (src/net/multiplayer.ts):
    SERVER_URL       = '%s'
    SERVER_CERT_HASH = '%s'
══════════════════════════════════════════════════════════════
`, listenURL("http", *httpAddress), listenURL("https", *webTransportAddress)+"/grimcraft", len(gameHub.List()), listenURL("https", *webTransportAddress)+"/grimcraft", hex.EncodeToString(fingerprint[:]))

	webTransportServer := &webtransport.Server{
		H3:          http3.Server{Addr: *webTransportAddress, TLSConfig: &tls.Config{Certificates: []tls.Certificate{certificate}}},
		CheckOrigin: func(request *http.Request) bool { return true },
	}
	webTransportRouter := http.NewServeMux()
	webTransportRouter.HandleFunc("/grimcraft", transport.WTHandler(gameHub, webTransportServer))
	webTransportServer.H3.Handler = webTransportRouter
	go func() {
		if currentOperationError := webTransportServer.ListenAndServe(); currentOperationError != nil {
			log.Fatalf("webtransport: %v", currentOperationError)
		}
	}()

	log.Printf("HTTP/WS+lobbies on %s", *httpAddress)
	log.Fatal(http.ListenAndServe(*httpAddress, router))
}

func ensureCert(certificateDirectory string) (tls.Certificate, []byte, error) {
	certificateFile := filepath.Join(certificateDirectory, "cert.pem")
	keyFile := filepath.Join(certificateDirectory, "key.pem")
	if rawCertificate, certificateReadError := os.ReadFile(certificateFile); certificateReadError == nil {
		if rawPrivateKey, privateKeyReadError := os.ReadFile(keyFile); privateKeyReadError == nil {
			if certificate, keyPairError := tls.X509KeyPair(rawCertificate, rawPrivateKey); keyPairError == nil {
				if parsedCertificate, certificateParseError := x509.ParseCertificate(certificate.Certificate[0]); certificateParseError == nil && browserCertificateValid(parsedCertificate, time.Now()) {
					return certificate, certificate.Certificate[0], nil
				}
			}
		}
	}
	privateKey, certificateError := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if certificateError != nil {
		return tls.Certificate{}, nil, certificateError
	}
	certificateTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: "grimcraft-wt"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(13 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}
	certificateDERBytes, certificateError := x509.CreateCertificate(rand.Reader, certificateTemplate, certificateTemplate, &privateKey.PublicKey, privateKey)
	if certificateError != nil {
		return tls.Certificate{}, nil, certificateError
	}
	if directoryCreationError := os.MkdirAll(certificateDirectory, 0o755); directoryCreationError != nil {
		return tls.Certificate{}, nil, directoryCreationError
	}
	certificateOutput, certificateError := os.Create(certificateFile)
	if certificateError != nil {
		return tls.Certificate{}, nil, certificateError
	}
	_ = pem.Encode(certificateOutput, &pem.Block{Type: "CERTIFICATE", Bytes: certificateDERBytes})
	_ = certificateOutput.Close()
	keyOutput, certificateError := os.OpenFile(keyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if certificateError != nil {
		return tls.Certificate{}, nil, certificateError
	}
	privateKeyDER, certificateError := x509.MarshalECPrivateKey(privateKey)
	if certificateError != nil {
		_ = keyOutput.Close()
		return tls.Certificate{}, nil, certificateError
	}
	_ = pem.Encode(keyOutput, &pem.Block{Type: "EC PRIVATE KEY", Bytes: privateKeyDER})
	_ = keyOutput.Close()
	currentCertificate, certificateError := tls.X509KeyPair(
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateDERBytes}),
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privateKeyDER}),
	)
	return currentCertificate, certificateDERBytes, certificateError
}

// WebTransport hash-pinned certificates must be ECDSA P-256 and valid for
// no more than 14 days. Renew legacy RSA, expired and near-expiry certificates.
func browserCertificateValid(certificate *x509.Certificate, now time.Time) bool {
	key, isValid := certificate.PublicKey.(*ecdsa.PublicKey)
	return isValid && key.Curve == elliptic.P256() && certificate.NotAfter.Sub(certificate.NotBefore) <= 14*24*time.Hour &&
		!now.Before(certificate.NotBefore) && now.Add(time.Hour).Before(certificate.NotAfter)
}

func listenURL(scheme, address string) string {
	host, port, operationError := net.SplitHostPort(address)
	if operationError != nil {
		return scheme + "://" + address
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return scheme + "://" + net.JoinHostPort(host, port)
}
