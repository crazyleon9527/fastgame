import { AssetManager, assetManager } from 'cc';
import { PerformanceConfig } from '../config/PerformanceConfig';

export type BundleName = keyof typeof PerformanceConfig.bundles;

/**
 * 分包加载 — core 首屏 <3MB，boss/audio 后台预加载
 */
export class AssetBundleLoader {
    private bundles = new Map<string, AssetManager.Bundle>();
    private bossPreloadStarted = false;

    async loadCore(onProgress?: (ratio: number) => void): Promise<AssetManager.Bundle> {
        return this.loadBundle('core', onProgress);
    }

    /** 首屏就绪后异步预加载 Boss 包，不阻塞抛竿 */
    scheduleBossPreload(delayMs = PerformanceConfig.bossPreloadDelayMs): void {
        if (this.bossPreloadStarted) {
            return;
        }
        this.bossPreloadStarted = true;
        setTimeout(() => {
            this.loadBundle('boss').catch((err) => {
                console.warn('[AssetBundleLoader] boss preload failed (non-fatal)', err);
            });
        }, delayMs);
    }

    async loadBundle(name: BundleName, onProgress?: (ratio: number) => void): Promise<AssetManager.Bundle> {
        const bundleName = PerformanceConfig.bundles[name];
        const cached = this.bundles.get(bundleName);
        if (cached) {
            return cached;
        }

        onProgress?.(0.1);
        return new Promise((resolve, reject) => {
            assetManager.loadBundle(bundleName, {}, (err, bundle) => {
                if (err || !bundle) {
                    reject(err ?? new Error(`bundle ${bundleName} missing`));
                    return;
                }
                this.bundles.set(bundleName, bundle);
                onProgress?.(1);
                resolve(bundle);
            });
        });
    }

    getBundle(name: BundleName): AssetManager.Bundle | undefined {
        return this.bundles.get(PerformanceConfig.bundles[name]);
    }

    /** 开发期校验 core 包体积预算 */
    warnIfCoreOversized(bytes: number): void {
        if (bytes > PerformanceConfig.coreBundleMaxBytes) {
            console.warn(
                `[AssetBundleLoader] core bundle ${(bytes / 1024 / 1024).toFixed(2)}MB exceeds ` +
                `${(PerformanceConfig.coreBundleMaxBytes / 1024 / 1024).toFixed(0)}MB budget`,
            );
        }
    }
}
