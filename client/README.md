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

## 生产构建与代码混淆

导出 Web Mobile 后，对游戏业务 JS 进行高强度混淆（控制流扁平化、变量替换、死代码注入）：

```bash
# 1. Cocos Creator: 项目 -> 构建 -> Web Mobile -> 构建
# 2. 混淆导出产物
cd client
npm install
chmod +x scripts/obfuscate-build.sh
./scripts/obfuscate-build.sh build/web-mobile
```

混淆目标：`assets/main/index.js`、`assets/internal/index.js`、`src/chunks/*.js`  
**不会**混淆 Cocos 引擎 (`cocos-js/`)，避免破坏 runtime。

配置见 `obfuscator.config.json`，也可通过 `make obfuscate-client` 执行。

## API 对接

| 接口 | 路径 |
|------|------|
| 查询余额 | `GET /api/v1/game/balance` |
| 抛竿/下注 | `POST /api/v1/game/bet` |
| 大奖广播 | `WS /ws/bigwin` |

## 动画映射（服务端驱动）

| fishState | animationKey | 表现 |
|-----------|--------------|------|
| miss | fish_miss | 空竿动画 |
| bite | fish_bite_normal | 普通咬钩 |
| big_win | fish_bite_bigwin | 大奖特效 |
