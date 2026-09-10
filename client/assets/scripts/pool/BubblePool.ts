import { Node, Prefab, tween, Vec3, UIOpacity } from 'cc';
import { NodePoolManager } from '../util/NodePoolManager';
import { PerformanceConfig } from '../config/PerformanceConfig';

/**
 * 深海气泡 ambient 对象池 — 持续低速 spawn，循环复用
 */
export class BubblePool {
    private readonly pool: NodePoolManager;
    private timer: ReturnType<typeof setInterval> | null = null;

    constructor(
        private readonly parent: Node,
        bubblePrefab: Prefab,
    ) {
        const cfg = PerformanceConfig.pools.bubble;
        this.pool = new NodePoolManager(bubblePrefab, cfg.maxActive, cfg.initial, (node) => {
            tween(node).stop();
            const opacity = node.getComponent(UIOpacity) ?? node.addComponent(UIOpacity);
            opacity.opacity = 180;
        });
    }

    startAmbient(intervalMs = 400): void {
        this.stopAmbient();
        this.timer = setInterval(() => this.spawnOne(), intervalMs);
    }

    stopAmbient(): void {
        if (this.timer) {
            clearInterval(this.timer);
            this.timer = null;
        }
    }

    private spawnOne(): void {
        const bubble = this.pool.acquire();
        if (!bubble) {
            return;
        }
        this.parent.addChild(bubble);

        const x = (Math.random() - 0.5) * 600;
        const y = -220 + Math.random() * 40;
        bubble.setPosition(x, y, 0);
        bubble.setScale(0.3 + Math.random() * 0.5, 0.3 + Math.random() * 0.5, 1);

        const opacity = bubble.getComponent(UIOpacity) ?? bubble.addComponent(UIOpacity);
        opacity.opacity = 120 + Math.random() * 80;

        tween(bubble)
            .to(2 + Math.random() * 2, { position: new Vec3(x, y + 280, 0) })
            .call(() => this.pool.release(bubble))
            .start();

        tween(opacity).to(3, { opacity: 0 }).start();
    }
}
