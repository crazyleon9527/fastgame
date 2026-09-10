import { _decorator, Component, sp, tween, Vec3 } from 'cc';
import { PerformanceConfig } from '../config/PerformanceConfig';

const { ccclass, property } = _decorator;

/**
 * Spine 骨骼鱼 — 替代逐帧序列图，包体/显存降低 70%+
 * 需在编辑器挂载 sp.Skeleton，骨骼资源来自 boss/core 分包
 */
@ccclass('SpineFishController')
export class SpineFishController extends Component {
    @property(sp.Skeleton)
    skeleton: sp.Skeleton | null = null;

    playSwim(loop = true): void {
        this.setTrack(PerformanceConfig.spineTracks.swim, loop);
    }

    async playResult(fishState: string, animationKey?: string): Promise<void> {
        const track = animationKey
            || PerformanceConfig.spineTracks[fishState]
            || PerformanceConfig.spineTracks[fishState === 'big_win' ? 'bigWin' : fishState]
            || fishState;

        if (this.skeleton?.findAnimation(track)) {
            await this.setTrackAsync(track, false);
            return;
        }
        await this.playFallback(fishState);
    }

    private setTrack(name: string, loop: boolean): void {
        this.skeleton?.setAnimation(0, name, loop);
    }

    private setTrackAsync(name: string, loop: boolean): Promise<void> {
        return new Promise((resolve) => {
            if (!this.skeleton) {
                resolve();
                return;
            }
            const entry = this.skeleton.setAnimation(0, name, loop);
            const duration = entry?.animation?.duration ?? 0.8;
            this.scheduleOnce(resolve, duration);
        });
    }

    private playFallback(fishState: string): Promise<void> {
        return new Promise((resolve) => {
            if (fishState === 'big_win') {
                tween(this.node)
                    .to(0.3, { scale: new Vec3(1.5, 1.5, 1) })
                    .to(0.3, { scale: new Vec3(1, 1, 1) })
                    .call(resolve)
                    .start();
            } else if (fishState === 'bite') {
                tween(this.node)
                    .by(0.2, { position: new Vec3(0, 30, 0) })
                    .by(0.2, { position: new Vec3(0, -30, 0) })
                    .call(resolve)
                    .start();
            } else {
                this.scheduleOnce(resolve, 0.5);
            }
        });
    }
}
