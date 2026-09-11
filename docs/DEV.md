# FastGame 本地开发指南

## 环境要求

- Docker Desktop (Windows/macOS) 或 Docker + Compose (Linux)
- Go 1.26+
- Node.js 18+（仅 Cocos 客户端构建时需要）

## 首次启动（10 步）

```powershell
# 1. 克隆后进入项目目录
cd fastgame

# 2. 复制环境变量（端口与 services/*/etc/*.yaml 对齐）
Copy-Item .env.example .env

# 3. 启动基础设施（MySQL / Redis / Kafka / ClickHouse / Gateway）
docker compose up -d

# 4. 等待 MySQL healthy 后导入种子数据
go run scripts/seed_admin.go | docker exec -i fastgame-mysql mysql -ufastgame -pfastgame_pass fastgame
docker exec -i fastgame-mysql mysql -ufastgame -pfastgame_pass fastgame < docker/mysql/init/02-seed.sql

# 5. RBAC 角色 + 测试账号（admin / operator / viewer，密码均为 admin123）
make seed-rbac-users
# Windows 无 make 时:
go run scripts/seed_rbac_users.go | docker exec -i fastgame-mysql mysql -ufastgame -pfastgame_pass fastgame

# 6. 初始化 Kafka Topic + ClickHouse 表
docker compose run --rm kafka-init
docker compose run --rm clickhouse-init

# 7. 编译并启动全部 Go 微服务
.\scripts\dev-start.ps1

# 8. 访问管理后台
#    http://localhost:18000/admin/  或  http://sponge.localhost:18000/admin/

# 9. 登录 admin / admin123，完成 2FA 绑定

# 10. 验证 Kafka→ClickHouse 管道（consumer 必须在运行）
#     有注单数据后可在「RTP 报表」Tab 查看
```

## 服务端口

| 服务 | 端口 | 说明 |
|------|------|------|
| Gateway | 18000 | 统一入口 |
| MySQL | 13306 | 需 `.env` |
| Redis | 16379 | 需 `.env` |
| Kafka | 19092 | 需 `.env` |
| ClickHouse HTTP | 18123 | 需 `.env` |
| RGS API | 18888 | 宿主机进程 |
| Admin API | 18889 | 宿主机进程 |
| Broadcast WS | 18900 | 宿主机进程 |
| Kafka UI | 18080 | 开发辅助（需 `--profile dev-tools`） |
| Adminer | 18081 | 开发辅助（需 `--profile dev-tools`） |
| RedisInsight | 15540 | 开发辅助（需 `--profile dev-tools`） |

## 管理后台角色

| 角色 | 用户名 | 权限 |
|------|--------|------|
| 超管 | admin | 全部 + 用户管理 |
| 运维 | operator | 日常运维，不可新建商户/轮换密钥 |
| 只读 | viewer | 仅查看 |

## 开发辅助工具

Dev 工具（Adminer / Kafka UI / RedisInsight）默认不启动，且绑定 `127.0.0.1` 仅本机可访问：

```powershell
docker compose --profile dev-tools up -d
```

## 游戏客户端部署

Cocos Creator 构建 Web Mobile 后：

```powershell
.\scripts\deploy-client.ps1
# 访问 http://localhost:18000/game/
```

## Windows 命令（替代 Makefile）

```powershell
.\scripts\make.ps1 help          # 查看全部 target
.\scripts\make.ps1 up            # 启动 Docker 基础设施
.\scripts\make.ps1 build         # 编译全部 Go 服务
.\scripts\make.ps1 dev-start     # 编译并启动 5 个服务
.\scripts\make.ps1 seed-admin      # 创建 admin 账号
.\scripts\make.ps1 dev-tools       # 启动 Adminer/Kafka UI
```

## 数据库迁移

完整表设计见 [docs/DATABASE.md](DATABASE.md)。

```powershell
.\scripts\apply-migration.ps1 11-platform-games   # 应用指定迁移
.\scripts\apply-migration.ps1 all                  # 应用全部迁移
.\scripts\make.ps1 migrate-11
```

每 10 分钟自动演进 Schema（后台循环，Ctrl+C 停止）：

```powershell
.\scripts\db-schema-loop.ps1
```

## 常用命令

```powershell
docker compose ps                          # 查看基础设施状态
docker compose restart gateway               # 重启 Nginx 网关
docker compose logs gateway --tail 30        # 网关日志
go build -o bin/admin-api.exe ./services/admin && .\bin\admin-api.exe -f services\admin\etc\admin-api.yaml
```

## 生产配置

各服务提供 `etc/*.prod.yaml` 模板，部署前替换 `CHANGE_ME` 占位符：

- `services/admin/etc/admin-api.prod.yaml`
- `services/rgs/etc/rgs-api.prod.yaml`
- `services/consumer/etc/consumer.prod.yaml`
- `services/rollback/etc/rollback.prod.yaml`
- `services/broadcast/etc/broadcast.prod.yaml`

## 注意事项

- Go 微服务当前运行在**宿主机**，Gateway 通过 `host.docker.internal` 转发
- RGS 开发配置启用了 `SkipMerchantSign` 和 `Wallet.Mock`，**不可用于生产**
- 旧 JWT（无 roleName）已失效，升级后需重新登录
- 商户私钥在 MySQL 中以 AES-GCM 加密存储（`Security.MerchantKeyCipher`），RGS 与 Admin 须使用相同密钥
- 冲正/孤儿单重试 10 次失败后会写入 Kafka `game.reconcile.dlq`
- Gateway WAF 不再拦截空 User-Agent（兼容 curl / 部分 SDK）
