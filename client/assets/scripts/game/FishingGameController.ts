import { _decorator, Component, Animation, tween, Vec3 } from 'cc';
import { GameConfig } from '../config/GameConfig';
import { RgsClient, BetResponse } from '../network/RgsClient';
import { GameHud } from '../ui/GameHud';
import { generateId } from '../util/Uuid';

const { ccclass, property } = _decorator;

/**
 * 钓鱼游戏主控制器
 * 严禁本地计算中奖结果 — 仅根据 RGS 返回的 fishState / animationKey 播放动画
 */
@ccclass('FishingGameController')
export class FishingGameController extends Component {
    @property(GameHud)
    hud: GameHud | null = null;

    @property(Animation)
    rodAnimation: Animation | null = null;

    @property(Animation)
    fishAnimation: Animation | null = null;

    private client = new RgsClient();
    private casting = false;

    async start(): Promise<void> {
        await this.refreshBalance();
        this.hud?.setStatus('点击抛竿开始游戏');
    }

    /** 绑定到抛竿按钮 Click Events */
    async onCastClick(): Promise<void> {
        if (this.casting) {
            return;
        }
        this.casting = true;
        this.hud?.setInteractable(false);
        this.hud?.setStatus('抛竿中...');

        try {
            await this.playCastAnimation();
            const result = await this.client.bet({
                merchantId: GameConfig.merchantId,
                userId: GameConfig.userId,
                roundId: generateId('round'),
                gameCode: GameConfig.gameCode,
                betAmount: GameConfig.defaultBet,
                idempotencyToken: generateId('tok'),
            });
            await this.playResultAnimation(result);
            this.hud?.setBalance(result.balance);
            this.hud?.setWin(result.winAmount, result.multiplier);
            this.hud?.setStatus(this.statusText(result));
        } catch (err) {
            console.error('[FishingGame]', err);
            this.hud?.setStatus(`请求失败: ${(err as Error).message}`);
        } finally {
            this.casting = false;
            this.hud?.setInteractable(true);
        }
    }

    private async refreshBalance(): Promise<void> {
        try {
            const balance = await this.client.getBalance(GameConfig.merchantId, GameConfig.userId);
            this.hud?.setBalance(balance);
        } catch (err) {
            console.error('[FishingGame] balance', err);
            this.hud?.setStatus('无法连接服务器');
        }
    }

    /** 根据服务端 animationKey 驱动表现层 — 不做任何概率计算 */
    private async playResultAnimation(result: BetResponse): Promise<void> {
        const clip = result.animationKey || result.fishState;
        if (this.fishAnimation) {
            const state = this.fishAnimation.getState(clip);
            if (state) {
                this.fishAnimation.play(clip);
                await this.wait(state.duration || 1);
                return;
            }
        }
        await this.playFallbackAnimation(result.fishState);
    }

    private playFallbackAnimation(fishState: string): Promise<void> {
        const node = this.fishAnimation?.node;
        if (!node) {
            return Promise.resolve();
        }
        return new Promise((resolve) => {
            if (fishState === 'big_win') {
                tween(node)
                    .to(0.3, { scale: new Vec3(1.5, 1.5, 1) })
                    .to(0.3, { scale: new Vec3(1, 1, 1) })
                    .call(resolve)
                    .start();
            } else if (fishState === 'bite') {
                tween(node)
                    .by(0.2, { position: new Vec3(0, 30, 0) })
                    .by(0.2, { position: new Vec3(0, -30, 0) })
                    .call(resolve)
                    .start();
            } else {
                tween(node).delay(0.5).call(resolve).start();
            }
        });
    }

    private playCastAnimation(): Promise<void> {
        if (this.rodAnimation) {
            const state = this.rodAnimation.getState('cast');
            if (state) {
                this.rodAnimation.play('cast');
                return this.wait(state.duration || 0.8);
            }
        }
        return this.wait(0.8);
    }

    private statusText(result: BetResponse): string {
        switch (result.fishState) {
            case 'big_win':
                return `大奖! ${result.multiplier.toFixed(1)}x`;
            case 'bite':
                return `咬钩了! ${result.multiplier.toFixed(2)}x`;
            default:
                return '鱼儿跑了，再试一次';
        }
    }

    private wait(seconds: number): Promise<void> {
        return new Promise((resolve) => setTimeout(resolve, seconds * 1000));
    }
}
