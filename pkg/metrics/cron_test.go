package metrics

import (
	"fmt"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestCronJobMetricsExportsCountersAndHistogram(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := NewCronJobMetrics(reg)
	job := "test-job"
	metrics.ObserveDuration(job, 250*time.Millisecond)
	metrics.IncSuccess(job)
	metrics.IncFailure(job)

	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}

	if got, err := fetchCounterValue(mfs, "job_success", "job", job); err != nil {
		t.Fatalf("fetch success: %v", err)
	} else if got != 1 {
		t.Fatalf("expected success=1, got %f", got)
	}

	if got, err := fetchCounterValue(mfs, "job_failure", "job", job); err != nil {
		t.Fatalf("fetch failure: %v", err)
	} else if got != 1 {
		t.Fatalf("expected failure=1, got %f", got)
	}

	if got, err := fetchHistogramSum(mfs, "job_duration_seconds", "job", job); err != nil {
		t.Fatalf("fetch duration: %v", err)
	} else if got <= 0 {
		t.Fatalf("expected duration sum > 0, got %f", got)
	}
}

func fetchCounterValue(mfs []*dto.MetricFamily, name, label, value string) (float64, error) {
	mf := findMetricFamily(mfs, name)
	if mf == nil {
		return 0, fmt.Errorf("metric %q not found", name)
	}
	for _, metric := range mf.GetMetric() {
		if matchesLabel(metric.GetLabel(), label, value) {
			return metric.GetCounter().GetValue(), nil
		}
	}
	return 0, fmt.Errorf("metric %q missing label %s=%s", name, label, value)
}

func fetchHistogramSum(mfs []*dto.MetricFamily, name, label, value string) (float64, error) {
	mf := findMetricFamily(mfs, name)
	if mf == nil {
		return 0, fmt.Errorf("metric %q not found", name)
	}
	for _, metric := range mf.GetMetric() {
		if matchesLabel(metric.GetLabel(), label, value) {
			return metric.GetHistogram().GetSampleSum(), nil
		}
	}
	return 0, fmt.Errorf("histogram %q missing label %s=%s", name, label, value)
}

func findMetricFamily(mfs []*dto.MetricFamily, name string) *dto.MetricFamily {
	for _, mf := range mfs {
		if mf.GetName() == name {
			return mf
		}
	}
	return nil
}

func matchesLabel(labels []*dto.LabelPair, name, value string) bool {
	for _, label := range labels {
		if label.GetName() == name && label.GetValue() == value {
			return true
		}
	}
	return false
}
