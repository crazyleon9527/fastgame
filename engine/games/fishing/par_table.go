package fishing

import "fastgame/pkg/par"

// FishDef 是赔付表的一行，直接复用 pkg/par 的定义。
// 之所以是别名而不是另开一份结构体：这张表有三个使用方
// （pkg/par 采样、本插件推演、web/shared/prng-money.js 镜像），
// 复制一份迟早会漂移，而漂移的后果是客户端验算算不出服务端的派彩。
type FishDef = par.Entry

var (
	// DefaultPARTable96 与 pkg/par 共享 96% 档位数据[cite: 40]
	DefaultPARTable96 = par.Default96.Entries()

	// DefaultPARTable94 共享 94% 档位数据
	DefaultPARTable94 = par.Default94.Entries()
)
