USE fastgame;

-- RBAC 三角色：超管 / 运维 / 只读
INSERT INTO roles (name, description) VALUES
  ('admin', 'Super Admin - full access'),
  ('operator', 'Operations - no create merchant or rotate key'),
  ('viewer', 'Read Only - GET requests only')
ON DUPLICATE KEY UPDATE description = VALUES(description);
