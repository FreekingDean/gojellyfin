package jobs

import "slices"

type Registry struct {
	jobs []Job
}

func NewRegistry() *Registry {
	return &Registry{}
}

func (r *Registry) Register(job Job) {
	r.jobs = append(r.jobs, job)
}

func (r *Registry) All() []Job {
	return r.jobs
}

func (r *Registry) Startable() []Job {
	found := make([]Job, 0, len(r.jobs))
	for _, job := range r.jobs {
		if job.Startable {
			found = append(found, job)
		}
	}

	return found
}

func (r *Registry) Queues() []string {
	queues := []string{TaskQueue}
	for _, job := range r.jobs {
		if !slices.Contains(queues, job.queue()) {
			queues = append(queues, job.queue())
		}
	}

	return queues
}

func (r *Registry) Find(name string) (Job, error) {
	for _, job := range r.jobs {
		if job.Name == name {
			return job, nil
		}
	}

	return Job{}, ErrNotFound
}
