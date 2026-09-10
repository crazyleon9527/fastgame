/**
 * 前端性能预算 — 与 Cocos Builder 分包、TexturePacker 图集配置对齐
 */
export const PerformanceConfig = {
    /** 全局 DrawCall 硬上限（含 UI） */
    maxDrawCalls: 30,

    /** 图集规格：1~2 张 2048²，所有鱼/竿/金币/气泡必须打入 atlas */
    atlas: {
        core: 'textures/atlas/core-2048',
        vfx: 'textures/atlas/vfx-2048',
        maxSize: 2048,
    },

    /** Asset Bundle 名称 — 在 Cocos Builder 中配置同名远程/本地包 */
    bundles: {
        /** Loading + HUD + 基础鱼种，目标 < 3MB（Brotli 后） */
        core: 'core',
        /** Boss / 高阶鱼种 Spine + 史诗音效，后台异步预加载 */
        boss: 'boss',
        /** 可选音效分包 */
        audio: 'audio',
    },

    /** 对象池初始容量（禁止在 update 中 instantiate/destroy） */
    pools: {
        fish: { initial: 12, maxActive: 24 },
        coin: { initial: 30, maxActive: 60 },
        bubble: { initial: 20, maxActive: 40 },
    },

    /** Spine 动画名 ↔ 服务端 animationKey / fishState */
    spineTracks: {
        swim: 'swim',
        miss: 'fish_miss',
        bite: 'fish_bite_normal',
        bigWin: 'fish_bite_bigwin',
        cast: 'cast',
    } as Record<string, string>,

    /** 弱网首屏：2 秒内必须出现 Loading 动画（core 包体积预算） */
    coreBundleMaxBytes: 3 * 1024 * 1024,

    /** 预加载 Boss 包的最小空闲时间（ms），避免阻塞首屏交互 */
    bossPreloadDelayMs: 1500,
};
