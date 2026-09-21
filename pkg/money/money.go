package money

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
)

// Scale 最小法币精度：1.0000 单位 = 10000 minor（4 位小数，杜绝 float 账本误差）
const Scale int64 = 10000

var (
	ErrOverflow = errors.New("money arithmetic overflow")
)

// Amount 账本金额 int64 定点数（minor units）
type Amount int64

// Multiplier 倍率定点数，1.5x = 15000
type Multiplier int64

func AmountFromMinor(v int64) Amount         { return Amount(v) }
func MultiplierFromMinor(v int64) Multiplier { return Multiplier(v) }

// FromMajor 仅用于测试/配置边界转换
func FromMajor(units float64) Amount {
	return Amount(math.Round(units * float64(Scale)))
}

// MultiplierFromFloat 浮点数倍率转换，如 1.5 -> 15000
func MultiplierFromFloat(f float64) Multiplier {
	return Multiplier(math.Round(f * float64(Scale)))
}

func (a Amount) Minor() int64 { return int64(a) }

func (a Amount) IsPositive() bool { return a > 0 }
func (a Amount) IsZero() bool     { return a == 0 }
func (a Amount) IsNegative() bool { return a < 0 }

func (m Multiplier) Minor() int64 { return int64(m) }

// Apply 派彩计算: win = bet * multiplier / Scale
// 采用大数防溢出保护，并使用四舍五入(Half-Up)消除微小除法截断误差
func (m Multiplier) Apply(bet Amount) Amount {
	if bet <= 0 || m <= 0 {
		return 0
	}

	b := int64(bet)
	mult := int64(m)

	// 快速路径：若两者相乘不溢出 int64，直接定点数四舍五入计算
	// math.MaxInt64 / mult >= b 说明不会发生溢出
	if mult == 0 || b <= math.MaxInt64/mult {
		// 加上 Scale/2 实现四舍五入
		return Amount((b*mult + Scale/2) / Scale)
	}

	// 溢出安全路径（针对巨额或越南盾/印尼盾高倍率大奖）
	bigB := big.NewInt(b)
	bigM := big.NewInt(mult)
	bigScale := big.NewInt(Scale)
	halfScale := big.NewInt(Scale / 2)

	prod := new(big.Int).Mul(bigB, bigM)
	prod.Add(prod, halfScale)
	res := new(big.Int).Div(prod, bigScale)

	if !res.IsInt64() {
		// 如果依然超出上限，截断为 MaxInt64 防崩溃
		return Amount(math.MaxInt64)
	}

	return Amount(res.Int64())
}

// Add 安全加法 (带溢出保护)
func Add(a, b Amount) Amount {
	if (b > 0 && a > math.MaxInt64-b) || (b < 0 && a < math.MinInt64-b) {
		if b > 0 {
			return Amount(math.MaxInt64)
		}
		return Amount(math.MinInt64)
	}
	return a + b
}

// Sub 安全减法
func Sub(a, b Amount) Amount {
	return Add(a, -b)
}

// ToFloat 转换为显示用的浮点数值
func (a Amount) ToFloat() float64 {
	return float64(a) / float64(Scale)
}

// String 格式化为标准货币字符串，正确处理 -0.xxxx 的符号问题
func (a Amount) String() string {
	raw := a.Minor()
	sign := ""
	if raw < 0 {
		sign = "-"
		raw = -raw
	}
	whole := raw / Scale
	frac := raw % Scale
	return fmt.Sprintf("%s%d.%04d", sign, whole, frac)
}

// MarshalJSON 序列化为整数 minor units
func (a Amount) MarshalJSON() ([]byte, error) {
	return json.Marshal(int64(a))
}

// UnmarshalJSON 支持数字与字符串反序列化
func (a *Amount) UnmarshalJSON(data []byte) error {
	var num int64
	if err := json.Unmarshal(data, &num); err == nil {
		*a = Amount(num)
		return nil
	}
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	parsed, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return err
	}
	*a = Amount(parsed)
	return nil
}

// Scan 实现 database/sql Scanner 接口
func (a *Amount) Scan(value any) error {
	if value == nil {
		*a = 0
		return nil
	}
	switch v := value.(type) {
	case int64:
		*a = Amount(v)
	case []byte:
		n, err := strconv.ParseInt(string(v), 10, 64)
		if err != nil {
			return err
		}
		*a = Amount(n)
	case string:
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return err
		}
		*a = Amount(n)
	default:
		return fmt.Errorf("cannot scan type %T into money.Amount", value)
	}
	return nil
}

// Value 实现 database/sql/driver Valuer 接口
func (a Amount) Value() (driver.Value, error) {
	return int64(a), nil
}
