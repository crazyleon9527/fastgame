import { Node, NodePool, Prefab, instantiate, isValid } from 'cc';

export type PoolResetFn = (node: Node) => void;

/**
 * 通用 NodePool 封装 — 禁止在游戏循环中 instantiate/destroy
 */
export class NodePoolManager {
    private readonly pool: NodePool;
    private activeCount = 0;

    constructor(
        private readonly prefab: Prefab,
        private readonly maxActive: number,
        initialSize: number,
        private readonly onReset?: PoolResetFn,
    ) {
        this.pool = new NodePool();
        for (let i = 0; i < initialSize; i++) {
            this.pool.put(instantiate(prefab));
        }
    }

    acquire(): Node | null {
        if (this.activeCount >= this.maxActive) {
            return null;
        }
        const node = this.pool.size() > 0 ? this.pool.get()! : instantiate(this.prefab);
        this.activeCount++;
        node.active = true;
        return node;
    }

    release(node: Node): void {
        if (!isValid(node)) {
            return;
        }
        this.onReset?.(node);
        node.removeFromParent();
        node.active = false;
        this.pool.put(node);
        this.activeCount = Math.max(0, this.activeCount - 1);
    }

    releaseAll(nodes: Node[]): void {
        nodes.forEach((n) => this.release(n));
    }

    get active(): number {
        return this.activeCount;
    }

    get pooled(): number {
        return this.pool.size();
    }
}
