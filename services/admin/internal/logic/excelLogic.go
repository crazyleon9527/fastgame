package logic

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"strconv"
	"strings"

	"fastgame/internal/model"
	"fastgame/pkg/auth"
	"fastgame/services/admin/internal/svc"

	"github.com/xuri/excelize/v2"
	"github.com/zeromicro/go-zero/core/logx"
)

type ExcelLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewExcelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ExcelLogic {
	return &ExcelLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ExcelLogic) ExportMerchants() ([]byte, string, error) {
	list, _, err := l.svcCtx.Merchants.ListPage(l.ctx, 1, 5000)
	if err != nil {
		return nil, "", err
	}
	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	headers := []string{"id", "merchantCode", "name", "status"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, h)
	}
	for i, m := range list {
		row := i + 2
		_ = f.SetCellValue(sheet, cellName(1, row), m.Id)
		_ = f.SetCellValue(sheet, cellName(2, row), m.MerchantCode)
		_ = f.SetCellValue(sheet, cellName(3, row), m.Name)
		_ = f.SetCellValue(sheet, cellName(4, row), m.Status)
	}
	return writeExcel(f, "merchants")
}

func (l *ExcelLogic) ExportPlatformGames() ([]byte, string, error) {
	list, _, err := l.svcCtx.PlatformGames.ListGamesPage(l.ctx, 1, 5000, "")
	if err != nil {
		return nil, "", err
	}
	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	headers := []string{"gameCode", "name", "gameType", "categoryCode", "defaultRtpPpm", "minBetMinor", "maxBetMinor", "clientVersion", "status", "thumbnailUrl"}
	for i, h := range headers {
		_ = f.SetCellValue(sheet, cellName(i+1, 1), h)
	}
	for i, g := range list {
		row := i + 2
		_ = f.SetCellValue(sheet, cellName(1, row), g.GameCode)
		_ = f.SetCellValue(sheet, cellName(2, row), g.Name)
		_ = f.SetCellValue(sheet, cellName(3, row), g.GameType)
		_ = f.SetCellValue(sheet, cellName(4, row), g.CategoryCode)
		_ = f.SetCellValue(sheet, cellName(5, row), g.DefaultRtpPpm)
		_ = f.SetCellValue(sheet, cellName(6, row), g.MinBetMinor)
		_ = f.SetCellValue(sheet, cellName(7, row), g.MaxBetMinor)
		if g.ClientVersion.Valid {
			_ = f.SetCellValue(sheet, cellName(8, row), g.ClientVersion.String)
		}
		_ = f.SetCellValue(sheet, cellName(9, row), g.Status)
		if g.ThumbnailUrl.Valid {
			_ = f.SetCellValue(sheet, cellName(10, row), g.ThumbnailUrl.String)
		}
	}
	return writeExcel(f, "platform-games")
}

func (l *ExcelLogic) ExportMerchantGames(merchantID uint64) ([]byte, string, error) {
	list, _, err := l.svcCtx.PlatformGames.ListMerchantGamesPage(l.ctx, merchantID, 1, 5000)
	if err != nil {
		return nil, "", err
	}
	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	headers := []string{"merchantCode", "gameCode", "gameName", "rtpTierCode", "sortOrder", "status", "minBetMinor", "maxBetMinor"}
	for i, h := range headers {
		_ = f.SetCellValue(sheet, cellName(i+1, 1), h)
	}
	for i, row := range list {
		r := i + 2
		_ = f.SetCellValue(sheet, cellName(1, r), row.MerchantCode)
		_ = f.SetCellValue(sheet, cellName(2, r), row.GameCode)
		_ = f.SetCellValue(sheet, cellName(3, r), row.GameName)
		if row.RtpTierCode.Valid {
			_ = f.SetCellValue(sheet, cellName(4, r), row.RtpTierCode.String)
		}
		_ = f.SetCellValue(sheet, cellName(5, r), row.SortOrder)
		_ = f.SetCellValue(sheet, cellName(6, r), row.Status)
		if row.MinBetMinor.Valid {
			_ = f.SetCellValue(sheet, cellName(7, r), row.MinBetMinor.Int64)
		}
		if row.MaxBetMinor.Valid {
			_ = f.SetCellValue(sheet, cellName(8, r), row.MaxBetMinor.Int64)
		}
	}
	name := fmt.Sprintf("merchant-games-%d", merchantID)
	return writeExcel(f, name)
}

