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
	"strings"
	"testing"
	"time"
)

func algoForPubKey(t *testing.T, certFile string) x509.PublicKeyAlgorithm {
	t.Helper()
	data, err := os.ReadFile(certFile)
	if err != nil {
		t.Fatalf("failed to read cert: %v", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		t.Fatalf("failed to decode cert PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse cert: %v", err)
	}
	return cert.PublicKeyAlgorithm
}

func TestGenerateSelfSignedCert_Success(t *testing.T) {
	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "cert.pem")
	keyFile := filepath.Join(tmpDir, "key.pem")

	err := GenerateSelfSignedCert(certFile, keyFile, localhostName)
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

	_ = GenerateSelfSignedCert(certFile, keyFile, localhostName)

	certData, _ := os.ReadFile(certFile)
	if !strings.Contains(string(certData), "BEGIN CERTIFICATE") {
		t.Fatal("cert file doesn't contain PEM certificate")
	}
	if !strings.Contains(string(certData), "END CERTIFICATE") {
		t.Fatal("cert file doesn't contain end marker")
	}
}

func TestGenerateSelfSignedCert_KeyContainsPEM(t *testing.T) {
	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "cert.pem")
	keyFile := filepath.Join(tmpDir, "key.pem")

	_ = GenerateSelfSignedCert(certFile, keyFile, localhostName)

	keyData, _ := os.ReadFile(keyFile)
	if !strings.Contains(string(keyData), "BEGIN PRIVATE KEY") {
		t.Fatal("key file doesn't contain EC private key")
	}
}

func TestGenerateSelfSignedCert_InvalidCertPath(t *testing.T) {
	keyFile := filepath.Join(t.TempDir(), "key.pem")
	err := GenerateSelfSignedCert("/nonexistent/path/cert.pem", keyFile, localhostName)
	if err == nil {
		t.Fatal("expected error for invalid cert path")
	}
}

func TestGenerateSelfSignedCert_InvalidKeyPath(t *testing.T) {
	certFile := filepath.Join(t.TempDir(), "cert.pem")
	err := GenerateSelfSignedCert(certFile, "/nonexistent/path/key.pem", localhostName)
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

	_ = GenerateSelfSignedCert(cert1, key1, localhostName)
	_ = GenerateSelfSignedCert(cert2, key2, localhostName)

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

	err := GenerateSelfSignedCert(certFile, keyFile, localhostName, "bedrud.example.com")
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
		if dns == localhostName {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected DNS SAN 'localhost', got %v", cert.DNSNames)
	}

	if len(cert.IPAddresses) < 2 {
		t.Fatalf("expected at least 2 IP SANs (127.0.0.1, ::1), got %d", len(cert.IPAddresses))
	}
	foundLoopback := false
	foundIPv6Loopback := false
	for _, ip := range cert.IPAddresses {
		if ip.Equal(net.ParseIP("127.0.0.1")) {
			foundLoopback = true
		}
		if ip.Equal(net.ParseIP("::1")) {
			foundIPv6Loopback = true
		}
	}
	if !foundLoopback {
		t.Fatalf("expected IP SAN 127.0.0.1, got %v", cert.IPAddresses)
	}
	if !foundIPv6Loopback {
		t.Fatalf("expected IP SAN ::1, got %v", cert.IPAddresses)
	}
}

