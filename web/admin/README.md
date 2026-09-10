# FastGame Admin — 风控黑名单

静态管理页面，通过 Gateway 访问。

## 访问地址

```
http://localhost:18000/admin/
```

## 默认账号

- 用户名: `admin`
- 密码: `admin123`（需先执行 `make seed-admin`）

## 功能

- 查看 / 筛选黑名单（IP、用户 ID、商户）
- 添加封禁（支持设置过期时间）
- 解除封禁

## 依赖

- Admin API 运行在 `:18889`
- Gateway 挂载 `web/admin` 静态目录

```bash
make migrate-security   # 首次部署
make seed-admin
make run-admin
docker compose restart gateway
```
