import { Node, Prefab, tween, Vec3, UIOpacity } from 'cc';
import { NodePoolManager } from '../util/NodePoolManager';
import { PerformanceConfig } from '../config/PerformanceConfig';

/**
 * 派彩金币粒子对象池 — big_win / bite 时 burst，结束后回收
 */
export class CoinBurstPool {
    private readonly pool: NodePoolManager;

    constructor(
        private readonly parent: Node,
        coinPrefab: Prefab,
    ) {
        const cfg = PerformanceConfig.pools.coin;
        this.pool = new NodePoolManager(coinPrefab, cfg.maxActive, cfg.initial, (node) => {
            tween(node).stop();
            const opacity = node.getComponent(UIOpacity) ?? node.addComponent(UIOpacity);
            opacity.opacity = 255;
            node.setScale(0.6, 0.6, 1);
        });
    }

    burst(count: number, origin: Vec3, intensity: number): void {
        const n = Math.min(count, PerformanceConfig.pools.coin.maxActive);
        for (let i = 0; i < n; i++) {
            const coin = this.pool.acquire();
            if (!coin) {
                break;
            }
            this.parent.addChild(coin);
            coin.setPosition(origin);

            const angle = (Math.PI * 2 * i) / n;
            const dist = 40 + intensity * 80;
            const target = new Vec3(
                origin.x + Math.cos(angle) * dist,
                origin.y + Math.sin(angle) * dist + 30,
                0,
            );

            tween(coin)
                .parallel(
                    tween().to(0.5, { position: target }),
                    tween().to(0.5, { scale: new Vec3(1, 1, 1) }),
                )
                .delay(0.2)
                .call(() => this.pool.release(coin))
                .start();
        }
    }
}
