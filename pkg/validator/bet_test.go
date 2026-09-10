package validator

import (
	"testing"

	"fastgame/pkg/money"
)

func TestBetLimitsValidate(t *testing.T) {
	limits := BetLimits{
		Min:     money.FromMajor(1).Minor(),
		Max:     money.FromMajor(100).Minor(),
		Allowed: []int64{money.FromMajor(1).Minor(), money.FromMajor(5).Minor(), money.FromMajor(10).Minor(), money.FromMajor(50).Minor(), money.FromMajor(100).Minor()},
	}

	cases := []struct {
		amount money.Amount
		ok     bool
	}{
		{money.FromMajor(10), true},
		{money.FromMajor(-10), false},
		{0, false},
		{money.FromMajor(1000), false},
		{money.FromMajor(7), false},
	}

	for _, tc := range cases {
		err := limits.Validate(tc.amount)
		if tc.ok && err != nil {
			t.Fatalf("amount %s should pass: %v", tc.amount.String(), err)
		}
		if !tc.ok && err == nil {
			t.Fatalf("amount %s should fail", tc.amount.String())
		}
	}
}