func TestParseSanHosts(t *testing.T) {
	dns, ips := ParseSanHosts(localhostName, "bedrud.example.com", "192.168.1.100", "10.0.0.1")
	if len(dns) != 2 {
		t.Fatalf("expected 2 DNS names, got %d: %v", len(dns), dns)
	}
	if dns[0] != localhostName {
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
	dns, ips := ParseSanHosts("example.com", "192.168.1.1", localhostName, "10.0.0.1", "sub.example.org")
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

	if err := GenerateSelfSignedCert(certFile, keyFile, localhostName); err != nil {
		t.Fatalf("failed to generate cert: %v", err)
	}

	info, err := ValidateTLSCertPair(certFile, keyFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Status != "valid" {
		t.Fatalf("expected status 'valid', got '%s'", info.Status)
	}
	if info.DaysRemaining < 1820 || info.DaysRemaining > 1826 {
		t.Fatalf("expected ~1825 days remaining, got %d", info.DaysRemaining)
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
	_ = GenerateSelfSignedCert(filepath.Join(tmpDir, "x.pem"), keyFile, localhostName)

	_, err := ValidateTLSCertPair(filepath.Join(tmpDir, "nonexistent.pem"), keyFile)
	if err == nil {
		t.Fatal("expected error for missing cert file")
	}
	if !strings.Contains(err.Error(), "certificate file not found") {
		t.Fatalf("expected 'not found' error, got: %v", err)
	}
}

func TestValidateTLSCertPair_KeyNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "cert.pem")
	keyFile := filepath.Join(tmpDir, "key.pem")

	_ = GenerateSelfSignedCert(certFile, keyFile, localhostName)

	_, err := ValidateTLSCertPair(certFile, filepath.Join(tmpDir, "nonexistent.pem"))
	if err == nil {
		t.Fatal("expected error for missing key file")
	}
	if !strings.Contains(err.Error(), "key file not found") {
		t.Fatalf("expected 'not found' error, got: %v", err)
	}
}

func TestValidateTLSCertPair_KeyMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "cert.pem")
	keyFile := filepath.Join(tmpDir, "key.pem")
	otherKeyFile := filepath.Join(tmpDir, "other_key.pem")

	_ = GenerateSelfSignedCert(certFile, keyFile, localhostName)

	tmpDir2 := t.TempDir()
	_ = GenerateSelfSignedCert(filepath.Join(tmpDir2, "x.pem"), otherKeyFile, localhostName)

	_, err := ValidateTLSCertPair(certFile, otherKeyFile)
	if err == nil {
		t.Fatal("expected error for key mismatch")
	}
	if !strings.Contains(err.Error(), "mismatch") {
		t.Fatalf("expected 'mismatch' error, got: %v", err)
	}
}

func TestValidateTLSCertPair_InvalidPEM(t *testing.T) {
	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "cert.pem")
	keyFile := filepath.Join(tmpDir, "key.pem")

	_ = GenerateSelfSignedCert(certFile, keyFile, localhostName)

	_ = os.WriteFile(certFile, []byte("not a pem file"), 0o644)

	_, err := ValidateTLSCertPair(certFile, keyFile)
	if err == nil {
		t.Fatal("expected error for invalid PEM")
	}
	if !strings.Contains(err.Error(), "not a valid PEM file") {
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
		KeyUsage:     x509.KeyUsageDigitalSignature,
		DNSNames:     []string{localhostName},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatal(err)
	}

	certOut, _ := SafeCreate(certFile, 0o644)
	_ = pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	certOut.Close()

	keyOut, _ := SafeCreate(keyFile, 0o600)
	privBytes, _ := x509.MarshalPKCS8PrivateKey(priv)
	_ = pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes})
	keyOut.Close()

	_, err = ValidateTLSCertPair(certFile, keyFile)
	if err == nil {
		t.Fatal("expected error for expired cert")
	}
	if !strings.Contains(err.Error(), "expired") {
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
		KeyUsage:     x509.KeyUsageDigitalSignature,
		DNSNames:     []string{localhostName},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatal(err)
	}

	certOut, _ := SafeCreate(certFile, 0o644)
	_ = pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	certOut.Close()

	keyOut, _ := SafeCreate(keyFile, 0o600)
	privBytes, _ := x509.MarshalPKCS8PrivateKey(priv)
	_ = pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes})
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

