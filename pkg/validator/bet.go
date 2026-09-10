package validator

import (
	"fmt"
	"math"
)

type BetLimits struct {
	Min     float64   `json:"min"`
	Max     float64   `json:"max"`
	Allowed []float64 `json:"allowed"`
}

func (l BetLimits) Validate(amount float64) error {
	if math.IsNaN(amount) || math.IsInf(amount, 0) {
		return fmt.Errorf("invalid bet amount")
	}
	if amount <= 0 {
		return fmt.Errorf("bet amount must be positive")
	}

	min := l.Min
	if min <= 0 {
		min = 0.01
	}
	max := l.Max
	if max <= 0 {
		max = 100000
	}
	if amount < min || amount > max {
		return fmt.Errorf("bet amount out of range [%.2f, %.2f]", min, max)
	}

	if len(l.Allowed) > 0 {
		for _, allowed := range l.Allowed {
			if math.Abs(amount-allowed) < 0.001 {
				return nil
			}
		}
		return fmt.Errorf("bet amount not in allowed list")
	}
	return nil
}
