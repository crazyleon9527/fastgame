// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package handler

import (
	"net/http"

	"fastgame/services/rgs/internal/logic"
	"fastgame/services/rgs/internal/svc"
	"fastgame/services/rgs/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 玩家抛竿/下注并结算
func BetHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.BetReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := logic.NewBetLogic(r.Context(), svcCtx)
		resp, err := l.Bet(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
