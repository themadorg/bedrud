package utils

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGenerateSelfSignedCert_Success(t *testing.T) {
	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "cert.pem")
	keyFile := filepath.Join(tmpDir, "key.pem")

	err := GenerateSelfSignedCert(certFile, keyFile, "localhost")
	if err != nil {
		t.Fatalf("failed to generate cert: %v", err)
	}

	// Verify cert file exists and is non-empty
	certInfo, err := os.Stat(certFile)
	if err != nil {
		t.Fatalf("cert file not created: %v", err)
	}
	if certInfo.Size() == 0 {
		t.Fatal("cert file is empty")
	}

	// Verify key file exists and is non-empty
	keyInfo, err := os.Stat(keyFile)
	if err != nil {
		t.Fatalf("key file not created: %v", err)
	}
	if keyInfo.Size() == 0 {
		t.Fatal("key file is empty")
	}
}

func TestGenerateSelfSignedCert_CertContainsPEM(t *testing.T) {
	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "cert.pem")
	keyFile := filepath.Join(tmpDir, "key.pem")

	_ = GenerateSelfSignedCert(certFile, keyFile, "localhost")

	certData, _ := os.ReadFile(certFile)
	if !containsSubstring(string(certData), "BEGIN CERTIFICATE") {
		t.Fatal("cert file doesn't contain PEM certificate")
	}
	if !containsSubstring(string(certData), "END CERTIFICATE") {
		t.Fatal("cert file doesn't contain end marker")
	}
}

func TestGenerateSelfSignedCert_KeyContainsPEM(t *testing.T) {
	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "cert.pem")
	keyFile := filepath.Join(tmpDir, "key.pem")

	_ = GenerateSelfSignedCert(certFile, keyFile, "localhost")

	keyData, _ := os.ReadFile(keyFile)
	if !containsSubstring(string(keyData), "BEGIN EC PRIVATE KEY") {
		t.Fatal("key file doesn't contain EC private key")
	}
}

func TestGenerateSelfSignedCert_InvalidCertPath(t *testing.T) {
	keyFile := filepath.Join(t.TempDir(), "key.pem")
	err := GenerateSelfSignedCert("/nonexistent/path/cert.pem", keyFile, "localhost")
	if err == nil {
		t.Fatal("expected error for invalid cert path")
	}
}

func TestGenerateSelfSignedCert_InvalidKeyPath(t *testing.T) {
	certFile := filepath.Join(t.TempDir(), "cert.pem")
	err := GenerateSelfSignedCert(certFile, "/nonexistent/path/key.pem", "localhost")
	if err == nil {
		t.Fatal("expected error for invalid key path")
	}
}

func TestGenerateSelfSignedCert_MultipleGenerations(t *testing.T) {
	tmpDir := t.TempDir()
	cert1 := filepath.Join(tmpDir, "cert1.pem")
	key1 := filepath.Join(tmpDir, "key1.pem")
	cert2 := filepath.Join(tmpDir, "cert2.pem")
	key2 := filepath.Join(tmpDir, "key2.pem")

	_ = GenerateSelfSignedCert(cert1, key1, "localhost")
	_ = GenerateSelfSignedCert(cert2, key2, "localhost")

	data1, _ := os.ReadFile(cert1)
	data2, _ := os.ReadFile(cert2)

	// Different certs should be generated each time (different serial numbers, keys)
	if bytes.Equal(data1, data2) {
		t.Fatal("expected different certificates for each generation")
	}
}

func TestGenerateSelfSignedCert_WithDnsSan(t *testing.T) {
	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "cert.pem")
	keyFile := filepath.Join(tmpDir, "key.pem")

	err := GenerateSelfSignedCert(certFile, keyFile, "localhost", "bedrud.example.com")
	if err != nil {
		t.Fatalf("failed to generate cert: %v", err)
	}

	certData, _ := os.ReadFile(certFile)
	block, _ := pem.Decode(certData)
	if block == nil {
		t.Fatal("failed to decode PEM cert")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse cert: %v", err)
	}

	if len(cert.DNSNames) == 0 {
		t.Fatal("expected DNS SANs but got none")
	}

	found := false
	for _, dns := range cert.DNSNames {
		if dns == "bedrud.example.com" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected DNS SAN 'bedrud.example.com', got %v", cert.DNSNames)
	}
}

