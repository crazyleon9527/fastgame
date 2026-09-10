package gameconfig

import "context"

// Provider 供 RGS 加载商户游戏配置（便于测试注入 stub）
type Provider interface {
	Load(ctx context.Context, merchantCode, gameCode string) (*Config, error)
}
