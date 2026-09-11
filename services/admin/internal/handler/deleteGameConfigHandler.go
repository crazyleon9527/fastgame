package handler

import (
	"net/http"

	"fastgame/services/admin/internal/logic"
	"fastgame/services/admin/internal/svc"
	"fastgame/services/admin/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

func DeleteGameConfigHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DeleteGameConfigReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := logic.NewDeleteGameConfigLogic(r.Context(), svcCtx)
		if err := l.DeleteGameConfig(req.Id); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			w.WriteHeader(http.StatusNoContent)
		}
	}
}
