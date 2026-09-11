//go:build ignore

// 生成 operator / viewer 测试账号 SQL（密码均为 admin123）
// 用法: go run scripts/seed_rbac_users.go | docker exec -i fastgame-mysql mysql -ufastgame -pfastgame_pass fastgame
package main

import (
	"fmt"
	"os"

	"fastgame/pkg/auth"
)

func main() {
	hash, err := auth.HashPassword("admin123")
	if err != nil {
		panic(err)
	}
	fmt.Fprintf(os.Stdout, `USE fastgame;
INSERT INTO roles (name, description) VALUES
  ('admin', 'Super Admin - full access'),
  ('operator', 'Operations - no create merchant or rotate key'),
  ('viewer', 'Read Only - GET requests only')
ON DUPLICATE KEY UPDATE description = VALUES(description);

INSERT INTO admin_users (username, password_hash, role_id, status)
SELECT 'operator', '%s', r.id, 1 FROM roles r WHERE r.name = 'operator'
ON DUPLICATE KEY UPDATE password_hash = VALUES(password_hash), role_id = VALUES(role_id), status = 1;

INSERT INTO admin_users (username, password_hash, role_id, status)
SELECT 'viewer', '%s', r.id, 1 FROM roles r WHERE r.name = 'viewer'
ON DUPLICATE KEY UPDATE password_hash = VALUES(password_hash), role_id = VALUES(role_id), status = 1;
`, hash, hash)
}
