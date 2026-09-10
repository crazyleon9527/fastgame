// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package handler

import (
	"net/http"

	"fastgame/pkg/httputil"
	"fastgame/services/rgs/internal/logic"
	"fastgame/services/rgs/internal/svc"
	"fastgame/services/rgs/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// 查询玩家余额
func BalanceHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := svcCtx.Guard.CheckRequest(r, ""); err != nil {
			writeSecurityError(w, r, err)
			return
		}

		var req types.BalanceReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		if err := svcCtx.Guard.CheckAccess(r.Context(), httputil.ClientIP(r), req.MerchantId, req.UserId); err != nil {
			writeSecurityError(w, r, err)
			return
		}

		l := logic.NewBalanceLogic(r.Context(), svcCtx)
		resp, err := l.Balance(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
