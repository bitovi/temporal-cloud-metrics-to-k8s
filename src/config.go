package main

import (
	"crypto/tls"
	"crypto/x509"
	"os"

	"gopkg.in/yaml.v2"
)

type TLSConfig struct {
	CA   string `yaml:"ca"`
	Cert string `yaml:"cert"`
	Key  string `yaml:"key"`
}

type TemporalCloudConfig struct {
	MetricsEndpoint string    `yaml:"metrics_endpoint"`
	TLS             TLSConfig `yaml:"tls"`
}

type Query struct {
	Query string `yaml:"query"`
}

type Config struct {
	Version       string              `yaml:"version"`
	TemporalCloud TemporalCloudConfig `yaml:"temporal_cloud"`
	Metrics       map[string]Query    `yaml:"metrics"`
}

func LoadConfig(configPath string) (*Config, error) {
	configYaml, err := os.ReadFile(configPath)
	if err != nil {
		return &Config{}, err
	}

	var config Config
	err = yaml.Unmarshal(configYaml, &config)
	if err != nil {
		return &Config{}, err
	}

	return &config, nil
}

func LoadTLSCerts(tlsConfig *TLSConfig) (*tls.Config, error) {
	var certPool *x509.CertPool

	if tlsConfig.CA != "" {
		caCert, err := os.ReadFile(tlsConfig.CA)
		if err != nil {
			return &tls.Config{}, err
		}
		certPool = x509.NewCertPool()
		certPool.AppendCertsFromPEM(caCert)
	}

	certificate, err := tls.LoadX509KeyPair(tlsConfig.Cert, tlsConfig.Key)
	if err != nil {
		return &tls.Config{}, err
	}
	return &tls.Config{
		RootCAs:      certPool,
		Certificates: []tls.Certificate{certificate},
	}, nil
}

func LoadNamespace(namespacePath string) (string, error) {
	namespaceBytes, err := os.ReadFile(namespacePath)
	if err != nil {
		return "", err
	}

	namespace := string(namespaceBytes)

	return namespace, nil
}
