package gameconfig

import (
	"context"
	"encoding/json"
	"fmt"

	"fastgame/internal/model"
)

const ConfigKeyRtpTier = "rtp_tier"

type Config struct {
	// MerchantID 是 merchantCode 对应的 merchants.id。
	// 对账类表（pending_transactions / game_round_replay）的唯一键以 merchant_id 前导，
	// 因此写入时必须带上它，否则会退化成 merchant_id=0，多商户下仍会互相覆盖。
	MerchantID uint64
	RtpTier    string
	Raw        map[string]json.RawMessage
}

type Loader struct {
	merchants    model.MerchantsModel
	gameConfigs  model.GameConfigsModel
}

func NewLoader(merchants model.MerchantsModel, gameConfigs model.GameConfigsModel) *Loader {
	return &Loader{
		merchants:   merchants,
		gameConfigs: gameConfigs,
	}
}

func (l *Loader) Load(ctx context.Context, merchantCode, gameCode string) (*Config, error) {
	merchant, err := l.merchants.FindOneByMerchantCode(ctx, merchantCode)
	if err != nil {
		return nil, fmt.Errorf("merchant %s: %w", merchantCode, err)
	}
	if merchant.Status != 1 {
		return nil, fmt.Errorf("merchant %s disabled", merchantCode)
	}

	configs, err := l.gameConfigs.FindActiveByMerchantGame(ctx, merchant.Id, gameCode)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		MerchantID: merchant.Id,
		RtpTier:    "default",
		Raw:        make(map[string]json.RawMessage),
	}

	for _, item := range configs {
		cfg.Raw[item.ConfigKey] = json.RawMessage(item.ConfigValue)
		if item.ConfigKey == ConfigKeyRtpTier && item.RtpTier.Valid {
			cfg.RtpTier = item.RtpTier.String
		}
	}

	return cfg, nil
}

// MerchantID 把商户编码解析为 merchants.id。
//
// 只按 round_id 查回放/对账记录是不完整的：迁移 33 之后这三张表的唯一键都以
// merchant_id 前导，round_id 只在商户内唯一。给只带 round_id 的接口（公开回放）
// 补上商户时，客户端给的是商户编码，这里负责换算成 merchants.id。
func (l *Loader) MerchantID(ctx context.Context, merchantCode string) (uint64, error) {
	merchant, err := l.merchants.FindOneByMerchantCode(ctx, merchantCode)
	if err != nil {
		return 0, fmt.Errorf("merchant %s: %w", merchantCode, err)
	}
	return merchant.Id, nil
}