func TestGenerateSelfSignedCert_WithIpSan(t *testing.T) {
	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "cert.pem")
	keyFile := filepath.Join(tmpDir, "key.pem")

	err := GenerateSelfSignedCert(certFile, keyFile, "192.168.1.100")
	if err != nil {
		t.Fatalf("failed to generate cert: %v", err)
	}

	certData, _ := os.ReadFile(certFile)
	block, _ := pem.Decode(certData)
	if block == nil {
		t.Fatal("failed to decode PEM cert")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse cert: %v", err)
	}

	if len(cert.IPAddresses) == 0 {
		t.Fatal("expected IP SANs but got none")
	}

	expected := net.ParseIP("192.168.1.100")
	found := false
	for _, ip := range cert.IPAddresses {
		if ip.Equal(expected) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected IP SAN 192.168.1.100, got %v", cert.IPAddresses)
	}
}

func TestGenerateSelfSignedCert_EmptyHostsDefaultsToLocalhost(t *testing.T) {
	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "cert.pem")
	keyFile := filepath.Join(tmpDir, "key.pem")

	err := GenerateSelfSignedCert(certFile, keyFile)
	if err != nil {
		t.Fatalf("failed to generate cert: %v", err)
	}

	certData, _ := os.ReadFile(certFile)
	block, _ := pem.Decode(certData)
	if block == nil {
		t.Fatal("failed to decode PEM cert")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse cert: %v", err)
	}

	if len(cert.DNSNames) == 0 {
		t.Fatal("expected at least 'localhost' DNS SAN when no hosts given")
	}

	found := false
	for _, dns := range cert.DNSNames {
		if dns == "localhost" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected DNS SAN 'localhost', got %v", cert.DNSNames)
	}
}

func TestParseSanHosts(t *testing.T) {
	dns, ips := ParseSanHosts("localhost", "bedrud.example.com", "192.168.1.100", "10.0.0.1")
	if len(dns) != 2 {
		t.Fatalf("expected 2 DNS names, got %d: %v", len(dns), dns)
	}
	if dns[0] != "localhost" {
		t.Fatalf("expected dns[0]='localhost', got '%s'", dns[0])
	}
	if dns[1] != "bedrud.example.com" {
		t.Fatalf("expected dns[1]='bedrud.example.com', got '%s'", dns[1])
	}
	if len(ips) != 2 {
		t.Fatalf("expected 2 IPs, got %d: %v", len(ips), ips)
	}
	if !ips[0].Equal(net.ParseIP("192.168.1.100")) {
		t.Fatalf("expected ips[0]=192.168.1.100, got %v", ips[0])
	}
	if !ips[1].Equal(net.ParseIP("10.0.0.1")) {
		t.Fatalf("expected ips[1]=10.0.0.1, got %v", ips[1])
	}
}

func TestParseSanHosts_Empty(t *testing.T) {
	dns, ips := ParseSanHosts()
	if len(dns) != 0 {
		t.Fatalf("expected 0 DNS names, got %d", len(dns))
	}
	if len(ips) != 0 {
		t.Fatalf("expected 0 IPs, got %d", len(ips))
	}
}

func TestParseSanHosts_Mixed(t *testing.T) {
	dns, ips := ParseSanHosts("example.com", "192.168.1.1", "localhost", "10.0.0.1", "sub.example.org")
	if len(dns) != 3 {
		t.Fatalf("expected 3 DNS names, got %d: %v", len(dns), dns)
	}
	if len(ips) != 2 {
		t.Fatalf("expected 2 IPs, got %d: %v", len(ips), ips)
	}
}

func TestValidateTLSCertPair_ValidCert(t *testing.T) {
	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "cert.pem")
	keyFile := filepath.Join(tmpDir, "key.pem")

	if err := GenerateSelfSignedCert(certFile, keyFile, "localhost"); err != nil {
		t.Fatalf("failed to generate cert: %v", err)
	}

	info, err := ValidateTLSCertPair(certFile, keyFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Status != "valid" {
		t.Fatalf("expected status 'valid', got '%s'", info.Status)
	}
	if info.DaysRemaining < 360 || info.DaysRemaining > 366 {
		t.Fatalf("expected ~365 days remaining, got %d", info.DaysRemaining)
	}
	if info.Subject == "" {
		t.Fatal("expected non-empty subject")
	}
	if info.Issuer == "" {
		t.Fatal("expected non-empty issuer")
	}
	if len(info.SANs) == 0 {
		t.Fatal("expected at least one SAN")
	}
}

