package utils

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"time"
)

const CertWarnDays = 30

// GenerateSelfSignedCert creates a self-signed certificate and key for development.
// hosts may include IP addresses and DNS names; they are added as SANs.
func GenerateSelfSignedCert(certFile, keyFile string, hosts ...string) error {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * time.Hour)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return err
	}

	dnsNames, ipAddrs := ParseSanHosts(hosts...)
	if len(dnsNames) == 0 && len(ipAddrs) == 0 {
		dnsNames = []string{"localhost"}
	}

	commonName := "localhost"
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
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              dnsNames,
		IPAddresses:           ipAddrs,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return err
	}

	certOut, err := os.Create(certFile)
	if err != nil {
		return err
	}
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		certOut.Close()
		return err
	}
	certOut.Close()

	keyOut, err := os.OpenFile(keyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	privBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		keyOut.Close()
		return err
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes}); err != nil {
		keyOut.Close()
		return err
	}
	keyOut.Close()

	return nil
}

// ParseSanHosts splits a list of hostnames into DNS names and IP addresses
// suitable for use as Subject Alternative Names on a certificate.
func ParseSanHosts(hosts ...string) (dnsNames []string, ipAddrs []net.IP) {
	for _, host := range hosts {
		if ip := net.ParseIP(host); ip != nil {
			ipAddrs = append(ipAddrs, ip)
		} else {
			dnsNames = append(dnsNames, host)
		}
	}
	return
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

	daysRemaining := int(time.Until(x509Cert.NotAfter).Hours() / 24)

	var sans []string
	sans = append(sans, x509Cert.DNSNames...)
	for _, ip := range x509Cert.IPAddresses {
		sans = append(sans, ip.String())
	}

	status := "valid"
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
