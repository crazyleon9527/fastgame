# FishingMinimal — 最小可玩场景

在 Cocos Creator 3.8 中验证对象池 / Spine / DrawCall 框架。

## 1. 场景文件

仓库已包含 starter：`client/assets/scenes/FishingMinimal.scene`  
在 Cocos Creator 3.8 中 **打开项目 → 双击该 scene**；若组件引用丢失，按下面步骤重新绑定。

## 2. 新建/修复场景

1. 场景名 `FishingMinimal`（路径 `assets/scenes/FishingMinimal.scene`）
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

## 5. DrawCall 实测

1. Cocos **预览**（DEBUG 模式）打开 `FishingMinimal`
2. 根节点挂 `MinimalSceneBootstrap`（自动附加 `DrawCallMonitor`）
3. 连续抛竿 20 次，观察控制台：
   - 无 `[DrawCallMonitor] drawCalls=… exceeds budget 30` 警告
   - Profiler 中 DrawCall 峰值 ≤ 30

## 6. Gateway

构建后：`make deploy-client`，访问 `http://localhost:18000/game/`
