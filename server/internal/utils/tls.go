package utils

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

const (
	localhostName   = "localhost"
	certStatusValid = "valid"
)

const (
	CertWarnDays       = 30
	SelfSignedCertDays = 1825
)

type KeyAlgorithm string

const (
	KeyEd25519  KeyAlgorithm = "ed25519"
	KeyECDSA256 KeyAlgorithm = "ecdsa256"
	KeyRSA2048  KeyAlgorithm = "rsa2048"
	KeyRSA4096  KeyAlgorithm = "rsa4096"
)

func GenerateSelfSignedCert(certFile, keyFile string, hosts ...string) error {
	return GenerateSelfSignedCertWithAlgo(certFile, keyFile, KeyEd25519, hosts...)
}

func GenerateSelfSignedCertWithAlgo(certFile, keyFile string, algo KeyAlgorithm, hosts ...string) error {
	return generateCertToFiles(certFile, keyFile, algo, hosts...)
}

func RenewSelfSignedCert(certFile, keyFile string, hosts ...string) error {
	algo, err := DetectCertAlgorithm(certFile)
	if err != nil {
		return fmt.Errorf("failed to detect cert algorithm: %w", err)
	}
	return RenewSelfSignedCertWithAlgo(certFile, keyFile, algo, hosts...)
}

func RenewSelfSignedCertWithAlgo(certFile, keyFile string, algo KeyAlgorithm, hosts ...string) error {
	dnsNames, ipAddrs := ParseSanHosts(hosts...)
	if len(dnsNames) == 0 && len(ipAddrs) == 0 {
		dnsNames = []string{localhostName}
		ipAddrs = []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}
	}

	priv, err := generateKey(algo)
	if err != nil {
		return err
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(time.Duration(SelfSignedCertDays) * 24 * time.Hour)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return fmt.Errorf("failed to generate serial number: %w", err)
	}

	commonName := localhostName
	if len(dnsNames) > 0 {
		commonName = dnsNames[0]
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Bedrud Open Source"},
			CommonName:   commonName,
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              keyUsageForAlgo(algo),
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              dnsNames,
		IPAddresses:           ipAddrs,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, priv.Public(), priv)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %w", err)
	}

	certTmp := certFile + ".new"
	keyTmp := keyFile + ".new"

	certDir := filepath.Dir(certFile)
	if err := os.MkdirAll(certDir, 0o755); err != nil {
		return fmt.Errorf("failed to create cert directory %s: %w", certDir, err)
	}

	certOut, err := os.OpenFile(certTmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("failed to create temp cert file: %w", err)
	}
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		certOut.Close()
		os.Remove(certTmp)
		return fmt.Errorf("failed to write certificate PEM: %w", err)
	}
	certOut.Close()

	keyOut, err := os.OpenFile(keyTmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		os.Remove(certTmp)
		return fmt.Errorf("failed to create temp key file: %w", err)
	}
	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		keyOut.Close()
		os.Remove(certTmp)
		os.Remove(keyTmp)
		return fmt.Errorf("failed to marshal private key: %w", err)
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		keyOut.Close()
		os.Remove(certTmp)
		os.Remove(keyTmp)
		return fmt.Errorf("failed to write key PEM: %w", err)
	}
	keyOut.Close()

	if err := os.Rename(certTmp, certFile); err != nil {
		os.Remove(certTmp)
		os.Remove(keyTmp)
		return fmt.Errorf("failed to rename cert file: %w", err)
	}
	if err := os.Rename(keyTmp, keyFile); err != nil {
		os.Remove(keyTmp)
		return fmt.Errorf("failed to rename key file: %w", err)
	}

	return nil
}

func generateCertToFiles(certFile, keyFile string, algo KeyAlgorithm, hosts ...string) error {
	priv, err := generateKey(algo)
	if err != nil {
		return err
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(time.Duration(SelfSignedCertDays) * 24 * time.Hour)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return fmt.Errorf("failed to generate serial number: %w", err)
	}

	dnsNames, ipAddrs := ParseSanHosts(hosts...)
	if len(dnsNames) == 0 && len(ipAddrs) == 0 {
		dnsNames = []string{localhostName}
		ipAddrs = []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}
	}

	commonName := localhostName
	if len(dnsNames) > 0 {
		commonName = dnsNames[0]
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Bedrud Open Source"},
			CommonName:   commonName,
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              keyUsageForAlgo(algo),
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              dnsNames,
		IPAddresses:           ipAddrs,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, priv.Public(), priv)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %w", err)
	}

	certOut, err := SafeCreate(certFile, 0o644)
	if err != nil {
		return fmt.Errorf("failed to create cert file: %w", err)
	}
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		certOut.Close()
		os.Remove(certFile)
		return fmt.Errorf("failed to write certificate PEM: %w", err)
	}
	certOut.Close()

	keyOut, err := SafeCreate(keyFile, 0o600)
	if err != nil {
		os.Remove(certFile)
		return fmt.Errorf("failed to create key file: %w", err)
	}
	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		keyOut.Close()
		os.Remove(certFile)
		os.Remove(keyFile)
		return fmt.Errorf("failed to marshal private key: %w", err)
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		keyOut.Close()
		os.Remove(certFile)
		os.Remove(keyFile)
		return fmt.Errorf("failed to write key PEM: %w", err)
	}
	keyOut.Close()

	return nil
}

