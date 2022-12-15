package async_job

import (
	"context"
	"gitlab.com/aiku-open-source/go-help/src/core/hotfix"
	"sync"
)

type (
	Job func(ctx context.Context, req interface{}, resp interface{}, err error)

	jobList struct {
		sync.Mutex
		jobs []Job
	}
)

var (
	instanceSM = sync.Map{}
)

func delInstance(ctx context.Context) {
	instanceSM.Delete(ctx)
}

func getInstance(ctx context.Context) (result *jobList) {
	var (
		ok bool
		v  interface{}
	)
	v, ok = instanceSM.Load(ctx)
	if !ok {
		result = &jobList{
			jobs: []Job{},
		}
		instanceSM.Store(ctx, result)
	} else {
		result = v.(*jobList)
	}

	return
}

func Push(ctx context.Context, f Job) {
	result := getInstance(ctx)
	result.Lock()
	result.jobs = append(result.jobs, f)
	result.Unlock()
}

func Run(ctx context.Context, req interface{}, resp interface{}, err error) {

	defer hotfix.RecoverError()
	defer delInstance(ctx)
	result := getInstance(ctx)
	for _, job := range result.jobs {
		job(ctx, req, resp, err)
	}
	return
}
