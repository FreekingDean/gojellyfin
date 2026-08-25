package jobs

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

func (r *Registry) Find(name string) (Job, error) {
	for _, job := range r.jobs {
		if job.Name() == name {
			return job, nil
		}
	}

	return nil, ErrNotFound
}
