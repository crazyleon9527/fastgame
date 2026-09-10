import { _decorator, Component, Animation, Prefab, Node, Vec3, tween } from 'cc';
import { GameConfig } from '../config/GameConfig';
import { RgsClient, BetResponse, ReplayPayload } from '../network/RgsClient';
import { GameHud } from '../ui/GameHud';
import { generateId } from '../util/Uuid';
import { ReplayScene } from './ReplayEngine';
import { FishPool } from '../pool/FishPool';
import { CoinBurstPool } from '../pool/CoinBurstPool';
import { BubblePool } from '../pool/BubblePool';
import { SpineFishController } from '../animation/SpineFishController';
import { LoadingGate } from '../ui/LoadingGate';

const { ccclass, property } = _decorator;

/**
 * 钓鱼游戏主控制器
 * 确定性回放 + 对象池 + Spine 骨骼动画 + 分包预加载
 */
@ccclass('FishingGameController')
export class FishingGameController extends Component {
    @property(GameHud)
    hud: GameHud | null = null;

    @property(LoadingGate)
    loadingGate: LoadingGate | null = null;

    @property(Animation)
    rodAnimation: Animation | null = null;

    /** 兼容旧版 Animation 节点；优先使用 spineFish */
    @property(Animation)
    fishAnimation: Animation | null = null;

    @property(SpineFishController)
    spineFish: SpineFishController | null = null;

    @property(Node)
    fishLayer: Node | null = null;

    @property(Node)
    vfxLayer: Node | null = null;

    @property(Prefab)
    fishPrefab: Prefab | null = null;

    @property(Prefab)
    coinPrefab: Prefab | null = null;

    @property(Prefab)
    bubblePrefab: Prefab | null = null;

    private client = new RgsClient();
    private casting = false;
    private fishPool: FishPool | null = null;
    private coinPool: CoinBurstPool | null = null;
    private bubblePool: BubblePool | null = null;

    async start(): Promise<void> {
        this.initPools();
        this.bubblePool?.startAmbient(450);
        this.spineFish?.playSwim(true);

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

        this.loadingGate?.bundleLoader.scheduleBossPreload();
    }

    onDestroy(): void {
        this.bubblePool?.stopAmbient();
        this.fishPool?.recycleAll();
    }

    private initPools(): void {
        const fishParent = this.fishLayer ?? this.node;
        const vfxParent = this.vfxLayer ?? this.node;
        if (this.fishPrefab) {
            this.fishPool = new FishPool(fishParent, this.fishPrefab);
        }
        if (this.coinPrefab) {
            this.coinPool = new CoinBurstPool(vfxParent, this.coinPrefab);
        }
        if (this.bubblePrefab) {
            this.bubblePool = new BubblePool(vfxParent, this.bubblePrefab);
        }
    }

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

    private playFishPath(scene: ReplayScene | ReplayPayload['scene']): Promise<void> {
        if (this.fishPool) {
            return new Promise((resolve) => {
                this.fishPool!.spawnSchool(scene as ReplayScene, resolve);
            });
        }
        return this.playFishPathLegacy(scene);
    }

    /** 无 prefab 时的单节点 tween 回退 */
    private async playFishPathLegacy(scene: ReplayScene | ReplayPayload['scene']): Promise<void> {
        const node = this.fishAnimation?.node ?? this.spineFish?.node;
        if (!node || !scene.fishPath?.length) {
            return;
        }
        return new Promise((resolve) => {
            let chain = tween(node);
            const stepMs = 300 / scene.fishSpeed;
            scene.fishPath.forEach((pt) => {
                chain = chain.to(stepMs / 1000, { position: new Vec3((pt.x - 0.5) * 800, (pt.y - 0.5) * 400, 0) });
            });
            chain.call(resolve).start();
        });
    }

    private async refreshBalance(): Promise<void> {
        const balance = await this.client.getBalance(GameConfig.merchantId, GameConfig.userId);
        this.hud?.setBalance(balance);
    }

    private async playResultAnimation(result: BetResponse): Promise<void> {
        const origin = this.spineFish?.node.position ?? this.fishAnimation?.node.position ?? new Vec3(0, 0, 0);

        if (result.fishState === 'big_win') {
            this.coinPool?.burst(24, origin, 1.2);
        } else if (result.fishState === 'bite') {
            this.coinPool?.burst(10, origin, 0.6);
        }

        if (this.spineFish) {
            await this.spineFish.playResult(result.fishState, result.animationKey);
            return;
        }

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
        const node = this.spineFish?.node ?? this.fishAnimation?.node;
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
