import { _decorator, Component, director } from 'cc';
import { DEBUG } from 'cc/env';
import { PerformanceConfig } from '../config/PerformanceConfig';

const { ccclass, property } = _decorator;

/**
 * DrawCall 预算监控 — 开发/预览环境告警，确保图集合批后 ≤30
 */
@ccclass('DrawCallMonitor')
export class DrawCallMonitor extends Component {
    @property
    enabledInPreview = true;

    @property
    warnIntervalSec = 2;

    private elapsed = 0;

    update(dt: number): void {
        if (!this.enabledInPreview || !DEBUG) {
            return;
        }
        this.elapsed += dt;
        if (this.elapsed < this.warnIntervalSec) {
            return;
        }
        this.elapsed = 0;

        const stats = (director.root as { device?: { numDrawCalls?: number } } | null)?.device;
        const drawCalls = stats?.numDrawCalls;
        if (drawCalls != null && drawCalls > PerformanceConfig.maxDrawCalls) {
            console.warn(
                `[DrawCallMonitor] drawCalls=${drawCalls} exceeds budget ${PerformanceConfig.maxDrawCalls}. ` +
                'Check TexturePacker atlases and disable per-fish loose textures.',
            );
        }
    }
}
