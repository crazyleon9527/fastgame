package engine

import (
	"fastgame/pkg/money"
)

// TurnInput 客户端单次操作的标准输入
type TurnInput struct {
	TraceID      string       `json:"trace_id"`
	RoundID      string       `json:"round_id"`
	MerchantID   string       `json:"merchant_id"`
	MerchantCode string       `json:"merchant_code"`
	UserID       string       `json:"user_id"`
	GameCode     string       `json:"game_code"`
	Currency     string       `json:"currency"`
	BetAmount    money.Amount `json:"bet_amount"`
	IsDemo       bool         `json:"is_demo"`

	// 客户端自定义参数 (例如: 钓鱼的鱼竿等级、抛竿力度，生肖的选线等)
	ExtraParams map[string]any `json:"extra_params"`
}

// TurnOutcome 数学引擎推演出的标准结果
type TurnOutcome struct {
	WinAmount        money.Amount `json:"win_amount"`        // 本次派彩总额 (0表示未中)
	PayoutMultiplier float64      `json:"payout_multiplier"` // 实际倍率 (win / bet)
	MathVersion      string       `json:"math_version"`      // 本次计算所采用的 PAR 版本
	RtpApplied       float64      `json:"rtp_applied"`       // 标称理论 RTP (如 96.00)

	// 可验证公平性 (Provably Fair) 证据链
	ServerSeed     string `json:"server_seed"`
	ServerSeedHash string `json:"server_seed_hash"`
	ClientSeed     string `json:"client_seed"`
	Nonce          uint64 `json:"nonce"`

	// 纯前端视觉演播 JSON (透传给 Cocos，底层无需理解具体字段)
	PresentationPayload string `json:"presentation_payload"`
}

// TurnResult 管道执行完毕返回给前端的最终结构
type TurnResult struct {
	RoundID             string       `json:"round_id"`
	Balance             money.Amount `json:"balance"` // 账变后最新余额
	WinAmount           money.Amount `json:"win_amount"`
	PayoutMultiplier    float64      `json:"payout_multiplier"`
	PresentationPayload string       `json:"presentation_payload"`
}
