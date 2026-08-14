package jobs

// A Registry is what a package registers its jobs with. Both deployments hold
// one: the worker runs what is in it, and the server lists and starts what is
// in it, so a job is declared once and both ends of it agree.
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
