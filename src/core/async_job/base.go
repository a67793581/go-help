package async_job

import (
	"context"
	"gitlab.com/aiku-open-source/go-help/src/core/logger"
	"runtime/debug"
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

func DelInstance(ctx context.Context) {
	instanceSM.Delete(ctx)
}

func GetInstance(ctx context.Context) (result *jobList) {
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
	result := GetInstance(ctx)
	result.Lock()
	result.jobs = append(result.jobs, f)
	result.Unlock()
}

func RecoverError() {
	if err := recover(); err != nil {
		if logger.Log != nil {
			logger.Log.Errorf("err:%+v\nStack:%s", err, string(debug.Stack()))
		}
	}
}

func do(ctx context.Context, req interface{}, resp interface{}, err error) {

	defer RecoverError()
	defer DelInstance(ctx)
	result := GetInstance(ctx)
	for _, job := range result.jobs {
		job(ctx, req, resp, err)
	}
	return
}
