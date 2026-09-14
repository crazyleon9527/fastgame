package engine

import (
	"context"
	"fmt"
	"sync"
)

// GamePlugin 所有具体游戏必须实现的数学插件接口
// 核心要求：纯内存推演、微秒级响应、严禁任何网络与磁盘 I/O
type GamePlugin interface {
	GameCode() string
	CalculateOutcome(ctx context.Context, input *TurnInput) (*TurnOutcome, error)
}

var (
	pluginMu sync.RWMutex
	plugins  = make(map[string]GamePlugin)
)

// RegisterPlugin 游戏插件自注册
func RegisterPlugin(p GamePlugin) {
	pluginMu.Lock()
	defer pluginMu.Unlock()
	if p == nil {
		panic("engine: cannot register nil plugin")
	}
	code := p.GameCode()
	if _, exists := plugins[code]; exists {
		panic(fmt.Sprintf("engine: plugin %s already registered", code))
	}
	plugins[code] = p
}

// GetPlugin 检索指定游戏的数学插件
func GetPlugin(gameCode string) (GamePlugin, bool) {
	pluginMu.RLock()
	defer pluginMu.RUnlock()
	p, ok := plugins[gameCode]
	return p, ok
}
