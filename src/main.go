package main

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/component-base/logs"
	"k8s.io/component-base/metrics/legacyregistry"

	"sigs.k8s.io/custom-metrics-apiserver/pkg/apiserver/metrics"
	"sigs.k8s.io/custom-metrics-apiserver/pkg/cmd"
)

const namespacePath string = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

func NewTemporalCloudProvider(temporalCloudAdapter ITemporalCloudAdapter, configPath string, namespacePath string) (*TemporalCloudProvider, error) {
	logs.AddFlags(temporalCloudAdapter.Flags())

	err := temporalCloudAdapter.Flags().Parse(os.Args)
	if err != nil {
		return nil, fmt.Errorf("unable to parse flags: %v", err)
	}

	config, err := LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration file: %v", err)
	}

	tlsConfig, err := LoadTLSCerts(&config.TemporalCloud.TLS)
	if err != nil {
		return nil, fmt.Errorf("failed to load mTLS keys: %v", err)
	}

	namespace, err := LoadNamespace(namespacePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load namespace: %v", err)
	}

	temporalCloudProvider := &TemporalCloudProvider{
		Config:    config,
		TLSConfig: tlsConfig,
		Namespace: namespace,
	}

	return temporalCloudProvider, nil
}

type ITemporalCloudAdapter interface {
	Flags() *pflag.FlagSet
	RESTMapper() (meta.RESTMapper, error)
}

type TemporalCloudAdapter struct {
	cmd.AdapterBase
	ConfigPath string
}

func main() {
	logs.InitLogs()
	defer logs.FlushLogs()

	temporalCloudAdapter := &TemporalCloudAdapter{}

	temporalCloudAdapter.Flags().StringVar(&temporalCloudAdapter.ConfigPath, "config-path", "/app/tcma/config.yaml", "config file path")
	temporalCloudAdapter.Flags().StringVar(&temporalCloudAdapter.Name, "name", "temporal-cloud-adapter", "name of the adapter")

	temporalCloudProvider, err := NewTemporalCloudProvider(temporalCloudAdapter, temporalCloudAdapter.ConfigPath, namespacePath)

	if err != nil {
		log.Fatalf("Failed to build startup prerequisites: %v", err)
	}

	temporalCloudAdapter.WithExternalMetrics(temporalCloudProvider)

	if err := metrics.RegisterMetrics(legacyregistry.Register); err != nil {
		log.Fatalf("Failed to register metrics: %v", err)
	}

	log.Println("Starting adapter...")

	if err := temporalCloudAdapter.Run(wait.NeverStop); err != nil {
		log.Fatalf("Failed to run Temporal Cloud adapter: %v", err)
	}
}