func generateKey(algo KeyAlgorithm) (crypto.Signer, error) {
	switch algo {
	case KeyEd25519:
		_, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("failed to generate Ed25519 key: %w", err)
		}
		return priv, nil
	case KeyECDSA256:
		priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("failed to generate ECDSA P256 key: %w", err)
		}
		return priv, nil
	case KeyRSA2048:
		priv, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return nil, fmt.Errorf("failed to generate RSA 2048 key: %w", err)
		}
		return priv, nil
	case KeyRSA4096:
		priv, err := rsa.GenerateKey(rand.Reader, 4096)
		if err != nil {
			return nil, fmt.Errorf("failed to generate RSA 4096 key: %w", err)
		}
		return priv, nil
	default:
		return nil, fmt.Errorf("unsupported key algorithm: %s", algo)
	}
}

func keyUsageForAlgo(algo KeyAlgorithm) x509.KeyUsage {
	switch algo {
	case KeyRSA2048, KeyRSA4096:
		return x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment
	default:
		return x509.KeyUsageDigitalSignature
	}
}

// DetectCertAlgorithm reads a PEM certificate and returns its public key algorithm.
func DetectCertAlgorithm(certFile string) (KeyAlgorithm, error) {
	data, err := os.ReadFile(certFile)
	if err != nil {
		return "", fmt.Errorf("cannot read cert file: %w", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return "", fmt.Errorf("no PEM data in cert file")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("cannot parse certificate: %w", err)
	}
	switch cert.PublicKeyAlgorithm {
	case x509.Ed25519:
		return KeyEd25519, nil
	case x509.ECDSA:
		return KeyECDSA256, nil
	case x509.RSA:
		return KeyRSA2048, nil
	default:
		return "", fmt.Errorf("unsupported cert key algorithm: %v", cert.PublicKeyAlgorithm)
	}
}

func ParseSanHosts(hosts ...string) (dnsNames []string, ipAddrs []net.IP) {
	for _, host := range hosts {
		if ip := net.ParseIP(host); ip != nil {
			ipAddrs = append(ipAddrs, ip)
		} else {
			dnsNames = append(dnsNames, host)
		}
	}
	return dnsNames, ipAddrs
}

type CertInfo struct {
	Subject       string    `json:"subject"`
	Issuer        string    `json:"issuer"`
	NotBefore     time.Time `json:"notBefore"`
	NotAfter      time.Time `json:"notAfter"`
	DaysRemaining int       `json:"daysRemaining"`
	SANs          []string  `json:"sans"`
	Status        string    `json:"status"`
}

func ValidateTLSCertPair(certFile, keyFile string) (*CertInfo, error) {
	if _, err := os.Stat(certFile); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("certificate file not found: %s", certFile)
		}
		return nil, fmt.Errorf("certificate file inaccessible: %s: %w", certFile, err)
	}

	if _, err := os.Stat(keyFile); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("key file not found: %s", keyFile)
		}
		return nil, fmt.Errorf("key file inaccessible: %s: %w", keyFile, err)
	}

	certPEM, err := os.ReadFile(certFile)
	if err != nil {
		return nil, fmt.Errorf("cannot read certificate file %s: %w", certFile, err)
	}

	keyPEM, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, fmt.Errorf("cannot read key file %s: %w", keyFile, err)
	}

	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		return nil, fmt.Errorf("failed to decode certificate PEM from %s: not a valid PEM file", certFile)
	}

	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, fmt.Errorf("failed to decode key PEM from %s: not a valid PEM file", keyFile)
	}

	x509Cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate from %s: %w", certFile, err)
	}

	if _, err := tls.X509KeyPair(certPEM, keyPEM); err != nil {
		return nil, fmt.Errorf("certificate/key mismatch: %w", err)
	}

	now := time.Now()
	if now.After(x509Cert.NotAfter) {
		return nil, fmt.Errorf("certificate expired on %s", x509Cert.NotAfter.Format(time.RFC3339))
	}

	if now.Before(x509Cert.NotBefore) {
		return nil, fmt.Errorf("certificate is not valid until %s", x509Cert.NotBefore.Format(time.RFC3339))
	}

	daysRemaining := int((time.Until(x509Cert.NotAfter).Hours() + 23) / 24)

	var sans []string
	sans = append(sans, x509Cert.DNSNames...)
	for _, ip := range x509Cert.IPAddresses {
		sans = append(sans, ip.String())
	}

	status := certStatusValid
	if daysRemaining <= CertWarnDays {
		status = "expiring"
	}

	return &CertInfo{
		Subject:       x509Cert.Subject.String(),
		Issuer:        x509Cert.Issuer.String(),
		NotBefore:     x509Cert.NotBefore,
		NotAfter:      x509Cert.NotAfter,
		DaysRemaining: daysRemaining,
		SANs:          sans,
		Status:        status,
	}, nil
}
