package activations

import (
	"fmt"
	"testing"
)

func TestVerifyCode(t *testing.T) {
	type args struct {
		code string
	}
	type test struct {
		name string
		args args
		want bool
	}
	var tests []test
	var secret = "carlo"
	var total = 100
	var service = NewActivationV1(3, total, "jSYNv1rsihTxmU63wI5Mtb7JuKAOf8qoazL2FHXCd9GkZeD4RcEpy0lgBVQnPW", secret)
	var service2 = NewActivationV1(3, total*2, "jSYNv1rsihTxmU63wI5Mtb7JuKAOf8qoazL2FHXCd9GkZeD4RcEpy0lgBVQnPW", secret)
	for i := 0; i < total*2; i++ {
		if i < total {
			code, err := service.GenerateActivationCode(i)
			if err != nil {
				t.Errorf("GenCode() error = %v", err)
			}
			tests = append(tests, test{
				name: fmt.Sprintf("%v_%v", secret, i),
				args: args{
					code: code,
				},
				want: true,
			})
		} else {
			code, err := service2.GenerateActivationCode(i)
			if err != nil {
				t.Errorf("GenCode() error = %v", err)
			}
			tests = append(tests, test{
				name: fmt.Sprintf("%v_%v", secret, i),
				args: args{
					code: code,
				},
				want: false,
			})
		}
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fmt.Println(tt.args.code)
			if got := do(service, tt.args.code); got != tt.want {
				t.Errorf("VerifyCode() = %v, want %v", got, tt.want)
				t.FailNow() // 失败时立即终止当前测试函数
			}
		})
		// 检查是否有失败的测试，如果有则终止
		if t.Failed() {
			t.Errorf("测试失败，提前终止")
			break
		}
	}
}

func do(s ActivationInterface, code string) bool {
	return s.VerifyActivationCode(code)
}
