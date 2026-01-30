package cron

import (
	"context"
	"testing"
)

type stubJob struct {
	name string
}

func (s *stubJob) Name() string              { return s.name }
func (s *stubJob) Run(context.Context) error { return nil }

func TestRegistryStoresJobs(t *testing.T) {
	registry := NewRegistry()
	jobA := &stubJob{name: "a"}
	jobB := &stubJob{name: "b"}
	registry.Register(jobA)
	registry.Register(jobB)
	jobs := registry.Jobs()
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}
	if jobs[0] != jobA || jobs[1] != jobB {
		t.Fatalf("jobs returned out of order")
	}
	// ensure caller cannot mutate internal slice
	jobs[0] = nil
	if registry.Jobs()[0] == nil {
		t.Fatalf("internal slice leaked")
	}
}
