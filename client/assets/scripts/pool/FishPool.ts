import { Node, Prefab, tween, Vec3, UITransform } from 'cc';
import { NodePoolManager } from '../util/NodePoolManager';
import { PerformanceConfig } from '../config/PerformanceConfig';
import type { ReplayScene } from '../game/ReplayEngine';

export interface FishPathPoint {
    x: number;
    y: number;
}

/**
 * 游动鱼群对象池 — 多鱼同屏时复用节点，避免 GC Pause
 */
export class FishPool {
    private readonly pool: NodePoolManager;
    private readonly active: Node[] = [];

    constructor(
        private readonly parent: Node,
        fishPrefab: Prefab,
    ) {
        const cfg = PerformanceConfig.pools.fish;
        this.pool = new NodePoolManager(fishPrefab, cfg.maxActive, cfg.initial, (node) => {
            tween(node).stop();
            node.setScale(1, 1, 1);
            node.setPosition(0, 0, 0);
        });
    }

    spawnSchool(scene: ReplayScene, onComplete?: () => void): void {
        this.recycleAll();
        const path = scene.fishPath;
        if (!path?.length) {
            onComplete?.();
            return;
        }

        const node = this.pool.acquire();
        if (!node) {
            console.warn('[FishPool] max active fish reached');
            onComplete?.();
            return;
        }

        this.parent.addChild(node);
        this.active.push(node);

        const transform = node.getComponent(UITransform);
        const width = transform?.contentSize.width ?? 800;
        const height = transform?.contentSize.height ?? 400;
        const stepMs = 300 / scene.fishSpeed;

        let chain = tween(node);
        path.forEach((pt: FishPathPoint) => {
            const x = (pt.x - 0.5) * width;
            const y = (pt.y - 0.5) * height;
            chain = chain.to(stepMs / 1000, { position: new Vec3(x, y, 0) });
        });
        chain
            .call(() => {
                this.recycle(node);
                onComplete?.();
            })
            .start();
    }

    recycle(node: Node): void {
        const idx = this.active.indexOf(node);
        if (idx >= 0) {
            this.active.splice(idx, 1);
        }
        this.pool.release(node);
    }

    recycleAll(): void {
        [...this.active].forEach((n) => this.recycle(n));
    }
}
