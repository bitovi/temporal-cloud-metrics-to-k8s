package main

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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var metricsEndpoint = "https://example.com/prometheus"
var mockNamespace = "testing.xyz"

func generateSelfSignedCert() (*os.File, *os.File, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(24 * time.Hour)

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Test Org"},
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, nil, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	privBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return nil, nil, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes})

	certFile, err := os.CreateTemp("", "cert.pem")
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to create temp cert file: %v", err)
	}

	keyFile, err := os.CreateTemp("", "key.pem")
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to create temp key file: %v", err)
	}

	if _, err := certFile.Write(certPEM); err != nil {
		return nil, nil, fmt.Errorf("Failed to write to temp cert file: %v", err)
	}
	if _, err := keyFile.Write(keyPEM); err != nil {
		return nil, nil, fmt.Errorf("Failed to write to temp key file: %v", err)
	}

	return certFile, keyFile, nil
}

func generateConfigFile(certPath string, keyPath string) (*os.File, error) {
	config := fmt.Sprintf(`
    version: "1"
    temporal_cloud:
      metrics_endpoint: %s
      tls:
        cert: %s
        key:  %s
    metrics:
      temporal_cloud_sync_match_rate:
        query: >
          foo - bar
      temporal_cloud_service_latency:
        query: >
          foo * baz
  `, metricsEndpoint, certPath, keyPath)
	configFile, err := os.CreateTemp("", "config.yaml")
	if err != nil {
		return nil, fmt.Errorf("Failed to create temp config file: %v", err)
	}

	if _, err := configFile.Write([]byte(config)); err != nil {
		return nil, fmt.Errorf("Failed to write to temp config file: %v", err)
	}

	return configFile, nil
}

func generateNamespaceFile() (*os.File, error) {
	namespaceFile, err := os.CreateTemp("", "key.pem")
	if err != nil {
		return nil, fmt.Errorf("Failed to create temp key file: %v", err)
	}

	if _, err := namespaceFile.Write([]byte(mockNamespace)); err != nil {
		return nil, fmt.Errorf("Failed to write to temp key file: %v", err)
	}

	return namespaceFile, nil
}

func TestTemporalCloudProvider(t *testing.T) {
	certFile, keyFile, err := generateSelfSignedCert()
	assert.Equal(t, nil, err)
	defer os.Remove(certFile.Name())
	defer os.Remove(keyFile.Name())

	configFile, err := generateConfigFile(certFile.Name(), keyFile.Name())
	assert.Equal(t, nil, err)
	defer os.Remove(configFile.Name())

	namespace, err := generateNamespaceFile()
	assert.Equal(t, nil, err)

	temporalCloudAdapter := &TemporalCloudAdapter{}
	temporalCloudProvider, err := NewTemporalCloudProvider(temporalCloudAdapter, configFile.Name(), namespace.Name())
	assert.Equal(t, nil, err)

	certificate, err := tls.LoadX509KeyPair(certFile.Name(), keyFile.Name())
	assert.Equal(t, nil, err)
	assert.Equal(t, &TemporalCloudProvider{
		Config: &Config{
			Version: "1",
			TemporalCloud: TemporalCloudConfig{
				MetricsEndpoint: metricsEndpoint,
				TLS: TLSConfig{
					Cert: certFile.Name(),
					Key:  keyFile.Name(),
				},
			},
			Metrics: map[string]Query{
				"temporal_cloud_sync_match_rate": {
					Query: "foo - bar\n",
				},
				"temporal_cloud_service_latency": {
					Query: "foo * baz\n",
				},
			},
		},
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{certificate},
		},
		Namespace: mockNamespace,
	}, temporalCloudProvider)
}
