package cron

import "context"

// Job represents a scheduled task that runs inside the cron worker.
type Job interface {
	Name() string
	Run(ctx context.Context) error
}

// Registry tracks registered cron jobs.
type Registry struct {
	jobs []Job
}

// NewRegistry builds a registry preloaded with the provided jobs.
func NewRegistry(jobs ...Job) *Registry {
	registry := &Registry{}
	for _, job := range jobs {
		if job == nil {
			continue
		}
		registry.jobs = append(registry.jobs, job)
	}
	return registry
}

// Register adds a job to the registry.
func (r *Registry) Register(job Job) {
	if job == nil {
		return
	}
	r.jobs = append(r.jobs, job)
}

// Jobs returns the registered jobs in the order they were added.
func (r *Registry) Jobs() []Job {
	jobs := make([]Job, len(r.jobs))
	copy(jobs, r.jobs)
	return jobs
}
