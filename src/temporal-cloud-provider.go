package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"time"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/metrics/pkg/apis/external_metrics"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/custom-metrics-apiserver/pkg/provider"
)

type PrometheusResponse struct {
	Status string `json:"status"`
	Data   Data   `json:"data"`
}

type Data struct {
	ResultType string   `json:"resultType"`
	Result     []Result `json:"result"`
}

type Result struct {
	Metric map[string]string `json:"metric"`
	Value  Value             `json:"value"`
}

type Value [2]interface{}

type TemporalCloudProvider struct {
	Config    *Config
	TLSConfig *tls.Config
	Namespace string
}

func prometheusTimestampToK8s(timestamp float64) *metav1.Time {
	timestampSeconds := int64(timestamp)
	timestampNanoseconds := int64(math.Mod(timestamp, 1) * 1e9)
	timestampUnixFormat := time.Unix(timestampSeconds, timestampNanoseconds)

	return &metav1.Time{Time: timestampUnixFormat}
}

func prometheusValueToK8s(value Value) (resource.Quantity, *metav1.Time, error) {
	if len(value) < 2 {
		return resource.Quantity{}, nil, errors.New("invalid metric response from Temporal Cloud")
	}

	timestampDecimal, ok := value[0].(float64)
	if !ok {
		return resource.Quantity{}, nil, errors.New("invalid metric timestamp from Temporal Cloud")
	}
	timestamp := prometheusTimestampToK8s(timestampDecimal)

	metric, ok := value[1].(string)
	if !ok {
		return resource.Quantity{}, nil, errors.New("invalid metric value from Temporal Cloud")
	}

	quantity, err := resource.ParseQuantity(metric)
	if err != nil {
		return resource.Quantity{}, nil, err
	}

	return quantity, timestamp, nil
}

func prometheusQuery(metricsEndpoint string, query string, tlsConfig *tls.Config) (PrometheusResponse, error) {
	transport := &http.Transport{TLSClientConfig: tlsConfig}
	client := &http.Client{Transport: transport}
	url := fmt.Sprintf(
		"%s/api/v1/query?query=%s",
		metricsEndpoint,
		url.QueryEscape(query),
	)

	resp, err := client.Get(url)
	if err != nil {
		return PrometheusResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return PrometheusResponse{}, fmt.Errorf("unexpected status from Temporal Cloud: %d", resp.StatusCode)
	}

	var response PrometheusResponse

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return PrometheusResponse{}, err
	}

	return response, nil
}

func (p *TemporalCloudProvider) GetExternalMetric(ctx context.Context, namespace string, selector labels.Selector, info provider.ExternalMetricInfo) (*external_metrics.ExternalMetricValueList, error) {
	externalMetricsFormat := []external_metrics.ExternalMetricValue{}
	metricConfig := p.Config.Metrics[info.Metric]

	if metricConfig.Query != "" && p.Namespace == namespace {
		response, err := prometheusQuery(p.Config.TemporalCloud.MetricsEndpoint, metricConfig.Query, p.TLSConfig)
		if err != nil {
			return nil, err
		}
		log.Printf("Temporal Cloud Metrics: %+v\n", response)

		for _, result := range response.Data.Result {
			labels := labels.Set(result.Metric)
			if !selector.Matches(labels) {
				continue
			}

			value, timestamp, err := prometheusValueToK8s(result.Value)
			if err != nil {
				return nil, err
			}

			externalMetricValue := external_metrics.ExternalMetricValue{
				MetricName:   info.Metric,
				Value:        value,
				Timestamp:    *timestamp,
				MetricLabels: labels,
			}

			externalMetricsFormat = append(externalMetricsFormat, externalMetricValue)
		}
	}

	log.Printf("Kubernetes Mapped Metrics: %+v\n", externalMetricsFormat)

	return &external_metrics.ExternalMetricValueList{
		Items: externalMetricsFormat,
	}, nil
}

func (p *TemporalCloudProvider) ListAllExternalMetrics() []provider.ExternalMetricInfo {
	var metricsInfoList []provider.ExternalMetricInfo

	for name := range p.Config.Metrics {
		metricsInfoList = append(metricsInfoList, provider.ExternalMetricInfo{
			Metric: name,
		})
	}

	return metricsInfoList
}
