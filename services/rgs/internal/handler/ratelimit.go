package handler

import (
	"net/http"

	"fastgame/pkg/xerr"
	"fastgame/services/rgs/internal/svc"
)

func checkUserRateLimit(w http.ResponseWriter, r *http.Request, svcCtx *svc.ServiceContext, userID uint64) bool {
	if svcCtx.RateLimit != nil && !svcCtx.RateLimit.AllowUser(r.Context(), userID) {
		writeBetError(w, r, http.StatusTooManyRequests, xerr.ErrRateLimited)
		return false
	}
	return true
}
