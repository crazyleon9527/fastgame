package model

import (
	"context"
	"database/sql"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// StubMerchantsModel 供集成测试注入商户密钥（非生产路径）
type StubMerchantsModel struct {
	Secrets *MerchantSecrets
}

func (s StubMerchantsModel) FindSecretsByMerchantCode(_ context.Context, _ string) (*MerchantSecrets, error) {
	if s.Secrets == nil {
		return nil, ErrNotFound
	}
	return s.Secrets, nil
}

func (s StubMerchantsModel) FindAllowedIPs(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}

func (s StubMerchantsModel) FindAllowedIPsByID(_ context.Context, _ uint64) (string, []string, error) {
	return "", nil, nil
}

func (s StubMerchantsModel) UpdateAllowedIPs(_ context.Context, _ uint64, _ []string) error {
	return nil
}

func (s StubMerchantsModel) RotatePrivateKey(_ context.Context, _ uint64, _ string, _ time.Duration) error {
	return nil
}

func (s StubMerchantsModel) ListPage(_ context.Context, _, _ int) ([]*Merchants, int64, error) {
	return nil, 0, nil
}

func (s StubMerchantsModel) withSession(_ sqlx.Session) MerchantsModel { return s }

func (s StubMerchantsModel) Insert(_ context.Context, _ *Merchants) (sql.Result, error) {
	return nil, nil
}

func (s StubMerchantsModel) FindOne(_ context.Context, _ uint64) (*Merchants, error) {
	return nil, ErrNotFound
}

func (s StubMerchantsModel) FindOneByMerchantCode(_ context.Context, code string) (*Merchants, error) {
	return &Merchants{MerchantCode: code, Status: 1}, nil
}

func (s StubMerchantsModel) Update(_ context.Context, _ *Merchants) error { return nil }

func (s StubMerchantsModel) Delete(_ context.Context, _ uint64) error { return nil }
