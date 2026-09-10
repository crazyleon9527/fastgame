/**
 * 与 pkg/money + pkg/prng/engine.go 对齐的定点数 PRNG 工具
 * Scale = 10000（4 位小数 minor units）
 */

export const MONEY_SCALE = 10000;
export const MAX_UINT64 = 18446744073709551615n;

export function rollToFloat(n) {
    return Number(n) / Number(MAX_UINT64);
}

export function rollBelow(n, num, den) {
    return n < (MAX_UINT64 / BigInt(den)) * BigInt(num);
}

export function multiplierFromRange(multiRoll, minMult, maxMult) {
    const span = BigInt(maxMult - minMult);
    return Number(BigInt(minMult) + (multiRoll * span) / MAX_UINT64);
}

export function applyMultiplier(betMinor, multMinor) {
    if (betMinor <= 0 || multMinor <= 0) return 0;
    return Number((BigInt(betMinor) * BigInt(multMinor)) / BigInt(MONEY_SCALE));
}

/** 接受 major（10.5）或 minor（105000） */
export function betToMinor(betAmount) {
    const n = Number(betAmount);
    if (!Number.isFinite(n) || n <= 0) return MONEY_SCALE;
    if (Number.isInteger(n) && n >= MONEY_SCALE) return n;
    return Math.round(n * MONEY_SCALE);
}

export function minorToMajor(minor) {
    return minor / MONEY_SCALE;
}

export function formatMinor(minor) {
    const whole = Math.trunc(minor / MONEY_SCALE);
    const frac = Math.abs(minor % MONEY_SCALE);
    return `${whole}.${String(frac).padStart(4, '0')}`;
}

export async function rollIndexUint64(serverSeed, clientSeed, nonce, index = 0) {
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

export async function hashServerSeed(seed) {
    const enc = new TextEncoder();
    const buf = await crypto.subtle.digest('SHA-256', enc.encode(seed));
    return Array.from(new Uint8Array(buf))
        .map((b) => b.toString(16).padStart(2, '0'))
        .join('');
}

export async function rollIndex(serverSeed, clientSeed, nonce, index = 0) {
    const n = await rollIndexUint64(serverSeed, clientSeed, nonce, index);
    return rollToFloat(n);
}

export async function roll(serverSeed, clientSeed, nonce) {
    return rollIndex(serverSeed, clientSeed, nonce, 0);
}

export async function computeOutcome(serverSeed, clientSeed, nonce, betAmount) {
    const betMinor = betToMinor(betAmount);
    const rollU = await rollIndexUint64(serverSeed, clientSeed, nonce, 0);
    const roll = rollToFloat(rollU);

    if (rollBelow(rollU, 55, 100)) {
        return {
            roll,
            fishState: 'miss',
            animationKey: 'fish_miss',
            multiplier: 0,
            multiplierMinor: 0,
            winAmount: 0,
            winAmountMinor: 0,
            betAmountMinor: betMinor,
        };
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
            multiplierMinor: multMinor,
            winAmount: minorToMajor(winMinor),
            winAmountMinor: winMinor,
            betAmountMinor: betMinor,
        };
    }

    const multMinor = multiplierFromRange(multiRoll, 500000, 1000000);
    const winMinor = applyMultiplier(betMinor, multMinor);
    return {
        roll,
        fishState: 'big_win',
        animationKey: 'fish_bite_bigwin',
        multiplier: minorToMajor(multMinor),
        multiplierMinor: multMinor,
        winAmount: minorToMajor(winMinor),
        winAmountMinor: winMinor,
        betAmountMinor: betMinor,
    };
}
