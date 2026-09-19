package svc

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"fastgame/internal/model"
	"fastgame/pkg/clickhouse"
	"fastgame/pkg/fieldcipher"
	"fastgame/pkg/ledger"
	"fastgame/pkg/security"
	"fastgame/services/admin/internal/config"
	"fastgame/services/admin/internal/middleware"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/rest"
)

type ServiceContext struct {
	Config            config.Config
	AuthMiddleware    rest.Middleware
	Redis             *redis.Client
	AdminUsers        model.AdminUsersModel
	AdminAuth         model.AdminAuthModel
	Roles             model.RolesModel
	Merchants         model.MerchantsModel
	RiskAlerts        model.RiskAlertsModel
	GameConfigs       model.GameConfigsModel
	RiskBlacklist     model.RiskBlacklistModel
	Blacklist         *security.Blacklist
	Suspend           *security.SuspendStore
	IPWhitelist       *security.IPWhitelist
	Reporter          *clickhouse.Writer
	PendingTx         model.PendingTransactionsModel
	DailySettlements  model.DailySettlementsModel
	SettlementPeriods model.SettlementPeriodsModel
	I18n              model.I18nModel
	PlatformGames     model.PlatformGamesModel
	AuditLogs         model.AuditLogsModel
	UploadDir         string
	UploadPublicBase  string
	UploadMaxBytes    int64
	// Ledger 账变的唯一写入口（pkg/ledger.Poster）。后台也只能通过它记账，
	// 绝不直接写 game_transactions / player_accounts。
	Ledger *ledger.Poster
	// LedgerModel 账变流水的只读查询入口（后台流水列表用）。
	LedgerModel model.LedgerModel
	// DB 裸连接：账变模型的方法签名要求传入 sqlx.SqlConn（既能是普通连接，
	// 也能是事务内连接），后台的只读查询直接用它即可。
	DB sqlx.SqlConn
}

func NewServiceContext(c config.Config) (*ServiceContext, error) {
	fieldcipher.Init(c.Security.MerchantKeyCipher)

	conn := sqlx.NewMysql(c.MySQL.DataSource)
	reporter, err := clickhouse.NewWriterWithAuth(c.CH.Addr, c.CH.Database, c.CH.User, c.CH.Password)
	if err != nil {
		return nil, err
	}

	rdb := redis.NewClient(&redis.Options{Addr: c.Redis.Addr})
	blacklistModel := model.NewRiskBlacklistModel(conn)
	merchantsModel := model.NewMerchantsModel(conn)
	blacklist := security.NewBlacklist(rdb)

	uploadDir := c.Upload.Dir
	if uploadDir == "" {
		uploadDir = "web/admin/uploads"
	}
	if !filepath.IsAbs(uploadDir) {
		if cwd, err := os.Getwd(); err == nil {
			uploadDir = filepath.Join(cwd, uploadDir)
		}
	}
	_ = os.MkdirAll(uploadDir, 0o755)
	uploadBase := c.Upload.PublicBase
	if uploadBase == "" {
		uploadBase = "/admin/uploads"
	}
	uploadMax := c.Upload.MaxBytes
	if uploadMax <= 0 {
		uploadMax = 10 << 20
	}

	svcCtx := &ServiceContext{
		Config:            c,
		AuthMiddleware:    middleware.NewAuthMiddleware().Handle,
		Redis:             rdb,
		AdminUsers:        model.NewAdminUsersModel(conn),
		AdminAuth:         model.NewAdminAuthModel(conn),
		Roles:             model.NewRolesModel(conn),
		Merchants:         merchantsModel,
		RiskAlerts:        model.NewRiskAlertsModel(conn),
		GameConfigs:       model.NewGameConfigsModel(conn),
		RiskBlacklist:     blacklistModel,
		Blacklist:         blacklist,
		Suspend:           security.NewSuspendStore(rdb),
		IPWhitelist:       security.NewIPWhitelist(merchantsModel, rdb),
		Reporter:          reporter,
		PendingTx:         model.NewPendingTransactionsModel(conn),
		DailySettlements:  model.NewDailySettlementsModel(conn),
		SettlementPeriods: model.NewSettlementPeriodsModel(conn),
		I18n:              model.NewI18nModel(conn),
		PlatformGames:     model.NewPlatformGamesModel(conn),
		AuditLogs:         model.NewAuditLogsModel(conn),
		UploadDir:         uploadDir,
		UploadPublicBase:  uploadBase,
		UploadMaxBytes:    uploadMax,
		// sink 传 nil：admin-api 没跑 outbox dispatcher，账变事件（game.ledger.posted）
		// 暂时不投递。账变本身的持久化不依赖 sink，接上 dispatcher 时再换成
		// ledger.NewOutboxSink(...) 即可，Poster 的调用点不用改。
		Ledger:      ledger.NewPoster(conn, nil),
		LedgerModel: model.NewLedgerModel(),
		DB:          conn,
	}

	bootstrapBlacklist(rdb, blacklistModel)
	return svcCtx, nil
}

func bootstrapBlacklist(rdb *redis.Client, blacklistModel model.RiskBlacklistModel) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	items, err := blacklistModel.ListAllActive(ctx)
	if err != nil {
		logx.Errorf("admin bootstrap blacklist failed: %v", err)
		return
	}
	if err := security.NewBlacklist(rdb).SyncAll(ctx, items); err != nil {
		logx.Errorf("admin sync blacklist failed: %v", err)
	}
}
