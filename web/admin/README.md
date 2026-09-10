# FastGame Admin

静态运营管理页面，通过 Gateway 访问。

## 访问地址

```
http://localhost:18000/admin/
```

## 默认账号

- 用户名: `admin`
- 密码: `admin123`（需先执行 `make seed-admin`）

## 功能

### 风控黑名单
- 查看 / 筛选黑名单（IP、用户 ID、商户）
- 添加封禁（支持设置过期时间）
- 解除封禁

### 商户密钥轮换
- 查看商户列表
- 轮换 API 私钥（可配置 1–168 小时过渡期）
- 新密钥仅展示一次，支持一键复制

## 依赖

- Admin API 运行在 `:18889`
- Gateway 挂载 `web/admin` 静态目录

```bash
make migrate-security   # 首次部署
make seed-admin
make run-admin
docker compose restart gateway
```
