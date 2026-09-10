package wallet

import (
	"context"
	"errors"
	"net"
	"strings"
)

var (
	ErrTimeout = errors.New("wallet request timeout")
)

func IsTimeoutErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "timeout")
}