func TestRenewSelfSignedCert_OverwritesExisting(t *testing.T) {
	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "cert.pem")
	keyFile := filepath.Join(tmpDir, "key.pem")

	if err := GenerateSelfSignedCert(certFile, keyFile, localhostName); err != nil {
		t.Fatalf("initial generation failed: %v", err)
	}

	originalCert, _ := os.ReadFile(certFile)

	if err := RenewSelfSignedCert(certFile, keyFile, localhostName, "example.com"); err != nil {
		t.Fatalf("renewal failed: %v", err)
	}

	renewedCert, _ := os.ReadFile(certFile)
	if bytes.Equal(originalCert, renewedCert) {
		t.Fatal("expected different certificate after renewal")
	}

	info, err := ValidateTLSCertPair(certFile, keyFile)
	if err != nil {
		t.Fatalf("validation after renewal failed: %v", err)
	}
	if info.Status != "valid" {
		t.Fatalf("expected status 'valid', got '%s'", info.Status)
	}

	foundExample := false
	for _, san := range info.SANs {
		if san == "example.com" {
			foundExample = true
		}
	}
	if !foundExample {
		t.Fatalf("expected SAN 'example.com', got %v", info.SANs)
	}
}

func TestRenewSelfSignedCert_DefaultsSANsWhenEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "cert.pem")
	keyFile := filepath.Join(tmpDir, "key.pem")

	if err := GenerateSelfSignedCert(certFile, keyFile); err != nil {
		t.Fatalf("initial generation failed: %v", err)
	}

	if err := RenewSelfSignedCert(certFile, keyFile); err != nil {
		t.Fatalf("renewal with no hosts failed: %v", err)
	}

	info, err := ValidateTLSCertPair(certFile, keyFile)
	if err != nil {
		t.Fatalf("validation failed: %v", err)
	}

	foundLocalhost := false
	for _, san := range info.SANs {
		if san == localhostName {
			foundLocalhost = true
		}
	}
	if !foundLocalhost {
		t.Fatalf("expected SAN 'localhost' in default renewal, got %v", info.SANs)
	}
}

func TestRenewSelfSignedCert_NoTempFilesLeftOnError(t *testing.T) {
	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "cert.pem")
	keyFile := filepath.Join(tmpDir, "key.pem")

	if err := GenerateSelfSignedCert(certFile, keyFile, localhostName); err != nil {
		t.Fatalf("initial generation failed: %v", err)
	}

	keyFileBad := filepath.Join(tmpDir, "nonexistent", "key.pem")
	err := RenewSelfSignedCert(certFile, keyFileBad, localhostName)
	if err == nil {
		t.Fatal("expected error for bad key path")
	}

	entries, _ := os.ReadDir(tmpDir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".new") {
			t.Fatalf("temp file left behind: %s", e.Name())
		}
	}
}

func TestGenerateSelfSignedCert_CleanupOnKeyFailure(t *testing.T) {
	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "cert.pem")
	keyFile := filepath.Join(tmpDir, "subdir", "key.pem")

	err := GenerateSelfSignedCert(certFile, keyFile, localhostName)
	if err == nil {
		t.Fatal("expected error for nonexistent key parent dir")
	}

	if _, statErr := os.Stat(certFile); !os.IsNotExist(statErr) {
		t.Fatal("expected cert file to be cleaned up after key creation failure")
	}
}

func TestGenerateSelfSignedCertWithAlgo_Ed25519(t *testing.T) {
	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "cert.pem")
	keyFile := filepath.Join(tmpDir, "key.pem")

	err := GenerateSelfSignedCertWithAlgo(certFile, keyFile, KeyEd25519, localhostName)
	if err != nil {
		t.Fatalf("failed to generate Ed25519 cert: %v", err)
	}
	if algoForPubKey(t, certFile) != x509.Ed25519 {
		t.Fatal("expected Ed25519 public key algorithm")
	}
}

func TestGenerateSelfSignedCertWithAlgo_ECDSA256(t *testing.T) {
	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "cert.pem")
	keyFile := filepath.Join(tmpDir, "key.pem")

	err := GenerateSelfSignedCertWithAlgo(certFile, keyFile, KeyECDSA256, localhostName)
	if err != nil {
		t.Fatalf("failed to generate ECDSA P256 cert: %v", err)
	}
	if algoForPubKey(t, certFile) != x509.ECDSA {
		t.Fatal("expected ECDSA public key algorithm")
	}
}

