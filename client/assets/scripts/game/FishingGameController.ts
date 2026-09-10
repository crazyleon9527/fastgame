import { _decorator, Component, Animation, tween, Vec3 } from 'cc';
import { GameConfig } from '../config/GameConfig';
import { RgsClient, BetResponse } from '../network/RgsClient';
import { GameHud } from '../ui/GameHud';
import { generateId } from '../util/Uuid';

const { ccclass, property } = _decorator;

/**
 * 钓鱼游戏主控制器
 * 胖服务端瘦客户端：仅上报 cast 动作 + betAmount，严禁本地计算任何游戏结果
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
        try {
            await this.client.createSession(
                GameConfig.merchantId,
                GameConfig.userId,
                GameConfig.gameCode,
                GameConfig.clientSeed || undefined,
            );
            await this.refreshBalance();
            this.hud?.setStatus(`就绪 | Seed Hash: ${this.client.session.serverSeedHash.slice(0, 12)}…`);
        } catch (err) {
            console.error('[FishingGame] session', err);
            this.hud?.setStatus('无法建立游戏会话');
        }
    }

    /** 绑定到抛竿按钮 Click Events */
    async onCastClick(): Promise<void> {
        if (this.casting) {
            return;
        }
        if (this.client.session.isExpired()) {
            await this.start();
        }

        this.casting = true;
        this.hud?.setInteractable(false);
        this.hud?.setStatus('抛竿中...');

        try {
            await this.playCastAnimation();
            const sequenceId = this.client.session.consumeSequence();
            const result = await this.client.bet({
                merchantId: GameConfig.merchantId,
                userId: GameConfig.userId,
                sessionToken: this.client.session.token,
                gameCode: GameConfig.gameCode,
                action: 'cast',
                betAmount: GameConfig.defaultBet,
                roundId: generateId('round'),
                sequenceId,
                clientSeed: this.client.session.clientSeed,
            });
            await this.playResultAnimation(result);
            this.hud?.setBalance(result.balance);
            this.hud?.setWin(result.winAmount, result.multiplier);
            this.hud?.setStatus(this.statusText(result));
            console.info('[ProvablyFair]', result.provablyFair);
        } catch (err) {
            console.error('[FishingGame]', err);
            this.hud?.setStatus(`请求失败: ${(err as Error).message}`);
        } finally {
            this.casting = false;
            this.hud?.setInteractable(true);
        }
    }

    private async refreshBalance(): Promise<void> {
        const balance = await this.client.getBalance(GameConfig.merchantId, GameConfig.userId);
        this.hud?.setBalance(balance);
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
