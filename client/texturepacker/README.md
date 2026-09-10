# TexturePacker 图集规范

所有 **深海鱼、鱼竿、金币、气泡** 切片必须打入 **1~2 张 2048×2048** Sprite Sheet，禁止散图单独加载。

## 目标

| 指标 | 预算 |
|------|------|
| 图集数量 | 1~2 张 |
| 单张尺寸 | 2048×2048 |
| 全局 DrawCall | ≤ 30 |
| 格式 | PNG-32 → Cocos `SpriteAtlas` |

## 目录

```
assets/textures/source/     # 原始散图（不提交 build，仅美术源）
assets/textures/atlas/      # TexturePacker 导出 → Cocos 导入
  core-2048.plist + .png    # 鱼、竿、HUD、常规 UI
  vfx-2048.plist + .png     # 金币、气泡、特效
```

## 导出命令 (CLI)

```bash
# 安装 TexturePacker: https://www.codeandweb.com/texturepacker
TexturePacker \
  --format cocos2d-x \
  --sheet assets/textures/atlas/core-2048.png \
  --data assets/textures/atlas/core-2048.plist \
  --max-size 2048 \
  --size-constraints POT \
  --multipack \
  --trim-mode Trim \
  --disable-rotation \
  assets/textures/source/core/
```

`vfx-2048` 同理，源目录改为 `assets/textures/source/vfx/`。

## Cocos 编辑器

1. 导入 `.plist` + `.png`，类型设为 **SpriteAtlas**
2. 所有 `Sprite` 组件引用 Atlas 内 `SpriteFrame`，**禁止** `resources.load('textures/fish_01')` 散图
3. Prefab（鱼/金币/气泡）共用 **同一 Material**（默认 UI/Spine 材质），利于合批

## 校验

预览场景挂载 `DrawCallMonitor`，多鱼同屏 + 金币 burst 时 DrawCall 应 ≤ 30。
