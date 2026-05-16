package main

import (
	"crypto/x509"
	"encoding/pem"
	"testing"
)

func TestGeneratedCertificateCoversDataPlaneHosts(t *testing.T) {
	_, certPEM, err := generateAndSaveCert(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(certPEM)
	if block == nil {
		t.Fatal("expected certificate PEM block")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	if err := cert.VerifyHostname("myappconfig.azconfig.io"); err != nil {
		t.Fatal(err)
	}
	if err := cert.VerifyHostname("myvault.vault.azure.net"); err != nil {
		t.Fatal(err)
	}
}
