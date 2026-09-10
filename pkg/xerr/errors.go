package xerr

import "errors"

var (
	ErrLockBusy        = errors.New("player action in progress")
	ErrDuplicateRound  = errors.New("duplicate round id")
	ErrWalletBetFailed = errors.New("wallet bet failed")
	ErrWalletWinFailed = errors.New("wallet win failed")
	ErrMerchantInvalid = errors.New("merchant invalid or disabled")
	ErrInvalidRequest  = errors.New("invalid request")
	ErrUnauthorized    = errors.New("unauthorized")
	ErrRateLimited     = errors.New("too many requests")
	ErrBlocked         = errors.New("access denied")
)
