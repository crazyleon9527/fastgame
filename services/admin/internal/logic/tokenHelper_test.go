package logic

import (
	"testing"

	"fastgame/internal/model"

	"github.com/golang-jwt/jwt/v4"
)

func TestIssueAdminTokenIncludesRoleName(t *testing.T) {
	user := &model.AdminAuthRecord{
		Id:          42,
		RoleId:      2,
		TotpEnabled: 1,
	}
	tokenStr, expireAt, err := issueAdminToken(user, "operator", "test-secret", 3600)
	if err != nil {
		t.Fatalf("issueAdminToken: %v", err)
	}
	if expireAt <= 0 {
		t.Fatal("expected positive expireAt")
	}

	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (any, error) {
		return []byte("test-secret"), nil
	})
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatal("expected map claims")
	}
	if claims["roleName"] != "operator" {
		t.Fatalf("roleName=%v want operator", claims["roleName"])
	}
	if claims["userId"].(float64) != 42 {
		t.Fatalf("userId=%v want 42", claims["userId"])
	}
	if claims["totpPending"].(bool) != false {
		t.Fatalf("totpPending=%v want false", claims["totpPending"])
	}
}

func TestIssueAdminTokenTotpPending(t *testing.T) {
	user := &model.AdminAuthRecord{Id: 1, RoleId: 1, TotpEnabled: 0}
	tokenStr, _, err := issueAdminToken(user, "admin", "secret", 3600)
	if err != nil {
		t.Fatal(err)
	}
	token, _ := jwt.Parse(tokenStr, func(token *jwt.Token) (any, error) {
		return []byte("secret"), nil
	})
	claims := token.Claims.(jwt.MapClaims)
	if claims["totpPending"].(bool) != true {
		t.Fatalf("totpPending=%v want true", claims["totpPending"])
	}
}
