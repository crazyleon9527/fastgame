// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package handler

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"fastgame/pkg/httputil"
	"fastgame/pkg/security"
	"fastgame/pkg/xerr"
	"fastgame/services/rgs/internal/logic"
	"fastgame/services/rgs/internal/svc"
	"fastgame/services/rgs/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// 玩家抛竿/下注并结算 — 仅接受 action=cast + betAmount，拒绝任何游戏结果类字段
func BetHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		defer security.NormalizeResponseDelay(start, svcCtx.Config.Security.MinResponseDelay())

		bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			writeBetError(w, r, http.StatusBadRequest, xerr.ErrInvalidRequest)
			return
		}

		if err := svcCtx.Guard.CheckRequest(r, string(bodyBytes)); err != nil {
			writeSecurityError(w, r, err)
			return
		}

		var req types.BetReq
		if err := httputil.StrictUnmarshal(bodyBytes, &req); err != nil {
			writeBetError(w, r, http.StatusBadRequest, xerr.ErrInvalidRequest)
			return
		}

		gameCfg, err := svcCtx.GameConfig.Load(r.Context(), req.MerchantId, req.GameCode)
		if err != nil {
			writeBetError(w, r, http.StatusBadRequest, xerr.ErrMerchantInvalid)
			return
		}

		limits := security.ParseBetLimits(gameCfg.Raw)
		if err := svcCtx.Guard.CheckBet(r.Context(), security.BetCheckInput{
			Method:     r.Method,
			Path:       r.URL.Path,
			Body:       string(bodyBytes),
			MerchantID: req.MerchantId,
			UserID:     req.UserId,
			ClientIP:   httputil.ClientIP(r),
			Headers:    r.Header,
		}, limits, req.BetAmount); err != nil { // betAmount: int64 minor units
			writeSecurityError(w, r, err)
			return
		}
		if !checkUserRateLimit(w, r, svcCtx, req.UserId) {
			return
		}

		l := logic.NewBetLogic(r.Context(), svcCtx)
		resp, err := l.Bet(&req)
		if err != nil {
			writeBetError(w, r, betErrorStatus(err), err)
			return
		}
		if resp != nil && resp.SettlementStatus == "pending" {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusAccepted, resp)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}

func betErrorStatus(err error) int {
	switch {
	case errors.Is(err, xerr.ErrLockBusy):
		return http.StatusTooManyRequests
	case errors.Is(err, xerr.ErrRateLimited):
		return http.StatusTooManyRequests
	case errors.Is(err, xerr.ErrInvalidSession), errors.Is(err, xerr.ErrInvalidSequence):
		return http.StatusUnauthorized
	case errors.Is(err, xerr.ErrDuplicateRound):
		return http.StatusConflict
	case errors.Is(err, xerr.ErrWalletUnavailable):
		return http.StatusServiceUnavailable
	case errors.Is(err, xerr.ErrUnauthorized):
		return http.StatusUnauthorized
	default:
		return http.StatusBadRequest
	}
}

func writeBetError(w http.ResponseWriter, r *http.Request, status int, err error) {
	httpx.WriteJsonCtx(r.Context(), w, status, map[string]string{
		"message": err.Error(),
	})
}

func writeSecurityError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case strings.Contains(err.Error(), "not whitelisted"):
		writeBetError(w, r, http.StatusForbidden, xerr.ErrBlocked)
	case strings.Contains(err.Error(), "blocked"), strings.Contains(err.Error(), "suspicious"), strings.Contains(err.Error(), "not allowed"):
		writeBetError(w, r, http.StatusForbidden, xerr.ErrBlocked)
	case strings.Contains(err.Error(), "rate limit"):
		writeBetError(w, r, http.StatusTooManyRequests, xerr.ErrRateLimited)
	case strings.Contains(err.Error(), "signature"), strings.Contains(err.Error(), "nonce"), strings.Contains(err.Error(), "timestamp"):
		writeBetError(w, r, http.StatusUnauthorized, xerr.ErrUnauthorized)
	default:
		writeBetError(w, r, http.StatusBadRequest, xerr.ErrInvalidRequest)
	}
}
