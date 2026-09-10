// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package handler

import (
	"encoding/json"
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

// 玩家抛竿/下注并结算
func BetHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		defer security.NormalizeResponseDelay(start, svcCtx.Config.Security.MinResponseDelay())

		bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, xerr.ErrInvalidRequest)
			return
		}

		if err := svcCtx.Guard.CheckRequest(r, string(bodyBytes)); err != nil {
			writeSecurityError(w, r, err)
			return
		}

		var req types.BetReq
		if err := json.Unmarshal(bodyBytes, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, xerr.ErrInvalidRequest)
			return
		}

		gameCfg, err := svcCtx.GameConfig.Load(r.Context(), req.MerchantId, req.GameCode)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, xerr.ErrMerchantInvalid)
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
		}, limits, req.BetAmount); err != nil {
			writeSecurityError(w, r, err)
			return
		}

		l := logic.NewBetLogic(r.Context(), svcCtx)
		resp, err := l.Bet(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}

func writeSecurityError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case strings.Contains(err.Error(), "blocked"), strings.Contains(err.Error(), "suspicious"), strings.Contains(err.Error(), "not allowed"):
		httpx.ErrorCtx(r.Context(), w, xerr.ErrBlocked)
	case strings.Contains(err.Error(), "rate limit"):
		httpx.ErrorCtx(r.Context(), w, xerr.ErrRateLimited)
	case strings.Contains(err.Error(), "signature"), strings.Contains(err.Error(), "nonce"), strings.Contains(err.Error(), "timestamp"):
		httpx.ErrorCtx(r.Context(), w, xerr.ErrUnauthorized)
	default:
		if errors.Is(err, xerr.ErrInvalidRequest) {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.ErrorCtx(r.Context(), w, xerr.ErrInvalidRequest)
	}
}
