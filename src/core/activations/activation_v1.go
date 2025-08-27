package activations

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"gitlab.com/aiku-open-source/go-help/src/core/any_base"
	"math"
	"reflect"
	"strconv"
)

// ActivationInterface 激活码接口
type ActivationInterface interface {
	GenerateActivationCode(any) (string, error)
	VerifyActivationCode(string) bool
}

// ActivationV1 简单激活码服务实现 (v1版本)
type ActivationV1 struct {
	SignatureLength int
	BaseChars       string
	secret          string
	total           int
}

// NewActivationV1 创建新的简单激活码服务实例
func NewActivationV1(signatureLength, total int, baseChars, secret string) *ActivationV1 {
	return &ActivationV1{
		SignatureLength: signatureLength, // 修改默认值为3
		BaseChars:       baseChars,
		secret:          secret,
		total:           total,
	}
}

// GenerateActivationCode 生成激活码
// number参数支持的类型包括:
//   - int, int32, int64: 直接使用
//   - uint, uint8, uint16, uint32, uint64, uintptr: 转换为int，超出范围会报错
//   - string: 尝试转换为整数，转换失败会报错
//   - float32, float64: 转换为整数，超出范围会报错
//
// number值必须在[0, total)范围内，其中total是创建ActivationV1时指定的总数
func (s *ActivationV1) GenerateActivationCode(number any) (res string, err error) {
	num, err := s.check(number, err)
	if err != nil {
		return
	}

	numberStr := s.getNumberStr(num)

	sign := s.getSign(numberStr)

	// 组合数据格式：签名前N位+numberStr+签名后N位
	combinedData := fmt.Sprintf("%s%s%s",
		sign[:s.SignatureLength], // 签名前N位
		numberStr,                // 数字部分
		sign[s.SignatureLength:]) // 签名后N位

	return combinedData, nil
}

// VerifyActivationCode 验证激活码
func (s *ActivationV1) VerifyActivationCode(code string) bool {
	// 计算总位数
	count := s.getCount(s.total)
	// 提取数字部分（中间剩余部分）
	numberStr := code[s.SignatureLength : len(code)-s.SignatureLength]

	done := s.checkByCode(code, count, numberStr)
	if done {
		return false
	}

	sign := s.getSignByCode(code)

	return sign == s.getSign(numberStr)
}

// getCount 根据总数计算数字位数
func (s *ActivationV1) getCount(total int) int {
	if total <= 0 {
		return 1
	}

	count := 0
	number := len(s.BaseChars)
	temp := total - 1
	for temp > 0 {
		count++
		temp /= number
	}

	// 至少需要1位
	if count == 0 {
		count = 1
	}

	return count
}

// checkByCode 检查code
func (s *ActivationV1) checkByCode(code string, count int, numberStr string) bool {
	// 检查长度（需要包含两段签名和数字）
	if len(code) != count+s.SignatureLength*2 {
		return true
	}
	check := s.checkByNumber(numberStr)
	if check {
		return true
	}
	return false
}

// check 检查数字
func (s *ActivationV1) checkByNumber(numberStr string) bool {
	// 从62进制解析数字
	number := any_base.AnyToDecimal(numberStr, []rune(s.BaseChars))

	// 验证数字范围
	if number < 0 || number >= s.total {
		return true
	}
	return false
}

// check 检查数字
func (s *ActivationV1) check(number any, err error) (int, error) {
	num, err := s.getNumber(number)
	if err != nil {
		return 0, nil
	}

	// 值范围检查
	if num < 0 || num >= s.total {
		err = fmt.Errorf("invalid number: must be between 0 and %d", s.total-1)
		return 0, nil
	}
	return num, err
}

// getSign 获取签名
func (s *ActivationV1) getSignByCode(code string) string {
	// 分割签名（前N个字符 + 后N个字符）
	part1 := code[:s.SignatureLength]
	part2 := code[len(code)-s.SignatureLength:]
	// 生成完整签名
	sign := part1 + part2
	return sign
}

// getSign 获取签名
func (s *ActivationV1) getSign(numberStr string) string {
	// 生成HMAC签名
	mac := hmac.New(sha256.New, []byte(s.secret))
	mac.Write([]byte(numberStr))
	sign := hex.EncodeToString(mac.Sum(nil))[:s.SignatureLength*2] // 截取需要长度的签名
	return sign
}

// getNumberStr 获取数字字符串
func (s *ActivationV1) getNumberStr(num int) string {
	// 转换为62进制并补码
	numberStr := any_base.DecimalToAny(num, []rune(s.BaseChars))

	// 补齐位数
	count := s.getCount(s.total)
	for len(numberStr) < count {
		numberStr = string([]rune(s.BaseChars)[0]) + numberStr
	}
	return numberStr
}

// getNumber 获取数字
func (s *ActivationV1) getNumber(number any) (int, error) {
	var num int
	switch v := number.(type) {
	case int:
		num = v
	case int32:
		num = int(v)
	case int64:
		num = int(v)
	case uint:
		if v > math.MaxInt64 { // 先检查是否超过int64范围
			return 0, fmt.Errorf("uint value exceeds integer range: %v", v)
		}
		num = int(v)
	case uint8: // 单独处理byte类型
		num = int(v)
	case uint16:
		num = int(v)
	case uint32:
		num = int(v)
	case uint64:
		if v > math.MaxInt64 {
			return 0, fmt.Errorf("uint64 value exceeds integer range: %v", v)
		}
		num = int(v)
	case uintptr:
		num = int(v)
	case string:
		// 字符串转整型
		i, err := strconv.Atoi(v)
		if err != nil {
			return 0, fmt.Errorf("invalid string number: %v", err)
		}
		num = i
	case float32, float64:
		// 浮点数转整型（检查是否超出范围）
		f := reflect.ValueOf(v).Float()
		if f > float64(math.MaxInt) || f < float64(math.MinInt) {
			return 0, fmt.Errorf("float value out of integer range: %v", f)
		}
		num = int(f)
	default:
		return 0, fmt.Errorf("unsupported number type: %T, must be int, string or float", number)
	}
	return num, nil
}
