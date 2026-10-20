/*
 * +-------------------------------------------------------------------+
 * | (C) Copyright IBM Corp. 2025, 2026                                |
 * | SPDX-License-Identifier: Apache-2.0                               |
 * +-------------------------------------------------------------------+
 */

package healthmock

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
	"os"
	"path/filepath"
	"time"

	"google.golang.org/grpc/credentials"
)

const (
	// ServerName is the TLS server name the driver's health client verifies.
	ServerName = "spyre-components"

	// File names match what the driver's health client reads from
	// SPYRE_HEALTH_TLS_{CERT,KEY,CA}_PATH.
	CertFileName = "tls.crt"
	KeyFileName  = "tls.key"
	CAFileName   = "ca.crt"
)

// GenerateCertificates writes a short-lived self-signed certificate into dir.
// The same certificate is used by the server, by the client, and as the CA
// (ca.crt), which is enough for mutual TLS between the mock and the driver.
func GenerateCertificates(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate key: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 62))
	if err != nil {
		return fmt.Errorf("generate serial: %w", err)
	}
	template := x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{Organization: []string{"spyre-health-mock"}, CommonName: ServerName},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		DNSNames:              []string{ServerName},
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return fmt.Errorf("create certificate: %w", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return fmt.Errorf("marshal key: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	files := []struct {
		name string
		data []byte
		mode os.FileMode
	}{
		{CertFileName, certPEM, 0o644},
		{CAFileName, certPEM, 0o644},
		{KeyFileName, keyPEM, 0o600},
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f.name), f.data, f.mode); err != nil {
			return fmt.Errorf("write %s: %w", f.name, err)
		}
	}
	return nil
}

// ServerCredentials loads the mTLS server credentials from the certificates in dir.
// Client certificates are required and verified against ca.crt.
func ServerCredentials(dir string) (credentials.TransportCredentials, error) {
	cert, err := tls.LoadX509KeyPair(filepath.Join(dir, CertFileName), filepath.Join(dir, KeyFileName))
	if err != nil {
		return nil, fmt.Errorf("load server certificate: %w", err)
	}
	caPEM, err := os.ReadFile(filepath.Join(dir, CAFileName))
	if err != nil {
		return nil, fmt.Errorf("read CA certificate: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("parse CA certificate")
	}
	return credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    pool,
		MinVersion:   tls.VersionTLS12,
	}), nil
}
