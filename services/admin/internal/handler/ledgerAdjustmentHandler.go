package handler

import (
	"net/http"

	"fastgame/pkg/httputil"
	"fastgame/services/admin/internal/logic"
	"fastgame/services/admin/internal/svc"
	"fastgame/services/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// LedgerAdjustmentHandler 人工调账。
//
// 接口层只做三件事：解析、补客户端 IP（logic 拿不到 http.Request）、
// 把 logic 判定的状态码写出去。业务规则与错误语义都在 logic 里。
func LedgerAdjustmentHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.LedgerAdjustmentReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		// 操作人 IP 写入流水的 extra_data，用于人工调账的审计追溯。
		req.OperatorIp = httputil.ClientIP(r)

		l := logic.NewLedgerAdjustmentLogic(r.Context(), svcCtx)
		resp, err := l.Adjust(&req)
		if err != nil {
			// go-zero 默认把所有错误写成 400，这里按 logic 的判定区分
			// "调用方可修复（400）"与"服务端故障（500）"。
			status, msg := logic.LedgerErrStatus(err)
			logx.WithContext(r.Context()).Errorw("ledger_adjustment_failed",
				logx.Field("merchant_id", req.MerchantId),
				logx.Field("user_id", req.UserId),
				logx.Field("tx_type", req.TypeCode),
				logx.Field("status", status),
				logx.Field("err", err.Error()),
			)
			httpx.WriteJsonCtx(r.Context(), w, status, map[string]string{"message": msg})
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}
