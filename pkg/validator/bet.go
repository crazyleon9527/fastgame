package validator

import (
	"fmt"

	"fastgame/pkg/money"
)

type BetLimits struct {
	Min     int64   `json:"min"`
	Max     int64   `json:"max"`
	Allowed []int64 `json:"allowed"`
}

func (l BetLimits) Validate(amount money.Amount) error {
	if amount <= 0 {
		return fmt.Errorf("bet amount must be positive")
	}

	min := l.Min
	if min <= 0 {
		min = money.Scale / 100 // 0.01
	}
	max := l.Max
	if max <= 0 {
		max = 100000 * money.Scale
	}
	amt := amount.Minor()
	if amt < min || amt > max {
		return fmt.Errorf("bet amount out of range")
	}

	if len(l.Allowed) > 0 {
		for _, allowed := range l.Allowed {
			if amt == allowed {
				return nil
			}
		}
		return fmt.Errorf("bet amount not in allowed list")
	}
	return nil
}