func TestGenerateSelfSignedCertWithAlgo_RSA2048(t *testing.T) {
	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "cert.pem")
	keyFile := filepath.Join(tmpDir, "key.pem")

	err := GenerateSelfSignedCertWithAlgo(certFile, keyFile, KeyRSA2048, localhostName)
	if err != nil {
		t.Fatalf("failed to generate RSA 2048 cert: %v", err)
	}

	data, _ := os.ReadFile(certFile)
	block, _ := pem.Decode(data)
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse RSA cert: %v", err)
	}
	if cert.PublicKeyAlgorithm != x509.RSA {
		t.Fatal("expected RSA public key algorithm")
	}
	if cert.KeyUsage&x509.KeyUsageKeyEncipherment == 0 {
		t.Fatal("expected KeyUsageKeyEncipherment for RSA cert")
	}
	if cert.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
		t.Fatal("expected KeyUsageDigitalSignature for RSA cert")
	}
}

func TestGenerateSelfSignedCertDefault_IsEd25519(t *testing.T) {
	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "cert.pem")
	keyFile := filepath.Join(tmpDir, "key.pem")

	err := GenerateSelfSignedCert(certFile, keyFile, localhostName)
	if err != nil {
		t.Fatalf("failed to generate default cert: %v", err)
	}
	if algoForPubKey(t, certFile) != x509.Ed25519 {
		t.Fatal("default GenerateSelfSignedCert should use Ed25519")
	}
}

func TestRenewSelfSignedCert_PreservesECDSAlgo(t *testing.T) {
	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "cert.pem")
	keyFile := filepath.Join(tmpDir, "key.pem")

	if err := GenerateSelfSignedCertWithAlgo(certFile, keyFile, KeyECDSA256, localhostName); err != nil {
		t.Fatalf("initial ECDSA generation failed: %v", err)
	}

	if err := RenewSelfSignedCert(certFile, keyFile, localhostName); err != nil {
		t.Fatalf("renewal failed: %v", err)
	}

	if algoForPubKey(t, certFile) != x509.ECDSA {
		t.Fatal("renewal should preserve ECDSA algorithm from existing cert")
	}
}

func TestRenewSelfSignedCert_PreservesEd25519Algo(t *testing.T) {
	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "cert.pem")
	keyFile := filepath.Join(tmpDir, "key.pem")

	if err := GenerateSelfSignedCertWithAlgo(certFile, keyFile, KeyEd25519, localhostName); err != nil {
		t.Fatalf("initial Ed25519 generation failed: %v", err)
	}

	if err := RenewSelfSignedCert(certFile, keyFile, localhostName); err != nil {
		t.Fatalf("renewal failed: %v", err)
	}

	if algoForPubKey(t, certFile) != x509.Ed25519 {
		t.Fatal("renewal should preserve Ed25519 algorithm from existing cert")
	}
}

func TestRenewSelfSignedCertWithAlgo_OverridesAlgo(t *testing.T) {
	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "cert.pem")
	keyFile := filepath.Join(tmpDir, "key.pem")

	if err := GenerateSelfSignedCertWithAlgo(certFile, keyFile, KeyECDSA256, localhostName); err != nil {
		t.Fatalf("initial ECDSA generation failed: %v", err)
	}

	if err := RenewSelfSignedCertWithAlgo(certFile, keyFile, KeyEd25519, localhostName); err != nil {
		t.Fatalf("renewal with override failed: %v", err)
	}

	if algoForPubKey(t, certFile) != x509.Ed25519 {
		t.Fatal("RenewSelfSignedCertWithAlgo should override existing algo")
	}
}

func TestDetectCertAlgorithm_Ed25519(t *testing.T) {
	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "cert.pem")
	keyFile := filepath.Join(tmpDir, "key.pem")

	if err := GenerateSelfSignedCertWithAlgo(certFile, keyFile, KeyEd25519, localhostName); err != nil {
		t.Fatalf("generation failed: %v", err)
	}

	algo, err := DetectCertAlgorithm(certFile)
	if err != nil {
		t.Fatalf("DetectCertAlgorithm failed: %v", err)
	}
	if algo != KeyEd25519 {
		t.Fatalf("expected KeyEd25519, got %s", algo)
	}
}

