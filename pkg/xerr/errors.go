package xerr

import (
	"errors"
	"fmt"
	"net/http"
)

// CodeError 结构化业务错误
type CodeError struct {
	code       int    // 内部业务错误码（如 40001、50002）
	httpStatus int    // 对应的标准 HTTP 状态码（如 400、401、429、503）
	msg        string // 面向客户端的安全展示文案
	cause      error  // 内部排查用的底层错误链（不对外暴露）
}

func (e *CodeError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("[%d] %s: %v", e.code, e.msg, e.cause)
	}
	return fmt.Sprintf("[%d] %s", e.code, e.msg)
}

func (e *CodeError) Code() int {
	return e.code
}

func (e *CodeError) HttpStatus() int {
	if e.httpStatus == 0 {
		return http.StatusOK // 默认业务异常仍走 200 包裹业务码
	}
	return e.httpStatus
}

func (e *CodeError) Msg() string {
	return e.msg
}

func (e *CodeError) Cause() error {
	return e.cause
}

// Unwrap 实现 Go 1.13+ 标准错误解包，兼容 errors.Is 与 errors.As
func (e *CodeError) Unwrap() error {
	return e.cause
}

// Is 实现自定义错误比对（按业务 code 或底层根因判断）
func (e *CodeError) Is(target error) bool {
	if t, ok := target.(*CodeError); ok {
		return e.code == t.code
	}
	return errors.Is(e.cause, target)
}

// WithCause 携带底层真实错误链（用于记录日志，不改动暴露给客户端的 code 和 msg）
func (e *CodeError) WithCause(cause error) *CodeError {
	return &CodeError{
		code:       e.code,
		httpStatus: e.httpStatus,
		msg:        e.msg,
		cause:      cause,
	}
}

// WithMsg 自定义当前错误的展示文案
func (e *CodeError) WithMsg(msg string) *CodeError {
	return &CodeError{
		code:       e.code,
		httpStatus: e.httpStatus,
		msg:        msg,
		cause:      e.cause,
	}
}

// NewCodeError 构造自定义错误
func NewCodeError(code, httpStatus int, msg string) *CodeError {
	return &CodeError{
		code:       code,
		httpStatus: httpStatus,
		msg:        msg,
	}
}

// -----------------------------------------------------------------------------
// 标准全局业务错误定义
// 规则：
// 400xx: 客户端/会话/参数异常
// 429xx: 频控与并发冲突
// 500xx: 内部业务与对账异常
// 503xx: 外部三方服务不可用
// -----------------------------------------------------------------------------

var (
	// 会话与鉴权 (400xx / 401xx)
	ErrUnauthorized    = NewCodeError(40101, http.StatusUnauthorized, "unauthorized access")
	ErrBlocked         = NewCodeError(40301, http.StatusForbidden, "access denied by security guard")
	ErrMerchantInvalid = NewCodeError(40302, http.StatusForbidden, "merchant invalid or disabled")
	ErrInvalidSession  = NewCodeError(40001, http.StatusBadRequest, "invalid or expired session")
	ErrInvalidSequence = NewCodeError(40002, http.StatusBadRequest, "invalid sequence id")
	ErrInvalidRequest  = NewCodeError(40003, http.StatusBadRequest, "invalid request parameters")

	// 频控与并发冲突 (429xx)
	ErrLockBusy    = NewCodeError(42901, http.StatusTooManyRequests, "player action in progress")
	ErrRateLimited = NewCodeError(42902, http.StatusTooManyRequests, "too many requests, please slow down")

	// 幂等与注单状态 (409xx)
	ErrDuplicateRound = NewCodeError(40901, http.StatusConflict, "duplicate round id")

	// 钱包与资金交互 (503xx / 500xx)
	ErrWalletUnavailable = NewCodeError(50301, http.StatusServiceUnavailable, "wallet service temporarily unavailable")
	ErrWalletBetFailed   = NewCodeError(50001, http.StatusOK, "wallet bet debit failed")
	ErrWalletWinFailed   = NewCodeError(50002, http.StatusOK, "wallet win credit failed")
	ErrSettlementPending = NewCodeError(50003, http.StatusOK, "settlement pending reconciliation")
)

// FromError 从任意 error 中提取 CodeError，若未定义则按未捕获系统错误兜底
func FromError(err error) *CodeError {
	if err == nil {
		return nil
	}
	var ce *CodeError
	if errors.As(err, &ce) {
		return ce
	}
	return &CodeError{
		code:       50000,
		httpStatus: http.StatusInternalServerError,
		msg:        "internal server error",
		cause:      err,
	}
}
