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

func TestEnsureCertBrowserCompatibleAndReused(t *testing.T) {
	dir := t.TempDir()
	_, der, err := ensureCert(dir)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	if !browserCertificateValid(cert, time.Now()) {
		t.Fatal("certificate not accepted for browser hash pinning")
	}
	if !bytes.Equal(cert.IPAddresses[0], []byte{127, 0, 0, 1}) && cert.IPAddresses[0].String() != "127.0.0.1" {
		t.Fatal("missing loopback SAN")
	}
	_, reused, err := ensureCert(dir)
	if err != nil || !bytes.Equal(der, reused) {
		t.Fatal("valid certificate should retain its fingerprint", err)
	}
	info, err := os.Stat(filepath.Join(dir, "key.pem"))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("private key permissions", err)
	}
}

func TestBrowserCertificateRejectsLegacyAndExpired(t *testing.T) {
	now := time.Now()
	ec, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	good := x509.Certificate{PublicKey: &ec.PublicKey, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(13 * 24 * time.Hour)}
	for _, test := range []struct {
		name string
		cert x509.Certificate
	}{
		{"RSA", x509.Certificate{PublicKey: &rsaKey.PublicKey, NotBefore: good.NotBefore, NotAfter: good.NotAfter}},
		{"year-long", x509.Certificate{PublicKey: good.PublicKey, NotBefore: good.NotBefore, NotAfter: now.Add(365 * 24 * time.Hour)}},
		{"expired", x509.Certificate{PublicKey: good.PublicKey, NotBefore: now.Add(-13 * 24 * time.Hour), NotAfter: now.Add(-time.Hour)}},
		{"not-yet-valid", x509.Certificate{PublicKey: good.PublicKey, NotBefore: now.Add(time.Hour), NotAfter: good.NotAfter}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if browserCertificateValid(&test.cert, now) {
				t.Fatal("must reject certificate")
			}
		})
	}
}

func TestListenURL(t *testing.T) {
	if listenURL("https", ":4433") != "https://127.0.0.1:4433" {
		t.Fatal("default address")
	}
	if listenURL("https", "127.0.0.1:4433") != "https://127.0.0.1:4433" {
		t.Fatal("explicit loopback")
	}
}
