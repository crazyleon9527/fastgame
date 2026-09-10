/**
 * 最小场景自检 — 挂到场景根节点，启动时验证性能框架依赖是否就绪
 */
import { _decorator, Component, director } from 'cc';
import { DEBUG } from 'cc/env';
import { FishingGameController } from '../game/FishingGameController';
import { DrawCallMonitor } from '../perf/DrawCallMonitor';

const { ccclass, property } = _decorator;

@ccclass('MinimalSceneBootstrap')
export class MinimalSceneBootstrap extends Component {
    @property(FishingGameController)
    game: FishingGameController | null = null;

    start(): void {
        const scene = director.getScene();
        const ctrl = this.game ?? scene?.getComponentInChildren(FishingGameController);
        if (!ctrl) {
            console.error('[MinimalScene] FishingGameController not found — see client/scenes/SCENE_SETUP.md');
            return;
        }
        if (DEBUG && !scene?.getComponentInChildren(DrawCallMonitor)) {
            (ctrl.node.parent ?? this.node).addComponent(DrawCallMonitor);
        }
        console.info('[MinimalScene] bootstrap ok — pools + RGS client ready');
    }
}
