package rtpwatchdog

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// RedisWatchdog 基于 Redis 滑动窗口的多租户 RTP 监控器
type RedisWatchdog struct {
	rdb    *redis.Client
	cfg    Config
	prefix string
}

func NewRedis(rdb *redis.Client, cfg Config) *RedisWatchdog {
	def := DefaultConfig()
	if cfg.GlobalMax <= 0 {
		cfg.GlobalMax = def.GlobalMax
	}
	if cfg.MinSamples <= 0 {
		cfg.MinSamples = def.MinSamples
	}
	if cfg.AlertCooldown <= 0 {
		cfg.AlertCooldown = def.AlertCooldown
	}
	if cfg.ThresholdPPM <= 0 {
		cfg.ThresholdPPM = def.ThresholdPPM
	}
	if cfg.MinSampleBet <= 0 {
		cfg.MinSampleBet = def.MinSampleBet
	}
	if cfg.PlayerMax < cfg.MinSamples {
		cfg.PlayerMax = cfg.MinSamples
	}
	if cfg.GlobalMax < cfg.MinSamples {
		cfg.GlobalMax = cfg.MinSamples
	}
	return &RedisWatchdog{
		rdb:    rdb,
		cfg:    cfg,
		prefix: "rtp:watch",
	}
}

// evalLua 原子脚本:
// 1. LPUSH 压入样本并 LTRIM 修剪长度
// 2. 判断是否存在冷却 key，若存在直接返回 0 跳过计算
// 3. 校验队列长度，未达 minSamples 时直接返回 0，杜绝高频全量求和
// 4. 样本量达标且不在冷却中，在 Redis 内部遍历累加返回 {totalBet, totalWin, sampleSize}
var evalLua = redis.NewScript(`
local listKey = KEYS[1]
local alertKey = KEYS[2]
local sample = ARGV[1]
local maxLen = tonumber(ARGV[2])
local minSamples = tonumber(ARGV[3])

-- 1. 压入样本并修剪
redis.call('LPUSH', listKey, sample)
redis.call('LTRIM', listKey, 0, maxLen - 1)

-- 2. 冷却中直接退出
if redis.call('EXISTS', alertKey) == 1 then
    return {0, 0, 0}
end

-- 3. 样本量未达下限直接退出
local currentLen = redis.call('LLEN', listKey)
if currentLen < minSamples then
    return {0, 0, 0}
end

-- 4. 在 Redis 内存遍历求和，避免网络传输大包
local items = redis.call('LRANGE', listKey, 0, -1)
local totalBet = 0
local totalWin = 0

for _, item in ipairs(items) do
    local sep = string.find(item, "|")
    if sep then
        local b = tonumber(string.sub(item, 1, sep - 1)) or 0
        local w = tonumber(string.sub(item, sep + 1)) or 0
        totalBet = totalBet + b
        totalWin = totalWin + w
    end
end

return {totalBet, totalWin, currentLen}
`)

func (w *RedisWatchdog) Record(ctx context.Context, in RecordInput) ([]Alert, error) {
	if in.BetMinor <= 0 || in.UserID == "" || in.MerchantCode == "" {
		return nil, nil
	}
	sample := fmt.Sprintf("%d|%d", in.BetMinor, in.WinMinor)

	// 多租户隔离：玩家维度与游戏维度均强制绑定 MerchantCode
	userKey := fmt.Sprintf("%s:user:%s:%s", w.prefix, in.MerchantCode, in.UserID)
	gameKey := fmt.Sprintf("%s:game:%s:%s", w.prefix, in.MerchantCode, in.GameCode)

	var alerts []Alert

	// 1. 评估玩家维度
	if alert, ok, err := w.runEval(ctx, userKey, "user", in.UserID, in, sample, w.cfg.PlayerMax); err != nil {
		return nil, err
	} else if ok {
		alerts = append(alerts, alert)
	}

	// 2. 评估游戏维度
	gameScope := fmt.Sprintf("%s:%s", in.MerchantCode, in.GameCode)
	if alert, ok, err := w.runEval(ctx, gameKey, "game", gameScope, in, sample, w.cfg.GlobalMax); err != nil {
		return nil, err
	} else if ok {
		alerts = append(alerts, alert)
	}

	return alerts, nil
}

func (w *RedisWatchdog) runEval(
	ctx context.Context,
	key, scopeType, scopeValue string,
	in RecordInput,
	sample string,
	maxLen int,
) (Alert, bool, error) {
	alertedKey := fmt.Sprintf("rtp:alerted:%s", key)

	res, err := evalLua.Run(ctx, w.rdb, []string{key, alertedKey}, sample, maxLen, w.cfg.MinSamples).Slice()
	if err != nil {
		return Alert{}, false, err
	}

	totalBet := res[0].(int64)
	totalWin := res[1].(int64)
	sampleSize := int(res[2].(int64))

	// 样本未满、投注额不足或处于冷却期
	if sampleSize < w.cfg.MinSamples || totalBet < w.cfg.MinSampleBet {
		return Alert{}, false, nil
	}

	rtpPPM := totalWin * 1_000_000 / totalBet
	if rtpPPM <= w.cfg.ThresholdPPM {
		return Alert{}, false, nil
	}

	// 触发漂移：打上冷却标记（TTL 到期后重新开始评估）
	if err := w.rdb.Set(ctx, alertedKey, "1", w.cfg.AlertCooldown).Err(); err != nil {
		return Alert{}, false, err
	}

	return Alert{
		ScopeType:    scopeType,
		ScopeValue:   scopeValue,
		MerchantCode: in.MerchantCode,
		GameCode:     in.GameCode,
		UserID:       in.UserID,
		RtpPPM:       rtpPPM,
		TotalBet:     totalBet,
		TotalWin:     totalWin,
		SampleSize:   sampleSize,
	}, true, nil
}
