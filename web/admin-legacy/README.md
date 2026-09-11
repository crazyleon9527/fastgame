# FastGame Admin

静态运营管理后台，通过 Gateway 的 sponge 域名访问。

## 访问地址

```
http://localhost:18000/admin/
```

或（推荐 hosts 配置后）：

```
http://sponge.localhost:18000/admin/
```

## 默认账号

| 用户名 | 密码 | 角色 |
|--------|------|------|
| admin | admin123 | 超管 |
| operator | admin123 | 运维 |
| viewer | admin123 | 只读 |

首次登录需完成 2FA 绑定。账号需先执行 `make seed-admin` 和 `make seed-rbac-users`。

## 功能模块

- **风控黑名单** — 查看/添加/解除封禁
- **商户管理** — 新建商户、编辑、密钥轮换、钱包熔断
- **游戏配置** — RTP 档位、下注限额等 JSON 配置
- **RTP 报表** — ClickHouse 按小时聚合对账
- **对账看板** — 每日对账单同步、确认锁定
- **IP 白名单** — 商户聚合器 IP 报备
- **Trace 追踪** — 全链路 I/O 排查
- **RTP 告警** — 看门狗告警确认
- **用户管理** — 超管专属，创建/编辑管理员账号
- **2FA** — Google Authenticator 绑定

## 角色权限

| 能力 | 超管 | 运维 | 只读 |
|------|:----:|:----:|:----:|
| 查看所有模块 | ✓ | ✓ | ✓ |
| 写操作（黑名单/配置等） | ✓ | ✓ | ✗ |
| 新建商户 / 轮换密钥 | ✓ | ✗ | ✗ |
| 用户管理 | ✓ | ✗ | ✗ |

## 依赖

- Admin API 运行在 `:18889`
- Gateway 挂载 `web/admin` 静态目录
- 完整启动流程见 [docs/DEV.md](../../docs/DEV.md)

```powershell
docker compose up -d
go run scripts/seed_admin.go | docker exec -i fastgame-mysql mysql -ufastgame -pfastgame_pass fastgame
.\scripts\dev-start.ps1
```
