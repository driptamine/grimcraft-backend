package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEnsureCertBrowserCompatibleAndReused(testCase *testing.T) {
	dir := testCase.TempDir()
	_, certificateDER, operationError := ensureCert(dir)
	if operationError != nil {
		testCase.Fatal(operationError)
	}
	certificate, operationError := x509.ParseCertificate(certificateDER)
	if operationError != nil {
		testCase.Fatal(operationError)
	}
	if !browserCertificateValid(certificate, time.Now()) {
		testCase.Fatal("certificate not accepted for browser hash pinning")
	}
	if !bytes.Equal(certificate.IPAddresses[0], []byte{127, 0, 0, 1}) && certificate.IPAddresses[0].String() != "127.0.0.1" {
		testCase.Fatal("missing loopback SAN")
	}
	_, reused, operationError := ensureCert(dir)
	if operationError != nil || !bytes.Equal(certificateDER, reused) {
		testCase.Fatal("valid certificate should retain its fingerprint", operationError)
	}
	info, operationError := os.Stat(filepath.Join(dir, "key.pem"))
	if operationError != nil || info.Mode().Perm() != 0600 {
		testCase.Fatal("private key permissions", operationError)
	}
}

func TestBrowserCertificateRejectsLegacyAndExpired(testCase *testing.T) {
	now := time.Now()
	privateKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	good := x509.Certificate{PublicKey: &privateKey.PublicKey, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(13 * 24 * time.Hour)}
	for _, test := range []struct {
		name string
		cert x509.Certificate
	}{
		{"RSA", x509.Certificate{PublicKey: &rsaKey.PublicKey, NotBefore: good.NotBefore, NotAfter: good.NotAfter}},
		{"year-long", x509.Certificate{PublicKey: good.PublicKey, NotBefore: good.NotBefore, NotAfter: now.Add(365 * 24 * time.Hour)}},
		{"expired", x509.Certificate{PublicKey: good.PublicKey, NotBefore: now.Add(-13 * 24 * time.Hour), NotAfter: now.Add(-time.Hour)}},
		{"not-yet-valid", x509.Certificate{PublicKey: good.PublicKey, NotBefore: now.Add(time.Hour), NotAfter: good.NotAfter}},
	} {
		testCase.Run(test.name, func(currentTestCase *testing.T) {
			if browserCertificateValid(&test.cert, now) {
				currentTestCase.Fatal("must reject certificate")
			}
		})
	}
}

func TestListenURL(testCase *testing.T) {
	if listenURL("https", ":4433") != "https://127.0.0.1:4433" {
		testCase.Fatal("default address")
	}
	if listenURL("https", "127.0.0.1:4433") != "https://127.0.0.1:4433" {
		testCase.Fatal("explicit loopback")
	}
}
