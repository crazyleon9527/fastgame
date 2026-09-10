package security

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWAFBlocksSQLMap(t *testing.T) {
	waf := NewWAF()
	req := httptest.NewRequest("POST", "/api/v1/game/bet", strings.NewReader(`{"betAmount":10}`))
	req.Header.Set("User-Agent", "sqlmap/1.0")
	if err := waf.InspectRequest(req, `{"betAmount":10}`); err == nil {
		t.Fatal("expected blocked user agent")
	}
}

func TestWAFBlocksInjectionInBody(t *testing.T) {
	waf := NewWAF()
	body := `{"betAmount":10,"note":"union select * from users"}`
	req := httptest.NewRequest("POST", "/api/v1/game/bet", strings.NewReader(body))
	if err := waf.InspectRequest(req, body); err == nil {
		t.Fatal("expected suspicious body block")
	}
}
