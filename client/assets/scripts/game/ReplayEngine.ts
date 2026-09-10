/**
 * 确定性回放引擎 — 与 pkg/prng/replay.go 算法一致
 * 仅依赖种子 + 基础输入，100% 复现场景参数
 */

export interface ReplayInputs {
    serverSeed: string;
    clientSeed: string;
    nonce: string;
    betAmount: number;
}

export interface ReplayPoint {
    x: number;
    y: number;
}

export interface ReplayScene {
    weather: string;
    fishSpecies: string;
    fishPath: ReplayPoint[];
    biteProp: string;
    castDurationMs: number;
    fishSpeed: number;
}

export interface ReplayOutcome {
    fishState: string;
    animationKey: string;
    multiplier: number;
    winAmount: number;
    roll: number;
}

const WEATHER = ['clear', 'cloudy', 'rain', 'storm'];
const SPECIES = ['bass', 'trout', 'tuna', 'salmon', 'shark', 'marlin'];
const PROPS = ['worm', 'lure', 'fly', 'jig'];
const MAX_UINT64 = 18446744073709551615n;

async function rollIndex(serverSeed: string, clientSeed: string, nonce: string, index = 0): Promise<number> {
    const payload = `${clientSeed}:${nonce}:${index}`;
    const enc = new TextEncoder();
    const key = await crypto.subtle.importKey(
        'raw',
        enc.encode(serverSeed),
        { name: 'HMAC', hash: 'SHA-256' },
        false,
        ['sign'],
    );
    const sig = await crypto.subtle.sign('HMAC', key, enc.encode(payload));
    const bytes = new Uint8Array(sig);
    let n = 0n;
    for (let i = 0; i < 8; i++) {
        n = (n << 8n) | BigInt(bytes[i]);
    }
    return Number(n) / Number(MAX_UINT64);
}

function pickIndex(r: number, n: number): number {
    const idx = Math.floor(r * n);
    return Math.min(Math.max(idx, 0), n - 1);
}

function round2(v: number): number {
    return Math.round(v * 100) / 100;
}

async function computeOutcome(serverSeed: string, clientSeed: string, nonce: string, betAmount: number): Promise<ReplayOutcome> {
    const roll = await rollIndex(serverSeed, clientSeed, nonce, 0);
    const multiRoll = await rollIndex(serverSeed, clientSeed, nonce, 1);

    if (roll < 0.55) {
        return { roll, fishState: 'miss', animationKey: 'fish_miss', multiplier: 0, winAmount: 0 };
    }
    if (roll < 0.9) {
        const multiplier = round2(1.5 + multiRoll * 3.5);
        return { roll, fishState: 'bite', animationKey: 'fish_bite_normal', multiplier, winAmount: round2(betAmount * multiplier) };
    }
    const multiplier = round2(50 + multiRoll * 50);
    return { roll, fishState: 'big_win', animationKey: 'fish_bite_bigwin', multiplier, winAmount: round2(betAmount * multiplier) };
}

export async function computeReplayScene(inputs: ReplayInputs): Promise<{ scene: ReplayScene; outcome: ReplayOutcome }> {
    const { serverSeed, clientSeed, nonce, betAmount } = inputs;
    const outcome = await computeOutcome(serverSeed, clientSeed, nonce, betAmount);

    const weatherRoll = await rollIndex(serverSeed, clientSeed, nonce, 2);
    const speciesRoll = await rollIndex(serverSeed, clientSeed, nonce, 3);
    const biteRoll = await rollIndex(serverSeed, clientSeed, nonce, 10);
    const castRoll = await rollIndex(serverSeed, clientSeed, nonce, 11);
    const speedRoll = await rollIndex(serverSeed, clientSeed, nonce, 12);

    const fishPath: ReplayPoint[] = [];
    for (let i = 0; i < 3; i++) {
        const xRoll = await rollIndex(serverSeed, clientSeed, nonce, 4 + i * 2);
        const yRoll = await rollIndex(serverSeed, clientSeed, nonce, 5 + i * 2);
        fishPath.push({ x: round2(xRoll), y: round2(yRoll) });
    }

    const scene: ReplayScene = {
        weather: WEATHER[pickIndex(weatherRoll, WEATHER.length)],
        fishSpecies: SPECIES[pickIndex(speciesRoll, SPECIES.length)],
        fishPath,
        biteProp: PROPS[pickIndex(biteRoll, PROPS.length)],
        castDurationMs: 600 + Math.floor(castRoll * 400),
        fishSpeed: round2(0.5 + speedRoll * 1.5),
    };

    return { scene, outcome };
}
