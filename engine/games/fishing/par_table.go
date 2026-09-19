package fishing

import "fastgame/pkg/par"

// FishDef 是赔付表的一行，直接复用 pkg/par 的定义。
// 之所以是别名而不是另开一份结构体：这张表有三个使用方
// （pkg/par 采样、本插件推演、web/shared/prng-money.js 镜像），
// 复制一份迟早会漂移，而漂移的后果是客户端验算算不出服务端的派彩。
type FishDef = par.Entry

// TargetRTP 是本表标注的理论 RTP（百分比口径，仅用于日志/自检）。
var TargetRTP = par.Default96.RTP() * 100

// DefaultPARTable96 与 pkg/par 共用同一份表数据。
var DefaultPARTable96 = par.Default96.Entries()
