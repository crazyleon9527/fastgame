package money

import (
	"fmt"
	"math"
)

// Scale 最小法币精度：1.0000 单位 = 10000 minor（4 位小数，杜绝 float 账本误差）
const Scale int64 = 10000

// Amount 账本金 int64 定点数（minor units）
type Amount int64

// Multiplier 倍率定点数，1.5x = 15000
type Multiplier int64

func AmountFromMinor(v int64) Amount   { return Amount(v) }
func MultiplierFromMinor(v int64) Multiplier { return Multiplier(v) }

// FromMajor 仅用于测试/种子数据边界转换
func FromMajor(units float64) Amount {
	return Amount(math.Round(units * float64(Scale)))
}

func (a Amount) Minor() int64 { return int64(a) }

func (a Amount) IsPositive() bool { return a > 0 }

func (m Multiplier) Minor() int64 { return int64(m) }

// Apply win = bet * multiplier / Scale（纯整数）
func (m Multiplier) Apply(bet Amount) Amount {
	if bet <= 0 || m <= 0 {
		return 0
	}
	return Amount(int64(bet) * int64(m) / Scale)
}

func Add(a, b Amount) Amount { return a + b }
func Sub(a, b Amount) Amount { return a - b }

func (a Amount) String() string {
	whole := a.Minor() / Scale
	frac := a.Minor() % Scale
	if frac < 0 {
		frac = -frac
	}
	return fmt.Sprintf("%d.%04d", whole, frac)
}