func TestDetectCertAlgorithm_ECDSA256(t *testing.T) {
	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "cert.pem")
	keyFile := filepath.Join(tmpDir, "key.pem")

	if err := GenerateSelfSignedCertWithAlgo(certFile, keyFile, KeyECDSA256, localhostName); err != nil {
		t.Fatalf("generation failed: %v", err)
	}

	algo, err := DetectCertAlgorithm(certFile)
	if err != nil {
		t.Fatalf("DetectCertAlgorithm failed: %v", err)
	}
	if algo != KeyECDSA256 {
		t.Fatalf("expected KeyECDSA256, got %s", algo)
	}
}

func TestDetectCertAlgorithm_RSA2048(t *testing.T) {
	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "cert.pem")
	keyFile := filepath.Join(tmpDir, "key.pem")

	if err := GenerateSelfSignedCertWithAlgo(certFile, keyFile, KeyRSA2048, localhostName); err != nil {
		t.Fatalf("generation failed: %v", err)
	}

	algo, err := DetectCertAlgorithm(certFile)
	if err != nil {
		t.Fatalf("DetectCertAlgorithm failed: %v", err)
	}
	if algo != KeyRSA2048 {
		t.Fatalf("expected KeyRSA2048, got %s", algo)
	}
}

func TestDetectCertAlgorithm_FileNotFound(t *testing.T) {
	_, err := DetectCertAlgorithm("/nonexistent/cert.pem")
	if err == nil {
		t.Fatal("expected error for nonexistent cert")
	}
	if !strings.Contains(err.Error(), "cannot read cert file") {
		t.Fatalf("expected 'cannot read cert file' error, got: %v", err)
	}
}

func TestDetectCertAlgorithm_InvalidPEM(t *testing.T) {
	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "cert.pem")
	if err := os.WriteFile(certFile, []byte("not a pem file"), 0o644); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	_, err := DetectCertAlgorithm(certFile)
	if err == nil {
		t.Fatal("expected error for invalid PEM")
	}
	if !strings.Contains(err.Error(), "no PEM data") {
		t.Fatalf("expected 'no PEM data' error, got: %v", err)
	}
}

func TestKeyUsageForAlgo_Ed25519(t *testing.T) {
	ku := keyUsageForAlgo(KeyEd25519)
	if ku != x509.KeyUsageDigitalSignature {
		t.Fatal("Ed25519 should have only KeyUsageDigitalSignature")
	}
}

func TestKeyUsageForAlgo_ECDSA256(t *testing.T) {
	ku := keyUsageForAlgo(KeyECDSA256)
	if ku != x509.KeyUsageDigitalSignature {
		t.Fatal("ECDSA should have only KeyUsageDigitalSignature")
	}
}

func TestKeyUsageForAlgo_RSA(t *testing.T) {
	ku := keyUsageForAlgo(KeyRSA2048)
	if ku&x509.KeyUsageKeyEncipherment == 0 {
		t.Fatal("RSA should have KeyUsageKeyEncipherment")
	}
	if ku&x509.KeyUsageDigitalSignature == 0 {
		t.Fatal("RSA should have KeyUsageDigitalSignature")
	}
	ku4096 := keyUsageForAlgo(KeyRSA4096)
	if ku4096&x509.KeyUsageKeyEncipherment == 0 {
		t.Fatal("RSA 4096 should have KeyUsageKeyEncipherment")
	}
	if ku4096&x509.KeyUsageDigitalSignature == 0 {
		t.Fatal("RSA 4096 should have KeyUsageDigitalSignature")
	}
}

func TestGenerateKey_UnsupportedAlgo(t *testing.T) {
	_, err := generateKey("invalid")
	if err == nil {
		t.Fatal("expected error for unsupported algorithm")
	}
	if !strings.Contains(err.Error(), "unsupported key algorithm") {
		t.Fatalf("expected 'unsupported key algorithm' error, got: %v", err)
	}
}
