package model

// round_id 的定位范围
//
// 迁移 33 之后，三张对账表的唯一键都以 merchant_id 前导：
//
//	pending_transactions  uk_merchant_round(merchant_id, round_id)
//	game_round_replay     uk_merchant_round(merchant_id, round_id)
//	wallet_pending_ops    uk_merchant_round_op(merchant_id, round_id, op_type)
//
// 即 round_id 只在商户内唯一。单靠 round_id 定位行有两个后果：
//
//  1. 正确性：会同时命中多个商户的数据。例如不同商户的两个客户端各自生成
//     roundId="abc" 时，`update ... where round_id='abc'` 会把两家商户的待对账
//     记录一起改成已结算。
//  2. 性能：原先支撑 round_id 单列查询的 uk_round_id 已被删除，只剩
//     (merchant_id, round_id) 这条复合唯一键，`where round_id = ?` 用不上它，
//     会退化成全表扫描——而 MarkSettled/MarkWinPending 在下注主链路上。
//
// 所以凡是按 round_id 定位行的读写都必须带上 merchant_id。merchantID == 0 表示
// 调用方确实拿不到商户（后台排障、公开回放接口），此时回退到 round_id 单列条件，
// 由迁移 34 补的 idx_round_id 支撑，且只应用于只读查询。
func roundScope(merchantID uint64, roundID string) (where string, args []any) {
	if merchantID == 0 {
		return "round_id = ?", []any{roundID}
	}
	return "merchant_id = ? and round_id = ?", []any{merchantID, roundID}
}
