package security

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"fastgame/internal/model"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestBlacklist(t *testing.T) (*Blacklist, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return NewBlacklist(rdb), mr
}

func activeRow(listType, value string) *model.RiskBlacklist {
	return &model.RiskBlacklist{ListType: listType, ListValue: value, Status: 1, Reason: "test"}
}

// TestBlacklistReconcileRemovesOrphanKeys —— 这是本次修的核心：
// 库里已经没有的封禁行，Redis 上的键必须被清掉。
//
// 真实事故形态：库里那行被手工删掉（或改成 status=0），而 SyncAll 只遍历
// 库里的行、只增不减，于是 Redis 上留下 TTL=-1 的永久封禁键——
// 被封的用户再也下不了注，后台却查不到任何依据。
func TestBlacklistReconcileRemovesOrphanKeys(t *testing.T) {
	bl, mr := newTestBlacklist(t)
	ctx := context.Background()

	// 制造孤儿键（库中不存在）
	if err := bl.Block(ctx, model.BlacklistTypeUserID, "10001", 0); err != nil {
		t.Fatal(err)
	}
	// 制造一个不相关的键，确认不会被误删
	if err := mr.Set("rtp:suspend:user:10001", "1"); err != nil {
		t.Fatal(err)
	}
	if err := mr.Set("merchant:allowips:m001", "[]"); err != nil {
		t.Fatal(err)
	}

	// 库里有一行有效封禁
	items := []*model.RiskBlacklist{activeRow(model.BlacklistTypeUserID, "20002")}
	if err := bl.Reconcile(ctx, items); err != nil {
		t.Fatalf("Reconcile 失败: %v", err)
	}

	if blocked, _ := bl.IsBlocked(ctx, model.BlacklistTypeUserID, "10001"); blocked {
		t.Error("库里不存在的封禁键没有被清理")
	}
	if blocked, _ := bl.IsBlocked(ctx, model.BlacklistTypeUserID, "20002"); !blocked {
		t.Error("库里的有效封禁没有同步到 Redis")
	}
	if !mr.Exists("rtp:suspend:user:10001") {
		t.Error("误删了非 blacklist: 前缀的键")
	}
	if !mr.Exists("merchant:allowips:m001") {
		t.Error("误删了非 blacklist: 前缀的键")
	}
}

// TestBlacklistReconcileUnblocksDisabledAndExpired —— status!=1 与已过期都应当解封。
func TestBlacklistReconcileUnblocksDisabledAndExpired(t *testing.T) {
	bl, _ := newTestBlacklist(t)
	ctx := context.Background()

	for _, v := range []string{"30001", "30002"} {
		if err := bl.Block(ctx, model.BlacklistTypeUserID, v, 0); err != nil {
			t.Fatal(err)
		}
	}

	expired := time.Now().Add(-time.Hour)
	items := []*model.RiskBlacklist{
		{ListType: model.BlacklistTypeUserID, ListValue: "30001", Status: 0},
		{ListType: model.BlacklistTypeUserID, ListValue: "30002", Status: 1,
			ExpiresAt: sql.NullTime{Time: expired, Valid: true}},
	}
	if err := bl.Reconcile(ctx, items); err != nil {
		t.Fatal(err)
	}

	for _, v := range []string{"30001", "30002"} {
		if blocked, _ := bl.IsBlocked(ctx, model.BlacklistTypeUserID, v); blocked {
			t.Errorf("user_id=%s 应当已解封（status=0 或已过期）", v)
		}
	}
}

// TestBlacklistReconcileKeepsFutureExpiry —— 未过期的封禁要保留，并沿用剩余 TTL。
func TestBlacklistReconcileKeepsFutureExpiry(t *testing.T) {
	bl, mr := newTestBlacklist(t)
	ctx := context.Background()

	future := time.Now().Add(2 * time.Hour)
	items := []*model.RiskBlacklist{{
		ListType: model.BlacklistTypeIP, ListValue: "10.0.0.9", Status: 1,
		ExpiresAt: sql.NullTime{Time: future, Valid: true},
	}}
	if err := bl.Reconcile(ctx, items); err != nil {
		t.Fatal(err)
	}

	if blocked, _ := bl.IsBlocked(ctx, model.BlacklistTypeIP, "10.0.0.9"); !blocked {
		t.Fatal("未过期的封禁不应被解封")
	}
	ttl := mr.TTL(blacklistKey(model.BlacklistTypeIP, "10.0.0.9"))
	if ttl <= 0 || ttl > 2*time.Hour {
		t.Errorf("TTL = %s，期望 (0, 2h]", ttl)
	}
}

// TestBlacklistReconcileEmptyDB —— 库为空时，所有孤儿键都应清掉（本次事故的形态）。
func TestBlacklistReconcileEmptyDB(t *testing.T) {
	bl, _ := newTestBlacklist(t)
	ctx := context.Background()

	_ = bl.Block(ctx, model.BlacklistTypeUserID, "10001", 0)
	_ = bl.Block(ctx, model.BlacklistTypeMerchant, "m001", 0)
	_ = bl.Block(ctx, model.BlacklistTypeIP, "127.0.0.1", 0)

	if err := bl.Reconcile(ctx, nil); err != nil {
		t.Fatal(err)
	}

	for _, c := range []struct{ listType, value string }{
		{model.BlacklistTypeUserID, "10001"},
		{model.BlacklistTypeMerchant, "m001"},
		{model.BlacklistTypeIP, "127.0.0.1"},
	} {
		if blocked, _ := bl.IsBlocked(ctx, c.listType, c.value); blocked {
			t.Errorf("库为空时 %s:%s 应被清理", c.listType, c.value)
		}
	}
}
