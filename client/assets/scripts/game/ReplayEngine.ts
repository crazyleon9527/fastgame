/**
 * 确定性回放引擎 — 与 pkg/prng/replay.go + engine.go 算法一致
 * 金额使用 int64 定点数（Scale=10000），与后端 money.Amount 对齐
 */

export interface ReplayInputs {
    serverSeed: string;
    clientSeed: string;
    nonce: string;
    /** major 或 minor units；>=10000 的整数视为 minor */
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
    betAmountMinor: number;
    winAmountMinor: number;
    multiplierMinor: number;
}

const MONEY_SCALE = 10000;
const WEATHER = ['clear', 'cloudy', 'rain', 'storm'];
const SPECIES = ['bass', 'trout', 'tuna', 'salmon', 'shark', 'marlin'];
const PROPS = ['worm', 'lure', 'fly', 'jig'];
const MAX_UINT64 = 18446744073709551615n;

function betToMinor(betAmount: number): number {
    if (Number.isInteger(betAmount) && betAmount >= MONEY_SCALE) return betAmount;
    return Math.round(betAmount * MONEY_SCALE);
}

function minorToMajor(minor: number): number {
    return minor / MONEY_SCALE;
}

function rollToFloat(n: bigint): number {
    return Number(n) / Number(MAX_UINT64);
}

function rollBelow(n: bigint, num: number, den: number): boolean {
    return n < (MAX_UINT64 / BigInt(den)) * BigInt(num);
}

function multiplierFromRange(multiRoll: bigint, minMult: number, maxMult: number): number {
    const span = BigInt(maxMult - minMult);
    return Number(BigInt(minMult) + (multiRoll * span) / MAX_UINT64);
}

function applyMultiplier(betMinor: number, multMinor: number): number {
    if (betMinor <= 0 || multMinor <= 0) return 0;
    return Number((BigInt(betMinor) * BigInt(multMinor)) / BigInt(MONEY_SCALE));
}

async function rollIndexUint64(serverSeed: string, clientSeed: string, nonce: string, index = 0): Promise<bigint> {
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
    return n;
}

async function rollIndex(serverSeed: string, clientSeed: string, nonce: string, index = 0): Promise<number> {
    return rollToFloat(await rollIndexUint64(serverSeed, clientSeed, nonce, index));
}

function pickIndex(r: number, n: number): number {
    const idx = Math.floor(r * n);
    return Math.min(Math.max(idx, 0), n - 1);
}

function round2(v: number): number {
    return Math.round(v * 100) / 100;
}

async function computeOutcome(serverSeed: string, clientSeed: string, nonce: string, betAmount: number): Promise<ReplayOutcome> {
    const betMinor = betToMinor(betAmount);
    const rollU = await rollIndexUint64(serverSeed, clientSeed, nonce, 0);
    const roll = rollToFloat(rollU);

    if (rollBelow(rollU, 55, 100)) {
        return { roll, fishState: 'miss', animationKey: 'fish_miss', multiplier: 0, winAmount: 0, betAmountMinor: betMinor, winAmountMinor: 0, multiplierMinor: 0 };
    }

    const multiRoll = await rollIndexUint64(serverSeed, clientSeed, nonce, 1);

    if (rollBelow(rollU, 90, 100)) {
        const multMinor = multiplierFromRange(multiRoll, 15000, 50000);
        const winMinor = applyMultiplier(betMinor, multMinor);
        return {
            roll,
            fishState: 'bite',
            animationKey: 'fish_bite_normal',
            multiplier: minorToMajor(multMinor),
            winAmount: minorToMajor(winMinor),
            betAmountMinor: betMinor,
            winAmountMinor: winMinor,
            multiplierMinor: multMinor,
        };
    }

    const multMinor = multiplierFromRange(multiRoll, 500000, 1000000);
    const winMinor = applyMultiplier(betMinor, multMinor);
    return {
        roll,
        fishState: 'big_win',
        animationKey: 'fish_bite_bigwin',
        multiplier: minorToMajor(multMinor),
        winAmount: minorToMajor(winMinor),
        betAmountMinor: betMinor,
        winAmountMinor: winMinor,
        multiplierMinor: multMinor,
    };
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
