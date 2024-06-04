package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/metrics/pkg/apis/external_metrics"
	"sigs.k8s.io/custom-metrics-apiserver/pkg/provider"
)

var temporalNamespace = "testing.123xyz"
var metricsQuery = "one_metric - other_metric"
var timestamp = 1717611392.57

func TestTemporalCloudMetricsTransform(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		expectedUrl := fmt.Sprintf("/api/v1/query?query=%s", url.QueryEscape(metricsQuery))
		assert.Equal(t, expectedUrl, req.URL.String())

		response := fmt.Sprintf(`
			{
				"status": "success",
				"data": {
					"resultType": "vector",
					"result": [
						{
							"metric": {
								"temporal_namespace": "%s"
							},
							"value": [
								%f,
								"0.05"
							]
						}
					]
				}
			}
		`, temporalNamespace, timestamp)
		rw.Write([]byte(response))
	}))
	defer server.Close()

	temporalCloudProvider := &TemporalCloudProvider{
		Config: &Config{
			Version: "1",
			TemporalCloud: TemporalCloudConfig{
				MetricsEndpoint: server.URL,
			},
			Metrics: map[string]Query{
				"test_metric": {
					Query: metricsQuery,
				},
			},
		},
		TLSConfig: &tls.Config{},
		Namespace: temporalNamespace,
	}

	allMetrics := temporalCloudProvider.ListAllExternalMetrics()
	assert.Equal(t, allMetrics, []provider.ExternalMetricInfo{
		{
			Metric: "test_metric",
		},
	})

	testMetric, err := temporalCloudProvider.GetExternalMetric(context.Background(), temporalNamespace, labels.NewSelector(), provider.ExternalMetricInfo{Metric: "test_metric"})
	assert.Equal(t, nil, err)

	value, err := resource.ParseQuantity("0.05")
	assert.Equal(t, nil, err)

	assert.Equal(t, testMetric, &external_metrics.ExternalMetricValueList{
		Items: []external_metrics.ExternalMetricValue{
			{
				MetricName: "test_metric",
				Value:      value,
				Timestamp:  *prometheusTimestampToK8s(timestamp),
				MetricLabels: labels.Set(map[string]string{
					"temporal_namespace": temporalNamespace,
				}),
			},
		},
	})

}
