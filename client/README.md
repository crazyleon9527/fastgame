# FastGame Cocos Creator 3.8 客户端

纯表现层 H5 游戏客户端，**所有结算逻辑由 RGS 服务端判定**，本地只负责动画与 UI。

## 导入步骤

1. 用 **Cocos Creator 3.8.x** 新建 2D 空项目
2. 将本目录 `assets/scripts/` 复制到项目的 `assets/scripts/`
3. 创建场景 `Game`，挂载组件：
   - 根节点 → `FishingGameController`
   - HUD 节点 → `GameHud`（绑定余额/下注/赢额 Label 与抛竿 Button）
   - 跑马灯节点 → `BigWinMarquee`（绑定跑马灯 Label）
4. 在 `GameConfig.ts` 中确认 Gateway 地址（默认 `http://localhost:18000`）
5. 启动后端：`make up && make seed && make run-rgs && make run-broadcast`，Gateway 随 `make up` 启动
6. Cocos 预览运行

## 性能与加载优化

| 模块 | 路径 | 说明 |
|------|------|------|
| 对象池 | `assets/scripts/pool/` | 鱼群 / 金币 / 气泡 NodePool，禁止循环内 instantiate |
| Spine | `assets/scripts/animation/SpineFishController.ts` | 骨骼动画替代逐帧序列，包体 ↓70% |
| 分包 | `assets/scripts/asset/AssetBundleLoader.ts` | core <3MB 首屏，boss 后台预加载 |
| DrawCall | `assets/scripts/perf/DrawCallMonitor.ts` | 预算 ≤30，配合 TexturePacker 图集 |
| 图集规范 | `texturepacker/README.md` | 1~2 张 2048² Sprite Sheet |

Cocos Builder 分包模板见 `settings/builder-performance.template.json`。

场景挂载建议：

- 根节点 → `LoadingGate`（首屏 core 分包）→ 完成后显示 `FishingGameController`
- `FishingGameController` 绑定 fish/coin/bubble Prefab、`SpineFishController`、`DrawCallMonitor`

## 生产构建、混淆与 Brotli

```bash
# 1. Cocos Creator: 构建 -> Web Mobile
# 2. 一键部署到 Gateway（混淆 + Brotli 预压缩 + 拷贝到 web/game）
make deploy-client
# 3. 重建带 Brotli 模块的 Gateway
docker compose build gateway && docker compose up -d gateway
```

访问：`http://localhost:18000/game/`

单独步骤：

```bash
cd client && ./scripts/obfuscate-build.sh build/web-mobile
cd client && ./scripts/compress-brotli.sh build/web-mobile
```

Nginx 开启 `brotli` + `brotli_static`，较 gzip 额外节省 15%~25% 传输体积。

配置见 `obfuscator.config.json`，也可通过 `make obfuscate-client` 执行。

## API 对接

| 接口 | 路径 |
|------|------|
| 查询余额 | `GET /api/v1/game/balance` |
| 抛竿/下注 | `POST /api/v1/game/bet` |
| 大奖广播 | `WS /ws/bigwin` |

## 确定性回放 (Deterministic Replay)

每局仅持久化 **种子 + 基础输入**（`serverSeed`, `clientSeed`, `nonce/roundId`, `betAmount`），
天气、鱼群轨迹、咬钩道具、派彩均由 PRNG 确定性派生：

```typescript
// 从历史 roundId 100% 复现
await fishingController.replayRound('round-xxx');
```

- 浏览器回放：`http://localhost:18000/replay/?roundId=round-xxx`
- API：`GET /api/v1/game/replay/:roundId`

## 动画映射（服务端驱动）

| fishState | animationKey | 表现 |
|-----------|--------------|------|
| miss | fish_miss | 空竿动画 |
| bite | fish_bite_normal | 普通咬钩 |
| big_win | fish_bite_bigwin | 大奖特效 |
