import { _decorator, Component, Label, ProgressBar } from 'cc';
import { AssetBundleLoader } from '../asset/AssetBundleLoader';

const { ccclass, property } = _decorator;

/**
 * 首屏 Loading — core 分包加载完成前展示，弱网 2 秒内必须可见
 */
@ccclass('LoadingGate')
export class LoadingGate extends Component {
    @property(Label)
    hintLabel: Label | null = null;

    @property(ProgressBar)
    progressBar: ProgressBar | null = null;

    @property
    rootToReveal: Component | null = null;

    private loader = new AssetBundleLoader();

    get bundleLoader(): AssetBundleLoader {
        return this.loader;
    }

    async start(): Promise<void> {
        this.node.active = true;
        this.setProgress(0, '加载核心资源…');
        try {
            await this.loader.loadCore((ratio) => this.setProgress(ratio * 0.9, '加载核心资源…'));
            this.setProgress(1, '就绪');
            this.loader.scheduleBossPreload();
            if (this.rootToReveal) {
                this.rootToReveal.node.active = true;
            }
            this.node.active = false;
        } catch (err) {
            console.error('[LoadingGate]', err);
            this.setProgress(0, '资源加载失败，请刷新');
        }
    }

    private setProgress(ratio: number, hint: string): void {
        if (this.progressBar) {
            this.progressBar.progress = ratio;
        }
        if (this.hintLabel) {
            this.hintLabel.string = hint;
        }
    }
}
