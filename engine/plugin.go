package engine

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
)

// GamePlugin 所有具体游戏必须实现的数学插件接口
// 核心要求：纯内存推演、微秒级响应、严禁任何网络与磁盘 I/O
type GamePlugin interface {
	GameCode() string
	CalculateOutcome(ctx context.Context, input *TurnInput) (*TurnOutcome, error)
}

var (
	pluginMu      sync.RWMutex
	plugins       = make(map[string]GamePlugin)
	pluginsAtomic atomic.Value // 存储 map[string]GamePlugin 快照，实现微秒级零锁查找
)

func init() {
	pluginsAtomic.Store(make(map[string]GamePlugin))
}

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

	// 更新只读原子快照
	snapshot := make(map[string]GamePlugin, len(plugins))
	for k, v := range plugins {
		snapshot[k] = v
	}
	pluginsAtomic.Store(snapshot)
}

// GetPlugin 检索指定游戏的数学插件 (完全无锁热路径)
func GetPlugin(gameCode string) (GamePlugin, bool) {
	m := pluginsAtomic.Load().(map[string]GamePlugin)
	p, ok := m[gameCode]
	return p, ok
}

// ListPlugins 列出所有已加载的游戏插件代码 (供健康检查/路由探活)
func ListPlugins() []string {
	m := pluginsAtomic.Load().(map[string]GamePlugin)
	list := make([]string, 0, len(m))
	for code := range m {
		list = append(list, code)
	}
	return list
}
