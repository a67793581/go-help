package token_bucket

import (
	"fmt"
	"testing"
)

func init() {

}

func TestDo(t *testing.T) {

	const tokenBucketMax = 100
	tokenBucket := NewTokenBucket(tokenBucketMax)
	go tokenBucket.TickerPush(1, 10)
	for i := 0; i < 120; i++ {
		tokenBucket.Pop(1)
		fmt.Println(i + 1)
	}
	tokenBucket.Close()

}
