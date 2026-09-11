# FastGame Admin

基于 [vue-pure-admin](https://github.com/pure-admin/vue-pure-admin)（pure-admin-thin）构建，由 Nginx 挂载到 `/admin/`。

- **源码**：`web/admin-vue/`
- **旧版备份**：`web/admin-legacy/`

## 构建部署

```powershell
.\scripts\deploy-admin.ps1
```

## 开发

```powershell
cd web/admin-vue
npm install
npm run dev
# http://localhost:8848  (API 代理到 :18000)
```

## 登录

| 用户 | 密码 | 角色 |
|------|------|------|
| admin | admin123 | 超管 |
| operator | admin123 | 运维 |
| viewer | admin123 | 只读 |

访问：**http://localhost:18000/admin/**
