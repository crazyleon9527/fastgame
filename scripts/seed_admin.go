//go:build ignore

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
	fmt.Fprintf(os.Stdout, `-- admin seed (password: admin123)
USE fastgame;
INSERT INTO roles (name, description) VALUES ('admin', 'Super Admin')
ON DUPLICATE KEY UPDATE description = VALUES(description);
INSERT INTO admin_users (username, password_hash, role_id, status)
SELECT 'admin', '%s', r.id, 1 FROM roles r WHERE r.name = 'admin'
ON DUPLICATE KEY UPDATE password_hash = VALUES(password_hash), status = 1;
`, hash)
}
