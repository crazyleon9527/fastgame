import { _decorator, Component, Animation, tween, Vec3, UITransform } from 'cc';
import { GameConfig } from '../config/GameConfig';
import { RgsClient, BetResponse, ReplayPayload, ReplayResponse } from '../network/RgsClient';
import { GameHud } from '../ui/GameHud';
import { generateId } from '../util/Uuid';
import { ReplayScene } from './ReplayEngine';

const { ccclass, property } = _decorator;

/**
 * 钓鱼游戏主控制器
 * 确定性回放：抛竿时长、鱼群轨迹、咬钩道具均由种子派生，与服务器 100% 一致
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
            const sequenceId = this.client.session.consumeSequence();
            const roundId = generateId('round');

            const result = await this.client.bet({
                merchantId: GameConfig.merchantId,
                userId: GameConfig.userId,
                sessionToken: this.client.session.token,
                gameCode: GameConfig.gameCode,
                action: 'cast',
                betAmount: GameConfig.defaultBet,
                roundId,
                sequenceId,
                clientSeed: this.client.session.clientSeed,
            });

            if (result.replay?.inputs.serverSeed) {
                await this.playDeterministicRound(result.replay, result);
            } else {
                await this.playResultAnimation(result);
            }

            this.hud?.setBalance(result.balance);
            this.hud?.setWin(result.winAmount, result.multiplier);
            this.hud?.setStatus(this.statusText(result));
            this.logVerifyLink(result);
        } catch (err) {
            console.error('[FishingGame]', err);
            this.hud?.setStatus(`请求失败: ${(err as Error).message}`);
        } finally {
            this.casting = false;
            this.hud?.setInteractable(true);
        }
    }

    /** 从历史 roundId 100% 复现整局（确定性回放） */
    async replayRound(roundId: string): Promise<void> {
        this.casting = true;
        this.hud?.setInteractable(false);
        try {
            const data = await this.client.fetchReplay(roundId);
            const replay = data.replay;
            if (!replay) {
                throw new Error('replay payload missing');
            }
            await this.playDeterministicRound(replay, {
                fishState: data.fishState,
                animationKey: data.animationKey,
            } as BetResponse);
            this.hud?.setStatus(`回放 ${roundId} | ${data.fishState}`);
        } finally {
            this.casting = false;
            this.hud?.setInteractable(true);
        }
    }

    private async playDeterministicRound(replay: ReplayPayload, result: BetResponse): Promise<void> {
        const { scene } = replay;
        this.hud?.setStatus(`天气: ${scene.weather} | ${scene.fishSpecies} | 饵: ${scene.biteProp}`);
        await this.playCastAnimation(scene.castDurationMs);
        await this.playFishPath(scene);
        await this.playResultAnimation(result);
    }

    private async playFishPath(scene: ReplayScene | ReplayPayload['scene']): Promise<void> {
        const node = this.fishAnimation?.node;
        if (!node || !scene.fishPath?.length) {
            return;
        }

        const transform = node.getComponent(UITransform);
        const width = transform?.contentSize.width ?? 800;
        const height = transform?.contentSize.height ?? 400;

        return new Promise((resolve) => {
            let chain = tween(node);
            const stepMs = 300 / scene.fishSpeed;
            scene.fishPath.forEach((pt) => {
                const x = (pt.x - 0.5) * width;
                const y = (pt.y - 0.5) * height;
                chain = chain.to(stepMs / 1000, { position: new Vec3(x, y, 0) });
            });
            chain.call(resolve).start();
        });
    }

    private async refreshBalance(): Promise<void> {
        const balance = await this.client.getBalance(GameConfig.merchantId, GameConfig.userId);
        this.hud?.setBalance(balance);
    }

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

    private playCastAnimation(durationMs: number): Promise<void> {
        if (this.rodAnimation) {
            const state = this.rodAnimation.getState('cast');
            if (state) {
                this.rodAnimation.play('cast');
            }
        }
        return this.wait(durationMs / 1000);
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

    private logVerifyLink(result: BetResponse): void {
        const pf = result.provablyFair;
        if (!pf) return;
        const base = GameConfig.gatewayUrl.replace(/\/$/, '');
        const params = new URLSearchParams({
            serverSeed: pf.serverSeed,
            serverSeedHash: pf.serverSeedHash,
            clientSeed: pf.clientSeed,
            nonce: pf.nonce,
            roll: String(pf.roll),
            betAmount: String(GameConfig.defaultBet),
        });
        console.info(`[ProvablyFair] 验算: ${base}/verify/?${params.toString()}`);
        console.info(`[Replay] 回放: ${base}/replay/?roundId=${result.roundId}`);
    }
}
