package any_base

import (
	"fmt"
	"testing"
)

func init() {

}

func TestA(t *testing.T) {
	res := DecimalToAny(20152015, 50)
	fmt.Println(res, AnyToDecimal(res, 50))
	res = Base76Encode(20152015)
	fmt.Println(res, Base76Decode(res))
}
