# FishingMinimal — 最小可玩场景

在 Cocos Creator 3.8 中验证对象池 / Spine / DrawCall 框架。

## 1. 新建场景

1. 创建场景 `FishingMinimal`（保存到 `assets/scenes/FishingMinimal.scene`）
2. 根节点挂载 `LoadingGate`（core bundle 加载完成后隐藏 Loading）
3. Canvas 下创建：
   - `Hud` → 挂 `GameHud`
   - `Rod` → 挂 `Animation`（可选）
   - `FishLayer` → 空节点，供 `FishPool` 使用
   - `VfxLayer` → 空节点，供 `CoinBurstPool` / `BubblePool`
   - `Game` → 挂 `FishingGameController`

## 2. 绑定 FishingGameController

| 属性 | 引用 |
|------|------|
| hud | GameHud 组件 |
| loadingGate | LoadingGate |
| fishLayer | FishLayer 节点 |
| vfxLayer | VfxLayer 节点 |
| fishPrefab | `assets/prefabs/Fish.prefab`（占位 Sprite 即可） |
| coinPrefab | `assets/prefabs/Coin.prefab` |
| bubblePrefab | `assets/prefabs/Bubble.prefab` |
| spineFish | 可选 SpineFishController |

## 3. 构建分包

使用 `settings/builder-performance.template.json`：

- **core**：Loading + HUD + FishingGameController
- **boss**：Spine 高模资源（后台预加载）

## 4. 验收

- 预览：点击抛竿 → 请求 RGS session/bet → 显示结果
- 控制台：`DrawCallMonitor` 报告 DrawCall ≤ 30
- 对象池：抛竿 20 次无 `instantiate` 峰值（Profiler GC 稳定）

## 5. Gateway

构建后：`make deploy-client`，访问 `http://localhost:18000/game/`
