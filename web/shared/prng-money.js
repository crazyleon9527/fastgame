/**
 * 与 pkg/money + pkg/prng + pkg/par 对齐的定点数 PRNG 工具
 * Scale = 10000（4 位小数 minor units）
 *
 * ⚠️ 改动本文件前请先读这里：
 *   开奖结果 = PAR 表按 roll 权重区间定位（roll 走 HMAC-SHA256）。
 *   服务端口径在 pkg/prng/engine.go（outcomeFromRoll），表在 pkg/par/table.go。
 *   下面的 PAR_TABLE_96 是那张表的**镜像**——三处必须完全一致，
 *   否则客户端"验算"出来的派彩和服务端不一致，provably-fair 就失去意义。
 *   改表时请同步：pkg/par/table.go → 本文件 → 重跑 pkg/par 与 pkg/prng 的测试。
 */

export const MONEY_SCALE = 10000;
export const MAX_UINT64 = 18446744073709551615n;

/** 与 pkg/par/table.go 的 Default96 一一对应（multiplier_minor = 倍率 × 10000） */
export const PAR_TABLE_96 = [
    { id: 0, name: 'Missed', tier: 'MISS', multiplierMinor: 0, weight: 570000, tensionMs: 300 },
    { id: 1, name: 'Clownfish', tier: 'COMMON', multiplierMinor: 5000, weight: 160000, tensionMs: 600 },
    { id: 2, name: 'Sardine', tier: 'COMMON', multiplierMinor: 10000, weight: 170000, tensionMs: 700 },
    { id: 3, name: 'Flying Fish', tier: 'COMMON', multiplierMinor: 20000, weight: 50000, tensionMs: 800 },
    { id: 4, name: 'Tuna', tier: 'RARE', multiplierMinor: 50000, weight: 30000, tensionMs: 1200 },
    { id: 5, name: 'Manta Ray', tier: 'RARE', multiplierMinor: 100000, weight: 12000, tensionMs: 1500 },
    { id: 6, name: 'Swordfish', tier: 'RARE', multiplierMinor: 200000, weight: 5000, tensionMs: 1800 },
    { id: 7, name: 'Golden Turtle', tier: 'BOSS', multiplierMinor: 500000, weight: 2000, tensionMs: 2500 },
    { id: 8, name: 'Hammerhead Shark', tier: 'BOSS', multiplierMinor: 1000000, weight: 800, tensionMs: 3000 },
    { id: 9, name: 'Giant Squid', tier: 'BOSS', multiplierMinor: 2500000, weight: 160, tensionMs: 3500 },
    { id: 10, name: 'Megalodon', tier: 'BOSS', multiplierMinor: 5000000, weight: 40, tensionMs: 4500 },
];

export const PAR_TOTAL_WEIGHT = PAR_TABLE_96.reduce((sum, e) => sum + e.weight, 0);

/** 理论 RTP（0.96 表示 96%），与 pkg/par.Table.RTP() 同式 */
export function theoreticalRTP(table = PAR_TABLE_96) {
    const total = table.reduce((sum, e) => sum + e.weight, 0);
    const payout = table.reduce((sum, e) => sum + e.multiplierMinor * e.weight, 0);
    return payout / (total * MONEY_SCALE);
}

/**
 * 把 [0, MaxUint64] 的 roll 映射到权重区间，等价于服务端的
 * bits.Mul64(roll, totalWeight) 取高 64 位（floor(roll × total / 2^64)）。
 *
 * 不能用 Number 直接乘：roll 超过 2^53 就会丢精度，必须走 BigInt。
 */
export function samplePAR(rollU, table = PAR_TABLE_96) {
    const total = table.reduce((sum, e) => sum + e.weight, 0);
    const pick = Number((BigInt(rollU) * BigInt(total)) >> 64n);

    let cumulative = 0;
    for (const entry of table) {
        cumulative += entry.weight;
        if (pick < cumulative) return entry;
    }
    return table[table.length - 1];
}

function animationFor(tier) {
    if (tier === 'MISS') return { fishState: 'miss', animationKey: 'fish_miss' };
    if (tier === 'BOSS') return { fishState: 'big_win', animationKey: 'fish_bite_bigwin' };
    return { fishState: 'bite', animationKey: 'fish_bite_normal' };
}

export function rollToFloat(n) {
    return Number(n) / Number(MAX_UINT64);
}

export function rollBelow(n, num, den) {
    return n < (MAX_UINT64 / BigInt(den)) * BigInt(num);
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

/**
 * 复现某局的派彩结果。
 * 服务端实现见 pkg/prng/engine.go 的 outcomeFromRoll：结果完全由 roll 决定，
 * roll 之外的索引只用于生成表现层（天气、鱼群路径等），与钱无关。
 */
export async function computeOutcome(serverSeed, clientSeed, nonce, betAmount) {
    const betMinor = betToMinor(betAmount);
    const rollU = await rollIndexUint64(serverSeed, clientSeed, nonce, 0);
    const roll = rollToFloat(rollU);

    const hit = samplePAR(rollU);
    const { fishState, animationKey } = animationFor(hit.tier);
    const winMinor = applyMultiplier(betMinor, hit.multiplierMinor);

    return {
        roll,
        fishState,
        animationKey,
        fishId: hit.id,
        fishName: hit.name,
        tier: hit.tier,
        tensionMs: hit.tensionMs,
        multiplier: minorToMajor(hit.multiplierMinor),
        multiplierMinor: hit.multiplierMinor,
        winAmount: minorToMajor(winMinor),
        winAmountMinor: winMinor,
        betAmountMinor: betMinor,
    };
}
