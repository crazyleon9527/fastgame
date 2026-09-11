package auth

import (
	"net/http"
	"testing"
)

func TestCanAccess(t *testing.T) {
	cases := []struct {
		role   string
		method string
		path   string
		want   bool
	}{
		{RoleViewer, http.MethodGet, "/api/v1/admin/merchants", true},
		{RoleViewer, http.MethodPost, "/api/v1/admin/risk-blacklist", false},
		{RoleOperator, http.MethodPost, "/api/v1/admin/risk-blacklist", true},
		{RoleOperator, http.MethodPost, "/api/v1/admin/merchants", false},
		{RoleOperator, http.MethodPost, "/api/v1/admin/merchants/1/rotate-key", false},
		{RoleOperator, http.MethodPut, "/api/v1/admin/merchants/1/allowed-ips", true},
		{RoleAdmin, http.MethodPost, "/api/v1/admin/merchants", true},
		{RoleViewer, http.MethodPost, "/api/v1/admin/totp/confirm", true},
		{RoleOperator, http.MethodGet, "/api/v1/admin/users", false},
		{RoleAdmin, http.MethodPost, "/api/v1/admin/users", true},
		{RoleViewer, http.MethodGet, "/api/v1/admin/reports/daily-settlements", true},
		{RoleViewer, http.MethodPost, "/api/v1/admin/reports/daily-settlements/sync", false},
		{RoleOperator, http.MethodPost, "/api/v1/admin/reports/daily-settlements/sync", true},
		{RoleOperator, http.MethodPost, "/api/v1/admin/reports/daily-settlements/1/confirm", true},
	}
	for _, c := range cases {
		got := CanAccess(c.role, c.method, c.path)
		if got != c.want {
			t.Fatalf("CanAccess(%q,%q,%q)=%v want %v", c.role, c.method, c.path, got, c.want)
		}
	}
}
