package gameconfig

import "context"

// Provider 供 RGS 加载商户游戏配置（便于测试注入 stub）
type Provider interface {
	Load(ctx context.Context, merchantCode, gameCode string) (*Config, error)
	// MerchantID 把商户编码解析为 merchants.id。
	// 回放/验算这类只给了 round_id 的接口需要它来把查询限定在商户内
	// （round_id 只在商户内唯一）。
	MerchantID(ctx context.Context, merchantCode string) (uint64, error)
}