func TestValidateTLSCertPair_CertNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	keyFile := filepath.Join(tmpDir, "key.pem")
	_ = GenerateSelfSignedCert(filepath.Join(tmpDir, "x.pem"), keyFile, "localhost")

	_, err := ValidateTLSCertPair(filepath.Join(tmpDir, "nonexistent.pem"), keyFile)
	if err == nil {
		t.Fatal("expected error for missing cert file")
	}
	if !containsSubstring(err.Error(), "certificate file not found") {
		t.Fatalf("expected 'not found' error, got: %v", err)
	}
}

func TestValidateTLSCertPair_KeyNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "cert.pem")
	keyFile := filepath.Join(tmpDir, "key.pem")

	_ = GenerateSelfSignedCert(certFile, keyFile, "localhost")

	_, err := ValidateTLSCertPair(certFile, filepath.Join(tmpDir, "nonexistent.pem"))
	if err == nil {
		t.Fatal("expected error for missing key file")
	}
	if !containsSubstring(err.Error(), "key file not found") {
		t.Fatalf("expected 'not found' error, got: %v", err)
	}
}

func TestValidateTLSCertPair_KeyMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "cert.pem")
	keyFile := filepath.Join(tmpDir, "key.pem")
	otherKeyFile := filepath.Join(tmpDir, "other_key.pem")

	_ = GenerateSelfSignedCert(certFile, keyFile, "localhost")

	tmpDir2 := t.TempDir()
	_ = GenerateSelfSignedCert(filepath.Join(tmpDir2, "x.pem"), otherKeyFile, "localhost")

	_, err := ValidateTLSCertPair(certFile, otherKeyFile)
	if err == nil {
		t.Fatal("expected error for key mismatch")
	}
	if !containsSubstring(err.Error(), "mismatch") {
		t.Fatalf("expected 'mismatch' error, got: %v", err)
	}
}

func TestValidateTLSCertPair_InvalidPEM(t *testing.T) {
	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "cert.pem")
	keyFile := filepath.Join(tmpDir, "key.pem")

	_ = GenerateSelfSignedCert(certFile, keyFile, "localhost")

	_ = os.WriteFile(certFile, []byte("not a pem file"), 0o644)

	_, err := ValidateTLSCertPair(certFile, keyFile)
	if err == nil {
		t.Fatal("expected error for invalid PEM")
	}
	if !containsSubstring(err.Error(), "not a valid PEM file") {
		t.Fatalf("expected PEM decode error, got: %v", err)
	}
}

func TestValidateTLSCertPair_ExpiredCert(t *testing.T) {
	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "cert.pem")
	keyFile := filepath.Join(tmpDir, "key.pem")

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	serialNumber, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-72 * time.Hour),
		NotAfter:     time.Now().Add(-24 * time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		DNSNames:     []string{"localhost"},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatal(err)
	}

	certOut, _ := os.Create(certFile)
	_ = pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	certOut.Close()

	keyOut, _ := os.OpenFile(keyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	privBytes, _ := x509.MarshalECPrivateKey(priv)
	_ = pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes})
	keyOut.Close()

	_, err = ValidateTLSCertPair(certFile, keyFile)
	if err == nil {
		t.Fatal("expected error for expired cert")
	}
	if !containsSubstring(err.Error(), "expired") {
		t.Fatalf("expected 'expired' error, got: %v", err)
	}
}

func TestValidateTLSCertPair_ExpiringSoon(t *testing.T) {
	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "cert.pem")
	keyFile := filepath.Join(tmpDir, "key.pem")

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	serialNumber, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-24 * time.Hour),
		NotAfter:     time.Now().Add(10 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		DNSNames:     []string{"localhost"},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatal(err)
	}

	certOut, _ := os.Create(certFile)
	_ = pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	certOut.Close()

	keyOut, _ := os.OpenFile(keyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	privBytes, _ := x509.MarshalECPrivateKey(priv)
	_ = pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes})
	keyOut.Close()

	info, err := ValidateTLSCertPair(certFile, keyFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Status != "expiring" {
		t.Fatalf("expected status 'expiring', got '%s'", info.Status)
	}
	if info.DaysRemaining > CertWarnDays {
		t.Fatalf("expected <= %d days remaining, got %d", CertWarnDays, info.DaysRemaining)
	}
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
