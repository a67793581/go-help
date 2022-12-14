package hotfix

import (
	"gitlab.com/aiku-open-source/go-help/src/core/logger"
	"runtime/debug"
)

func RecoverError() {
	if err := recover(); err != nil {
		if logger.Log != nil {
			logger.Log.Errorf("err:%+v\nStack:%s", err, string(debug.Stack()))
		}
	}
}
