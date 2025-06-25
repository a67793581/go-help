package token_bucket

import (
	"time"
)

type TokenBucket struct {
	c   chan struct{}
	max int
}

func NewTokenBucket(max int) *TokenBucket {
	result := new(TokenBucket)
	result.c = make(chan struct{}, max)
	result.max = max
	return result
}

func (t *TokenBucket) TickerPush(intervalSecond, num int) {
	t.Push(num)
	ticker := time.NewTicker(time.Second * time.Duration(intervalSecond))
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if len(t.c) <= t.max-num {
				t.Push(num)
			}
		}
	}
}
func (t *TokenBucket) Push(num int) {
	for i := 0; i < num; i++ {
		t.c <- struct{}{}
	}
}
func (t *TokenBucket) Pop(num int) {
	for i := 0; i < num; i++ {
		<-t.c
	}
}
func (t *TokenBucket) Close() {
	close(t.c)
}
