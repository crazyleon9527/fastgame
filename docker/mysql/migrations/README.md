# MySQL 增量迁移

## 命名

```
{序号}-{主题}-migration.sql
```

例：`11-platform-games-migration.sql`

## 应用（已有数据库）

```powershell
.\scripts\apply-migration.ps1 11-platform-games
# 或
.\scripts\apply-migration.ps1 all
```

## 新环境

`docker/mysql/init/` 下文件在 MySQL 首次启动时按字母序自动执行。

## 演进队列

见 `schema_backlog.json`。每完成一项迁移，将 version 移入 `completed` 并递增 `next_migration`。

定时任务 `scripts/db-schema-loop.ps1` 每 10 分钟触发 Agent 继续处理 backlog。
