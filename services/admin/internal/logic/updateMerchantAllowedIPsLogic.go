package logic

import (
	"context"
	"fmt"
	"net"
	"strings"

	"fastgame/services/admin/internal/svc"
	"fastgame/services/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateMerchantAllowedIPsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateMerchantAllowedIPsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateMerchantAllowedIPsLogic {
	return &UpdateMerchantAllowedIPsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateMerchantAllowedIPsLogic) UpdateMerchantAllowedIPs(req *types.UpdateMerchantAllowedIPsReq) (*types.MerchantAllowedIPsResp, error) {
	merchantCode, _, err := l.svcCtx.Merchants.FindAllowedIPsByID(l.ctx, req.Id)
	if err != nil {
		return nil, err
	}

	normalized, err := normalizeAllowedIPs(req.AllowedIps)
	if err != nil {
		return nil, err
	}

	if err := l.svcCtx.Merchants.UpdateAllowedIPs(l.ctx, req.Id, normalized); err != nil {
		return nil, err
	}

	if err := l.svcCtx.IPWhitelist.Invalidate(l.ctx, merchantCode); err != nil {
		l.Errorf("invalidate ip whitelist cache failed: merchant=%s err=%v", merchantCode, err)
	}

	return &types.MerchantAllowedIPsResp{
		MerchantId:   req.Id,
		MerchantCode: merchantCode,
		AllowedIps:   normalized,
	}, nil
}

func normalizeAllowedIPs(ips []string) ([]string, error) {
	if len(ips) == 0 {
		return []string{}, nil
	}

	seen := make(map[string]struct{}, len(ips))
	out := make([]string, 0, len(ips))
	for _, raw := range ips {
		ip := strings.TrimSpace(raw)
		if ip == "" {
			continue
		}
		if net.ParseIP(ip) == nil {
			return nil, fmt.Errorf("invalid ip address: %s", ip)
		}
		if _, ok := seen[ip]; ok {
			continue
		}
		seen[ip] = struct{}{}
		out = append(out, ip)
	}
	return out, nil
}
