/**
 * Provably Fair 验算 — 与 pkg/prng/provably_fair.go 算法完全一致
 * Roll = HMAC-SHA256(serverSeed, clientSeed:nonce:index) 前 8 字节 BigEndian / MaxUint64
 *
 * 回归向量 (pkg/prng/provably_fair_browser_test.go):
 *   Roll("abc123seed","client456","round-test-1") → 0.75434720333408778
 *   HashServerSeed("abc123seed") → d7e67bf1b02ad2c15e860ce392748c14279854cf69a74df10ccf11495f343c02
 */

const MAX_UINT64 = 18446744073709551615n;

export async function hashServerSeed(seed) {
    const enc = new TextEncoder();
    const buf = await crypto.subtle.digest('SHA-256', enc.encode(seed));
    return Array.from(new Uint8Array(buf))
        .map((b) => b.toString(16).padStart(2, '0'))
        .join('');
}

export async function rollIndex(serverSeed, clientSeed, nonce, index = 0) {
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

export async function roll(serverSeed, clientSeed, nonce) {
    return rollIndex(serverSeed, clientSeed, nonce, 0);
}

function round2(v) {
    return Math.round(v * 100) / 100;
}

export async function computeOutcome(serverSeed, clientSeed, nonce, betAmount) {
    const primaryRoll = await roll(serverSeed, clientSeed, nonce);
    const multiRoll = await rollIndex(serverSeed, clientSeed, nonce, 1);

    if (primaryRoll < 0.55) {
        return {
            roll: primaryRoll,
            fishState: 'miss',
            animationKey: 'fish_miss',
            multiplier: 0,
            winAmount: 0,
        };
    }
    if (primaryRoll < 0.9) {
        const multiplier = round2(1.5 + multiRoll * 3.5);
        return {
            roll: primaryRoll,
            fishState: 'bite',
            animationKey: 'fish_bite_normal',
            multiplier,
            winAmount: round2(betAmount * multiplier),
        };
    }
    const multiplier = round2(50 + multiRoll * 50);
    return {
        roll: primaryRoll,
        fishState: 'big_win',
        animationKey: 'fish_bite_bigwin',
        multiplier,
        winAmount: round2(betAmount * multiplier),
    };
}

export function parseProvablyFairJson(text) {
    const data = JSON.parse(text);
    const pf = data.provablyFair || data;
    return {
        serverSeed: pf.serverSeed || '',
        serverSeedHash: pf.serverSeedHash || '',
        clientSeed: pf.clientSeed || '',
        nonce: pf.nonce || data.roundId || '',
        roll: pf.roll ?? '',
        betAmount: data.betAmount ?? '',
    };
}
