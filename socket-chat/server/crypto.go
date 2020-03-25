package main

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"time"
)

type CertUsage int

const (
	CertUsageCA CertUsage = 1 << iota
	CertUsageServer
	CertUsageClient
)

func CreateServerCerts() error {
	caCert, caKey, err := genCert("ca", CertUsageCA, nil, nil, nil)
	if err != nil {
		return err
	}
	_, _, err = genCert("server", CertUsageServer, caCert, caKey, []string{"127.0.0.1", "localhost"})
	return err
}

func genCert(fileprefix string, usage CertUsage, caCert *x509.Certificate, caKey crypto.Signer, sans []string) (*x509.Certificate, crypto.Signer, error) {
	_, key, err := ed25519.GenerateKey(rand.Reader)
	//key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	keyBytes, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, nil, err
	}

	name := pkix.Name{
		CommonName:    fileprefix,
		Organization:  []string{"luxas labs Ltd."},
		Country:       []string{"FI"},
		Locality:      []string{"The Finnish West Coast"},
		StreetAddress: []string{"At the beach"},
	}

	serialNum, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 16*8)) // the maximum valid serial number is 20 bytes
	if err != nil {
		return nil, nil, err
	}
	cert := &x509.Certificate{
		SerialNumber:          serialNum,
		Subject:               name,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(1, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		BasicConstraintsValid: true,
	}

	for _, san := range sans {
		if ip := net.ParseIP(san); ip != nil {
			cert.IPAddresses = append(cert.IPAddresses, ip)
		} else {
			cert.DNSNames = append(cert.DNSNames, san)
		}
	}

	if caCert == nil {
		caCert = cert
	}
	if caKey == nil {
		caKey = key
	}

	if usage&CertUsageCA != 0 {
		cert.IsCA = true
		cert.KeyUsage |= x509.KeyUsageCertSign
	}
	if usage&CertUsageServer != 0 {
		cert.ExtKeyUsage = append(cert.ExtKeyUsage, x509.ExtKeyUsageServerAuth)
	}
	if usage&CertUsageClient != 0 {
		cert.ExtKeyUsage = append(cert.ExtKeyUsage, x509.ExtKeyUsageClientAuth)
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, cert, caCert, key.Public(), caKey)
	if err != nil {
		return nil, nil, err
	}

	files := []struct {
		pemType  string
		bytes    []byte
		filename string
	}{
		{
			pemType:  "CERTIFICATE",
			bytes:    certBytes,
			filename: fileprefix + ".crt",
		},
		{
			pemType:  "PRIVATE KEY",
			bytes:    keyBytes,
			filename: fileprefix + ".key",
		},
	}
	for _, file := range files {
		keyOut, err := os.OpenFile(file.filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			return nil, nil, fmt.Errorf("Failed to open key.pem for writing: %v", err)
		}
		if err := pem.Encode(keyOut, &pem.Block{Type: file.pemType, Bytes: file.bytes}); err != nil {
			return nil, nil, fmt.Errorf("Failed to write data to key.pem: %v", err)
		}
		if err := keyOut.Close(); err != nil {
			return nil, nil, fmt.Errorf("Error closing key.pem: %v", err)
		}
		log.Printf("Wrote %s\n", file.filename)
	}
	return cert, key, nil
}
