/**
 * Provably Fair + Deterministic Replay — mirrors pkg/prng/
 */

import {
    rollIndex,
    rollIndexUint64,
    computeOutcome,
    betToMinor,
    minorToMajor,
} from '/shared/prng-money.js';

const WEATHER = ['clear', 'cloudy', 'rain', 'storm'];
const SPECIES = ['bass', 'trout', 'tuna', 'salmon', 'shark', 'marlin'];
const PROPS = ['worm', 'lure', 'fly', 'jig'];

export { hashServerSeed, roll, rollIndex, computeOutcome } from '/shared/prng-money.js';

function pickIndex(r, n) {
    return Math.min(Math.max(Math.floor(r * n), 0), n - 1);
}

function round2(v) {
    return Math.round(v * 100) / 100;
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

/** 解析 API replay 响应中的 minor 下注额 */
export function betAmountFromReplay(data) {
    const raw = data?.replay?.inputs?.betAmount ?? data?.betAmount;
    if (raw == null) return 10;
    const n = Number(raw);
    if (Number.isInteger(n) && n >= 10000) return minorToMajor(n);
    return n;
}
