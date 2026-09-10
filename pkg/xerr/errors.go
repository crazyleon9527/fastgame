package xerr

import "errors"

var (
	ErrLockBusy        = errors.New("player action in progress")
	ErrDuplicateRound  = errors.New("duplicate round id")
	ErrWalletBetFailed = errors.New("wallet bet failed")
	ErrWalletWinFailed = errors.New("wallet win failed")
)
