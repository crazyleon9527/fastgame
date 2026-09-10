/**
 * Provably Fair + Deterministic Replay — mirrors pkg/prng/
 */

const MAX_UINT64 = 18446744073709551615n;
const WEATHER = ['clear', 'cloudy', 'rain', 'storm'];
const SPECIES = ['bass', 'trout', 'tuna', 'salmon', 'shark', 'marlin'];
const PROPS = ['worm', 'lure', 'fly', 'jig'];

export async function hashServerSeed(seed) {
    const enc = new TextEncoder();
    const buf = await crypto.subtle.digest('SHA-256', enc.encode(seed));
    return Array.from(new Uint8Array(buf)).map((b) => b.toString(16).padStart(2, '0')).join('');
}

export async function rollIndex(serverSeed, clientSeed, nonce, index = 0) {
    const payload = `${clientSeed}:${nonce}:${index}`;
    const enc = new TextEncoder();
    const key = await crypto.subtle.importKey('raw', enc.encode(serverSeed), { name: 'HMAC', hash: 'SHA-256' }, false, ['sign']);
    const sig = await crypto.subtle.sign('HMAC', key, enc.encode(payload));
    const bytes = new Uint8Array(sig);
    let n = 0n;
    for (let i = 0; i < 8; i++) n = (n << 8n) | BigInt(bytes[i]);
    return Number(n) / Number(MAX_UINT64);
}

export async function roll(serverSeed, clientSeed, nonce) {
    return rollIndex(serverSeed, clientSeed, nonce, 0);
}

function round2(v) {
    return Math.round(v * 100) / 100;
}

function pickIndex(r, n) {
    return Math.min(Math.max(Math.floor(r * n), 0), n - 1);
}

export async function computeOutcome(serverSeed, clientSeed, nonce, betAmount) {
    const primaryRoll = await roll(serverSeed, clientSeed, nonce);
    const multiRoll = await rollIndex(serverSeed, clientSeed, nonce, 1);
    if (primaryRoll < 0.55) {
        return { roll: primaryRoll, fishState: 'miss', animationKey: 'fish_miss', multiplier: 0, winAmount: 0 };
    }
    if (primaryRoll < 0.9) {
        const multiplier = round2(1.5 + multiRoll * 3.5);
        return { roll: primaryRoll, fishState: 'bite', animationKey: 'fish_bite_normal', multiplier, winAmount: round2(betAmount * multiplier) };
    }
    const multiplier = round2(50 + multiRoll * 50);
    return { roll: primaryRoll, fishState: 'big_win', animationKey: 'fish_bite_bigwin', multiplier, winAmount: round2(betAmount * multiplier) };
}

export async function computeReplayScene(serverSeed, clientSeed, nonce, betAmount) {
    const outcome = await computeOutcome(serverSeed, clientSeed, nonce, betAmount);
    const weatherRoll = await rollIndex(serverSeed, clientSeed, nonce, 2);
    const speciesRoll = await rollIndex(serverSeed, clientSeed, nonce, 3);
    const biteRoll = await rollIndex(serverSeed, clientSeed, nonce, 10);
    const castRoll = await rollIndex(serverSeed, clientSeed, nonce, 11);
    const speedRoll = await rollIndex(serverSeed, clientSeed, nonce, 12);

    const fishPath = [];
    for (let i = 0; i < 3; i++) {
        const xRoll = await rollIndex(serverSeed, clientSeed, nonce, 4 + i * 2);
        const yRoll = await rollIndex(serverSeed, clientSeed, nonce, 5 + i * 2);
        fishPath.push({ x: round2(xRoll), y: round2(yRoll) });
    }

    return {
        outcome,
        scene: {
            weather: WEATHER[pickIndex(weatherRoll, WEATHER.length)],
            fishSpecies: SPECIES[pickIndex(speciesRoll, SPECIES.length)],
            fishPath,
            biteProp: PROPS[pickIndex(biteRoll, PROPS.length)],
            castDurationMs: 600 + Math.floor(castRoll * 400),
            fishSpeed: round2(0.5 + speedRoll * 1.5),
        },
    };
}
