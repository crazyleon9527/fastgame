package handler

import (
	"fmt"
	"net/http"

	"fastgame/services/admin/internal/logic"
	"fastgame/services/admin/internal/svc"
	"fastgame/services/admin/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

const excelMIME = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

func UploadAssetHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(svcCtx.UploadMaxBytes); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		_, header, err := r.FormFile("file")
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := logic.NewFileLogic(r.Context(), svcCtx)
		resp, err := l.UploadAsset(header)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}

func ExportMerchantsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := logic.NewExcelLogic(r.Context(), svcCtx)
		data, filename, err := l.ExportMerchants()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		writeExcelAttachment(w, filename, data)
	}
}

func ExportPlatformGamesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := logic.NewExcelLogic(r.Context(), svcCtx)
		data, filename, err := l.ExportPlatformGames()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		writeExcelAttachment(w, filename, data)
	}
}

func ExportMerchantGamesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.MerchantGameListReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := logic.NewExcelLogic(r.Context(), svcCtx)
		data, filename, err := l.ExportMerchantGames(req.MerchantId)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		writeExcelAttachment(w, filename, data)
	}
}

func ImportMerchantGamesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(svcCtx.UploadMaxBytes); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		_, header, err := r.FormFile("file")
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := logic.NewExcelLogic(r.Context(), svcCtx)
		n, err := l.ImportMerchantGames(header)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, map[string]int{"imported": n})
		}
	}
}

func ImportPlatformGamesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(svcCtx.UploadMaxBytes); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		_, header, err := r.FormFile("file")
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := logic.NewExcelLogic(r.Context(), svcCtx)
		n, err := l.ImportPlatformGames(header)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, map[string]int{"imported": n})
		}
	}
}

func writeExcelAttachment(w http.ResponseWriter, filename string, data []byte) {
	w.Header().Set("Content-Type", excelMIME)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	_, _ = w.Write(data)
}
