package wallet

import (
	"net/http"
	"net/url"
	"testing"

	"fastgame/pkg/security"
)

func TestAttachSignHeaders(t *testing.T) {
	req, _ := http.NewRequest(http.MethodPost, "http://wallet/api/v1/wallet/bet", nil)
	req.URL = &url.URL{Path: "/api/v1/wallet/bet"}
	body := `{"merchantId":"m001","amount":10}`

	if err := attachSignHeaders(req, "test-secret", body); err != nil {
		t.Fatal(err)
	}

	ts := req.Header.Get(HeaderTimestamp)
	nonce := req.Header.Get(HeaderNonce)
	sig := req.Header.Get(HeaderSignature)
	payload := security.BuildSignPayload("POST", "/api/v1/wallet/bet", body, ts, nonce)
	if err := security.VerifySign("test-secret", payload, sig); err != nil {
		t.Fatalf("signature verify failed: %v", err)
	}
}
