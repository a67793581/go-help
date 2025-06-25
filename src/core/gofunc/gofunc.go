package gofunc

import (
	"context"
	"gitlab.com/aiku-open-source/go-help/src/core/hotfix"
	"time"
)

func Coroutine(ctx context.Context, f func()) {
	go func() {
		defer hotfix.RecoverError()

		f()
	}()
}

func CoroutineWithTimeOut(ctx context.Context, timeout time.Duration, f func(timeoutCtx context.Context)) {
	// 使用context.WithTimeout设置上下文的超时
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)

	go func() {
		defer func() {
			cancel() // 确保协程完成后取消上下文
			hotfix.RecoverError()
		}()

		f(timeoutCtx)
	}()
}
