package gofunc

import (
	"context"
	"gitlab.com/aiku-open-source/go-help/src/core/logger"
	"time"
)

func Coroutine(ctx context.Context, f func()) {
	go func() {
		defer func() {
			if err := recover(); err != nil {
				logger.Log.Error(ctx, "GoFunc err:", err)
			}
		}()

		f()
	}()
}

func CoroutineWithTimeOut(ctx context.Context, timeout time.Duration, f func(timeoutCtx context.Context)) {
	// 使用context.WithTimeout设置上下文的超时
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)

	go func() {
		defer func() {
			cancel() // 确保协程完成后取消上下文
			if err := recover(); err != nil {
				logger.Log.Error(timeoutCtx, "GoFunc err:", err)
			}
		}()

		f(timeoutCtx)
	}()
}
