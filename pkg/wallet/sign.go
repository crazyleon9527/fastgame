package wallet

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"fastgame/pkg/security"
)

const (
	HeaderTimestamp = security.HeaderTimestamp
	HeaderNonce     = security.HeaderNonce
	HeaderSignature = security.HeaderSignature
)

func attachSignHeaders(req *http.Request, secret, body string) error {
	if secret == "" {
		return fmt.Errorf("wallet sign secret missing")
	}
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	nonce, err := randomNonce()
	if err != nil {
		return err
	}
	payload := security.BuildSignPayload(req.Method, req.URL.Path, body, ts, nonce)
	sig := security.Sign(secret, payload)
	req.Header.Set(HeaderTimestamp, ts)
	req.Header.Set(HeaderNonce, nonce)
	req.Header.Set(HeaderSignature, sig)
	return nil
}

func verifyResponseSign(secret string, res *http.Response, body []byte) error {
	if secret == "" || res.Header.Get(HeaderSignature) == "" {
		return nil
	}
	ts := res.Header.Get(HeaderTimestamp)
	nonce := res.Header.Get(HeaderNonce)
	sig := res.Header.Get(HeaderSignature)
	payload := security.BuildSignPayload(http.MethodPost, res.Request.URL.Path, string(body), ts, nonce)
	return security.VerifySign(secret, payload, sig)
}

func randomNonce() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
