// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gcm

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"sort"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	cloudmonitoring "google.golang.org/api/monitoring/v3"
)

const (
	customMetricPrefix = "custom.googleapis.com"
)

type labelData struct {
	key         string
	description string
}

var aggLabelData = []labelData{
	labelData{
		key:         "aggregation",
		description: "The aggregation type (min, max, avg, sum, count)",
	},
}

// customMetricDescriptors is a map from metric's short names to their
// MetricDescriptor definitions.
var customMetricDescriptors = map[string]*cloudmonitoring.MetricDescriptor{
	// Custom metrics for recording stats of cloud syncbase instances.
	"cloud-syncbase": createMetric("cloud-syncbase", "Stats of cloud syncbase instances.", "double", false, []labelData{
		labelData{
			key:         "mounted_name",
			description: "The relative mounted name of the instance",
		},
	}),
	"cloud-syncbase-agg": createMetric("cloud-syncbase-agg", "The aggregated stats of cloud syncbase instances.", "double", false, aggLabelData),

	// Custom metrics for recording check latency and its aggregation
	// of vanadium production services.
	"service-latency":     createMetric("service/latency", "The check latency (ms) of vanadium production services.", "double", true, nil),
	"service-latency-agg": createMetric("service/latency-agg", "The aggregated check latency (ms) of vanadium production services.", "double", false, aggLabelData),

	// Custom metric for recording per-method rpc latency and its aggregation
	// for a service.
	"service-permethod-latency": createMetric("service/latency/method", "Service latency (ms) per method.", "double", true, []labelData{
		labelData{
			key:         "method_name",
			description: "The method name",
		},
	}),
	"service-permethod-latency-agg": createMetric("service/latency/method-agg", "Aggregated service latency (ms) per method.", "double", false, []labelData{
		labelData{
			key:         "method_name",
			description: "The method name",
		},
		aggLabelData[0],
	}),

	// Custom metric for recording various counters and their aggregations
	// of vanadium production services.
	"service-counters":     createMetric("service/counters", "Various counters of vanadium production services.", "double", true, nil),
	"service-counters-agg": createMetric("service/counters-agg", "Aggregated counters of vanadium production services.", "double", false, aggLabelData),

	// Custom metric for recording service metadata and its aggregation
	// of vanadium production services.
	"service-metadata": createMetric("service/metadata", "Various metadata of vanadium production services.", "double", true, []labelData{
		labelData{
			key:         "metadata_name",
			description: "The metadata name",
		},
	}),
	"service-metadata-agg": createMetric("service/metadata-agg", "Aggregated metadata of vanadium production services.", "double", false, []labelData{
		labelData{
			key:         "metadata_name",
			description: "The metadata name",
		},
		aggLabelData[0],
	}),

	// Custom metric for recording total rpc qps and its aggregation for a service.
	"service-qps-total":     createMetric("service/qps/total", "Total service QPS.", "double", true, nil),
	"service-qps-total-agg": createMetric("service/qps/total-agg", "Aggregated total service QPS.", "double", false, aggLabelData),

	// Custom metric for recording per-method rpc qps for a service.
	"service-qps-method": createMetric("service/qps/method", "Service QPS per method.", "double", true, []labelData{
		labelData{
			key:         "method_name",
			description: "The method name",
		},
	}),
	"service-qps-method-agg": createMetric("service/qps/method-agg", "Aggregated service QPS per method.", "double", false, []labelData{
		labelData{
			key:         "method_name",
			description: "The method name",
		},
		aggLabelData[0],
	}),

	// Custom metric for recording gce instance stats.
	"gce-instance": createMetric("gce-instance/stats", "Various stats for GCE instances.", "double", true, nil),

	// Custom metric for recording nginx stats.
	"nginx": createMetric("nginx/stats", "Various stats for Nginx server.", "double", true, nil),

	// Custom metric for rpc load tests.
	"rpc-load-test": createMetric("rpc-load-test", "Results of rpc load test.", "double", false, nil),

	// Custom metric for recording jenkins related data.
	"jenkins": createMetric("jenkins", "Jenkins related data.", "double", false, nil),
}

func createMetric(metricType, description, valueType string, includeGCELabels bool, extraLabels []labelData) *cloudmonitoring.MetricDescriptor {
	labels := []*cloudmonitoring.LabelDescriptor{}
	if includeGCELabels {
		labels = append(labels, &cloudmonitoring.LabelDescriptor{
			Key:         "gce_instance",
			Description: "The name of the GCE instance associated with this metric.",
			ValueType:   "string",
		}, &cloudmonitoring.LabelDescriptor{
			Key:         "gce_zone",
			Description: "The zone of the GCE instance associated with this metric.",
			ValueType:   "string",
		})
	}
	labels = append(labels, &cloudmonitoring.LabelDescriptor{
		Key:         "metric_name",
		Description: "The name of the metric.",
		ValueType:   "string",
	})
	if extraLabels != nil {
		for _, data := range extraLabels {
			labels = append(labels, &cloudmonitoring.LabelDescriptor{
				Key:         fmt.Sprintf("%s", data.key),
				Description: data.description,
				ValueType:   "string",
			})
		}
	}

	return &cloudmonitoring.MetricDescriptor{
		Type:        fmt.Sprintf("%s/vanadium/%s", customMetricPrefix, metricType),
		Description: description,
		MetricKind:  "gauge",
		ValueType:   valueType,
		Labels:      labels,
	}
}

// GetMetric gets the custom metric descriptor with the given name and project.
func GetMetric(name, project string) (*cloudmonitoring.MetricDescriptor, error) {
	md, ok := customMetricDescriptors[name]
	if !ok {
		return nil, fmt.Errorf("metric %q doesn't exist", name)
	}
	md.Name = fmt.Sprintf("projects/%s/metricDescriptors/%s", project, md.Type)
	return md, nil
}

// GetSortedMetricNames gets the sorted metric names.
func GetSortedMetricNames() []string {
	names := []string{}
	for n := range customMetricDescriptors {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func createClient(keyFilePath string) (*http.Client, error) {
	if len(keyFilePath) > 0 {
		data, err := ioutil.ReadFile(keyFilePath)
		if err != nil {
			return nil, err
		}
		conf, err := google.JWTConfigFromJSON(data, cloudmonitoring.MonitoringScope)
		if err != nil {
			return nil, fmt.Errorf("failed to create JWT config file: %v", err)
		}
		return conf.Client(oauth2.NoContext), nil
	}

	return google.DefaultClient(oauth2.NoContext, cloudmonitoring.MonitoringScope)
}

// Authenticate authenticates with the given JSON credentials file (or the
// default client if the file is not provided). If successful, it returns a
// service object that can be used in GCM API calls.
func Authenticate(keyFilePath string) (*cloudmonitoring.Service, error) {
	c, err := createClient(keyFilePath)
	if err != nil {
		return nil, err
	}
	s, err := cloudmonitoring.New(c)
	if err != nil {
		return nil, fmt.Errorf("New() failed: %v", err)
	}
	return s, nil
}
