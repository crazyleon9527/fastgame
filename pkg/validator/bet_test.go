package validator

import "testing"

func TestBetLimitsValidate(t *testing.T) {
	limits := BetLimits{Min: 1, Max: 100, Allowed: []float64{1, 5, 10, 50, 100}}

	cases := []struct {
		amount float64
		ok     bool
	}{
		{10, true},
		{-10, false},
		{0, false},
		{1000, false},
		{7, false},
	}

	for _, tc := range cases {
		err := limits.Validate(tc.amount)
		if tc.ok && err != nil {
			t.Fatalf("amount %.2f should pass: %v", tc.amount, err)
		}
		if !tc.ok && err == nil {
			t.Fatalf("amount %.2f should fail", tc.amount)
		}
	}
}
