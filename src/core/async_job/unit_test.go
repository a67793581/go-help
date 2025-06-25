package async_job

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"
)

func init() {

}

func TestDo(t *testing.T) {
	var err error
	rand.Seed(time.Now().Unix())
	if rand.Int()%2 == 0 {
		err = fmt.Errorf("失败")
	}
	fmt.Println("当前请求错误", err)
	ctx := context.Background()
	i1 := 1
	for i := 0; i < 10; i++ {
		func(iii int) {
			Push(ctx, func(ctx context.Context, req interface{}, resp interface{}, err1 error) {
				if err1 != nil {
					return
				}
				fmt.Printf("执行第%d个错误时不执行的函数\n", iii+1)
			})
		}(i)
	}
	Run(ctx, i1, i1, err)
	ctx2 := context.Background()
	i2 := 1
	for i := 0; i < 10; i++ {
		func(iii int) {
			Push(ctx, func(ctx context.Context, req interface{}, resp interface{}, err2 error) {
				fmt.Printf("执行第%d个错误时也执行的函数\n", iii+1)
			})
		}(i)
	}
	Run(ctx2, i2, i2, err)
	time.Sleep(1 * time.Second)
}
