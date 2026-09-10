package gameconfig

import (
	"context"
	"encoding/json"
	"fmt"

	"fastgame/internal/model"
)

const ConfigKeyRtpTier = "rtp_tier"

type Config struct {
	RtpTier string
	Raw     map[string]json.RawMessage
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
		RtpTier: "default",
		Raw:     make(map[string]json.RawMessage),
	}

	for _, item := range configs {
		cfg.Raw[item.ConfigKey] = json.RawMessage(item.ConfigValue)
		if item.ConfigKey == ConfigKeyRtpTier && item.RtpTier.Valid {
			cfg.RtpTier = item.RtpTier.String
		}
	}

	return cfg, nil
}
