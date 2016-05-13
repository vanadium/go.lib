// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gcm

import (
	"fmt"
	"reflect"
	"testing"

	cloudmonitoring "google.golang.org/api/monitoring/v3"
)

func TestCreateMetric(t *testing.T) {
	type testCase struct {
		metricType       string
		description      string
		valueType        string
		includeGCELabels bool
		extraLabels      []labelData
		expectedMetric   *cloudmonitoring.MetricDescriptor
	}
	testCases := []testCase{
		testCase{
			metricType:       "test",
			description:      "this is a test",
			valueType:        "double",
			includeGCELabels: false,
			extraLabels:      nil,
			expectedMetric: &cloudmonitoring.MetricDescriptor{
				Type:        fmt.Sprintf("%s/vanadium/test", customMetricPrefix),
				Description: "this is a test",
				MetricKind:  "gauge",
				ValueType:   "double",
				Labels: []*cloudmonitoring.LabelDescriptor{
					&cloudmonitoring.LabelDescriptor{
						Key:         "metric_name",
						Description: "The name of the metric.",
						ValueType:   "string",
					},
				},
			},
		},
		testCase{
			metricType:       "test2",
			description:      "this is a test2",
			valueType:        "string",
			includeGCELabels: true,
			extraLabels:      nil,
			expectedMetric: &cloudmonitoring.MetricDescriptor{
				Type:        fmt.Sprintf("%s/vanadium/test2", customMetricPrefix),
				Description: "this is a test2",
				MetricKind:  "gauge",
				ValueType:   "string",
				Labels: []*cloudmonitoring.LabelDescriptor{
					&cloudmonitoring.LabelDescriptor{
						Key:         "gce_instance",
						Description: "The name of the GCE instance associated with this metric.",
						ValueType:   "string",
					},
					&cloudmonitoring.LabelDescriptor{
						Key:         "gce_zone",
						Description: "The zone of the GCE instance associated with this metric.",
						ValueType:   "string",
					},
					&cloudmonitoring.LabelDescriptor{
						Key:         "metric_name",
						Description: "The name of the metric.",
						ValueType:   "string",
					},
				},
			},
		},
		testCase{
			metricType:       "test3",
			description:      "this is a test3",
			valueType:        "double",
			includeGCELabels: true,
			extraLabels: []labelData{
				labelData{
					key:         "extraLabel",
					description: "this is an extra label",
				},
			},
			expectedMetric: &cloudmonitoring.MetricDescriptor{
				Type:        fmt.Sprintf("%s/vanadium/test3", customMetricPrefix),
				Description: "this is a test3",
				MetricKind:  "gauge",
				ValueType:   "double",
				Labels: []*cloudmonitoring.LabelDescriptor{
					&cloudmonitoring.LabelDescriptor{
						Key:         "gce_instance",
						Description: "The name of the GCE instance associated with this metric.",
						ValueType:   "string",
					},
					&cloudmonitoring.LabelDescriptor{
						Key:         "gce_zone",
						Description: "The zone of the GCE instance associated with this metric.",
						ValueType:   "string",
					},
					&cloudmonitoring.LabelDescriptor{
						Key:         "metric_name",
						Description: "The name of the metric.",
						ValueType:   "string",
					},
					&cloudmonitoring.LabelDescriptor{
						Key:         "extraLabel",
						Description: "this is an extra label",
						ValueType:   "string",
					},
				},
			},
		},
	}
	for _, test := range testCases {
		got := createMetric(test.metricType, test.description, test.valueType, test.includeGCELabels, test.extraLabels)
		if !reflect.DeepEqual(got, test.expectedMetric) {
			t.Fatalf("want %#v, got %#v", test.expectedMetric, got)
		}
	}
}
