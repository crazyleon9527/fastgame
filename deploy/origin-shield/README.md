# 源站隐匿 (Origin Shielding)

游戏核心后端 **不得暴露公网 IP**。生产架构：

```
玩家 → Cloudflare (WAF / DDoS) → Nginx Gateway (80/443) → 127.0.0.1:RGS/Admin
```

## 部署步骤

### 1. Cloudflare 前置

- 域名接入 Cloudflare Enterprise 或 Magic Transit
- 开启 **WAF**、Bot Fight Mode、Rate Limiting
- SSL 模式：**Full (Strict)**
- 源站仅暴露 Nginx 80/443，**禁止** RGS (:18888) / Admin (:18889) 绑定公网

### 2. 更新 Cloudflare IP 列表

```bash
chmod +x deploy/origin-shield/*.sh
./deploy/origin-shield/fetch-cloudflare-ips.sh
```

### 3. Nginx 生产配置

`docker-compose.yml` 中将 origin-shield 挂载为生产版：

```yaml
- ./docker/nginx/origin-shield.prod.conf:/etc/nginx/origin-shield.conf:ro
- ./docker/nginx/cloudflare-realip.conf:/etc/nginx/cloudflare-realip.conf:ro
```

### 4. 主机防火墙（二选一）

```bash
# UFW
sudo ./deploy/origin-shield/apply-ufw.sh

# 或 iptables
sudo ./deploy/origin-shield/apply-iptables.sh
```

仅 Cloudflare 回源 IP 可访问 80/443，其他入站全部拒绝。

### 5. 后端仅监听 localhost

生产使用 `services/rgs/etc/rgs-api.prod.yaml`：

```yaml
Host: 127.0.0.1
Port: 18888
```

Admin 同理绑定 `127.0.0.1:18889`。

## 开发环境

默认 `origin-shield.conf` 允许所有来源，便于本地 `localhost:18000` 调试。