func (l *ExcelLogic) ImportMerchantGames(header *multipart.FileHeader) (int, error) {
	rows, err := readExcelRows(header)
	if err != nil {
		return 0, err
	}
	if len(rows) < 2 {
		return 0, errors.New("excel is empty")
	}
	col := mapHeader(rows[0])
	imported := 0
	for _, row := range rows[1:] {
		mCode := cell(row, col, "merchantcode", "merchant_code")
		gCode := cell(row, col, "gamecode", "game_code")
		if mCode == "" || gCode == "" {
			continue
		}
		merchant, err := l.svcCtx.Merchants.FindOneByMerchantCode(l.ctx, mCode)
		if err != nil {
			continue
		}
		game, err := l.svcCtx.PlatformGames.FindGameByCode(l.ctx, gCode)
		if err != nil {
			continue
		}
		exists, _ := l.svcCtx.PlatformGames.MerchantGameExists(l.ctx, merchant.Id, game.Id)
		if exists {
			continue
		}
		tier := cell(row, col, "rtptiercode", "rtp_tier_code", "rtptier")
		if tier == "" {
			tier = "default"
		}
		sortOrder, _ := strconv.ParseInt(cell(row, col, "sortorder", "sort_order"), 10, 64)
		status, _ := strconv.ParseInt(cell(row, col, "status"), 10, 64)
		if status == 0 {
			status = 1
		}
		mg := &model.MerchantGameRow{
			MerchantId: merchant.Id,
			GameId:     game.Id,
			SortOrder:  sortOrder,
			Status:     status,
		}
		mg.RtpTierCode.String = tier
		mg.RtpTierCode.Valid = true
		if _, err := l.svcCtx.PlatformGames.InsertMerchantGame(l.ctx, mg); err == nil {
			imported++
		}
	}
	return imported, nil
}

func (l *ExcelLogic) ImportPlatformGames(header *multipart.FileHeader) (int, error) {
	if roleNameFromCtx(l.ctx) != auth.RoleAdmin {
		return 0, errors.New("only admin can import platform games")
	}
	rows, err := readExcelRows(header)
	if err != nil {
		return 0, err
	}
	if len(rows) < 2 {
		return 0, errors.New("excel is empty")
	}
	col := mapHeader(rows[0])
	categories, _ := l.svcCtx.PlatformGames.ListCategories(l.ctx)
	catByCode := map[string]uint64{}
	for _, c := range categories {
		catByCode[strings.ToLower(c.Code)] = c.Id
	}
	imported := 0
	for _, row := range rows[1:] {
		code := cell(row, col, "gamecode", "game_code")
		name := cell(row, col, "name")
		if code == "" || name == "" {
			continue
		}
		if _, err := l.svcCtx.PlatformGames.FindGameByCode(l.ctx, code); err == nil {
			continue
		}
		gameType := cell(row, col, "gametype", "game_type")
		if gameType == "" {
			gameType = "slot"
		}
		rtp, _ := strconv.ParseInt(cell(row, col, "defaultrtpppm", "default_rtp_ppm", "rtpppm"), 10, 64)
		if rtp <= 0 {
			rtp = 960000
		}
		minBet, _ := strconv.ParseInt(cell(row, col, "minbetminor", "min_bet_minor"), 10, 64)
		if minBet <= 0 {
			minBet = 10000
		}
		maxBet, _ := strconv.ParseInt(cell(row, col, "maxbetminor", "max_bet_minor"), 10, 64)
		if maxBet <= 0 {
			maxBet = 10000000
		}
		status, _ := strconv.ParseInt(cell(row, col, "status"), 10, 64)
		if status == 0 {
			status = 1
		}
		g := &model.PlatformGame{
			GameCode: code, Name: name, GameType: gameType,
			DefaultRtpPpm: rtp, MinBetMinor: minBet, MaxBetMinor: maxBet, Status: status,
		}
		if catCode := strings.ToLower(cell(row, col, "categorycode", "category_code")); catCode != "" {
			if id, ok := catByCode[catCode]; ok {
				g.CategoryId.Int64 = int64(id)
				g.CategoryId.Valid = true
			}
		}
		if thumb := cell(row, col, "thumbnailurl", "thumbnail_url"); thumb != "" {
			g.ThumbnailUrl.String = thumb
			g.ThumbnailUrl.Valid = true
		}
		id, err := l.svcCtx.PlatformGames.InsertGame(l.ctx, g)
		if err != nil {
			continue
		}
		_ = l.svcCtx.PlatformGames.InsertRtpTier(l.ctx, &model.GameRtpTier{
			GameId: id, TierCode: "default", TargetRtpPpm: rtp,
			ParSheetRef: code + "/par-v1", Weight: 100,
		})
		imported++
	}
	return imported, nil
}

func writeExcel(f *excelize.File, name string) ([]byte, string, error) {
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), name + ".xlsx", nil
}

func readExcelRows(header *multipart.FileHeader) ([][]string, error) {
	file, err := header.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()
	xf, err := excelize.OpenReader(file)
	if err != nil {
		return nil, err
	}
	defer xf.Close()
	sheet := xf.GetSheetName(0)
	return xf.GetRows(sheet)
}

func mapHeader(header []string) map[string]int {
	m := make(map[string]int, len(header))
	for i, h := range header {
		key := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(h), " ", ""))
		m[key] = i
	}
	return m
}

func cell(row []string, col map[string]int, keys ...string) string {
	for _, k := range keys {
		if idx, ok := col[k]; ok && idx < len(row) {
			return strings.TrimSpace(row[idx])
		}
	}
	return ""
}

func cellName(col, row int) string {
	c, _ := excelize.CoordinatesToCellName(col, row)
	return c
}
