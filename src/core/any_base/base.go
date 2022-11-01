package any_base

import (
	"math"
	"sort"
	"strconv"
	"strings"
)

// 10进制数转换 n 进制
func DecimalToAny(num int, num2char []rune) string {
	length := len(num2char)
	var str []rune
	for {
		if num <= 0 {
			break
		}
		quotient := num % length // 余数
		str = append([]rune{num2char[quotient]}, str...)
		num = num / length // 商数
	}
	return string(str)
}

// n 进制数转换 10 进制
func AnyToDecimal(str string, num2char []rune) int {
	//    $len = strlen($bas);
	//    $str = strrev($str);
	//    $num = 0;
	//    for ($i = 0; $i < strlen($str); $i++) {
	//        $pos = strpos($bas, $str[$i]);
	//        $num = $num + (pow($len, $i) * $pos);
	//    }
	length := float64(len(num2char))

	r := Reverse([]rune(str))
	max := len(r)

	var num float64
	for i := 0; i < max; i++ {
		num = num + (math.Pow(length, float64(i)) * float64(find(num2char, r[i])))
	}
	return int(num)
}

func find(num2char []rune, str rune) int {
	for i, s := range num2char {
		if s == str {
			return i
		}
	}
	return -1
}

func Reverse(r []rune) []rune {
	// write code here
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return r
}

func GetMap() (m map[int]struct{}) {
	m = map[int]struct{}{
		38: {},
		60: {},
		62: {},
		34: {},
		92: {},
	}
	for i := 126; i < 161; i++ {
		m[i] = struct{}{}
	}
	return
}

func GetTenToAny(m map[int]struct{}) (tenToAny []rune) {
	var ok bool
	//var n int
	for i := 33; i < 256; i++ {
		if _, ok = m[i]; ok {
			continue
		}
		//tenToAny[n] = string(rune(i))
		//n++
		tenToAny = append(tenToAny, rune(i))
	}
	return
}

func IntegerGroupingEncode(list []int64, sep string) (res string) {
	if len(list) == 0 {
		return
	}
	var s string
	var prev int64
	sort.Slice(list, func(i, j int) bool {
		return list[i] < list[j]
	})
	var r []string
	for _, v := range list {
		s = strconv.FormatInt(v-prev, 10)
		r = append(r, s)
		prev = v
	}
	res = strings.Join(r, sep)

	return
}

func IntegerGroupingDecode(input string, sep string) (res []int64) {
	if len(input) == 0 {
		return
	}
	list := strings.Split(input, sep)
	var prev int64
	var tmp int64
	for _, v := range list {
		ii, _ := strconv.ParseInt(v, 10, 64)
		tmp = ii
		prev += tmp
		res = append(res, prev)
	}
	return
}
