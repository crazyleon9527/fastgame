package handler

import (
	"io"
	"net/http"

	"fastgame/pkg/httputil"
	"fastgame/pkg/xerr"
	"fastgame/services/rgs/internal/logic"
	"fastgame/services/rgs/internal/svc"
	"fastgame/services/rgs/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func SessionHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := svcCtx.Guard.CheckRequest(r, ""); err != nil {
			writeSecurityError(w, r, err)
			return
		}

		bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			logx.Errorf(">>> StrictUnmarshal err: %v", err)
			httpx.ErrorCtx(r.Context(), w, xerr.ErrInvalidRequest)
			return
		}

		var req types.SessionReq
		if err := httputil.StrictUnmarshal(bodyBytes, &req); err != nil {
			logx.Errorf(">>> CheckAccess err: %v", err)
			httpx.ErrorCtx(r.Context(), w, xerr.ErrInvalidRequest)
			return
		}

		if err := svcCtx.Guard.CheckAccess(r.Context(), httputil.ClientIP(r), req.MerchantId, req.UserId); err != nil {
			println(">>> CHECK_ACCESS_FAIL:", err.Error())
			writeSecurityError(w, r, err)
			return
		}
		if !checkUserRateLimit(w, r, svcCtx, req.UserId) {
			println(">>> USER_RATE_LIMIT_FAIL")
			return
		}

		l := logic.NewSessionLogic(r.Context(), svcCtx)
		resp, err := l.CreateSession(&req)
		if err != nil {
			logx.Errorf(">>> CreateSession err: %v", err)
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}
