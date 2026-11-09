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
	httpAddr := flag.String("http", ":8080", "HTTP/WebSocket/lobby listen address")
	wtAddr := flag.String("wt", ":4433", "WebTransport (QUIC) listen address")
	certDir := flag.String("cert-dir", "certs", "directory for the self-signed WT certificate")
	skipWebTransport := flag.Bool("no-wt", false, "disable WebTransport, serve HTTP/WS only")
	flag.Parse()

	gameHub := hub.NewHub()

	router := http.NewServeMux()
	router.Handle("/api/", server.API(gameHub))
	router.HandleFunc("/join", transport.WSHandler(gameHub))
	router.HandleFunc("/grimcraft", func(writer http.ResponseWriter, request *http.Request) {
		http.Error(writer, "use WebTransport (QUIC) for this endpoint", http.StatusBadRequest)
	})
	router.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Access-Control-Allow-Origin", "*")
		fmt.Fprintln(writer, "grimcraft-go multiplayer server. WS /join, API /api/lobbies, WT /grimcraft (QUIC).")
	})

	if *skipWebTransport {
		log.Printf("HTTP/WS+Lobbies on %s (WebTransport disabled)", *httpAddr)
		log.Fatal(http.ListenAndServe(*httpAddr, router))
	}

	cert, certDER, err := ensureCert(*certDir)
	if err != nil {
		log.Fatalf("cert: %v", err)
	}
	fingerprint := sha256.Sum256(certDER)
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
`, listenURL("http", *httpAddr), listenURL("https", *wtAddr)+"/grimcraft", len(gameHub.List()), listenURL("https", *wtAddr)+"/grimcraft", hex.EncodeToString(fingerprint[:]))

	wtServer := &webtransport.Server{
		H3:          http3.Server{Addr: *wtAddr, TLSConfig: &tls.Config{Certificates: []tls.Certificate{cert}}},
		CheckOrigin: func(request *http.Request) bool { return true },
	}
	wtRouter := http.NewServeMux()
	wtRouter.HandleFunc("/grimcraft", transport.WTHandler(gameHub, wtServer))
	wtServer.H3.Handler = wtRouter
	go func() {
		if err := wtServer.ListenAndServe(); err != nil {
			log.Fatalf("webtransport: %v", err)
		}
	}()

	log.Printf("HTTP/WS+lobbies on %s", *httpAddr)
	log.Fatal(http.ListenAndServe(*httpAddr, router))
}

func ensureCert(certDir string) (tls.Certificate, []byte, error) {
	certFile := filepath.Join(certDir, "cert.pem")
	keyFile := filepath.Join(certDir, "key.pem")
	if rawCert, err := os.ReadFile(certFile); err == nil {
		if rawKey, err := os.ReadFile(keyFile); err == nil {
			if cert, err := tls.X509KeyPair(rawCert, rawKey); err == nil {
				if leaf, err := x509.ParseCertificate(cert.Certificate[0]); err == nil && browserCertificateValid(leaf, time.Now()) {
					return cert, cert.Certificate[0], nil
				}
			}
		}
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	certTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: "grimcraft-wt"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(13 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, certTemplate, certTemplate, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	if err := os.MkdirAll(certDir, 0o755); err != nil {
		return tls.Certificate{}, nil, err
	}
	certOut, err := os.Create(certFile)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	_ = pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	_ = certOut.Close()
	keyOut, err := os.OpenFile(keyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		_ = keyOut.Close()
		return tls.Certificate{}, nil, err
	}
	_ = pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	_ = keyOut.Close()
	cert, err := tls.X509KeyPair(
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes}),
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}),
	)
	return cert, derBytes, err
}

// WebTransport hash-pinned certificates must be ECDSA P-256 and valid for
// no more than 14 days. Renew legacy RSA, expired and near-expiry certificates.
func browserCertificateValid(cert *x509.Certificate, now time.Time) bool {
	key, ok := cert.PublicKey.(*ecdsa.PublicKey)
	return ok && key.Curve == elliptic.P256() && cert.NotAfter.Sub(cert.NotBefore) <= 14*24*time.Hour &&
		!now.Before(cert.NotBefore) && now.Add(time.Hour).Before(cert.NotAfter)
}

func listenURL(scheme, addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return scheme + "://" + addr
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return scheme + "://" + net.JoinHostPort(host, port)
}
