/**
 * Provably Fair 验算 — 与 pkg/prng/provably_fair.go + engine.go 算法完全一致
 *
 * 回归向量 (pkg/prng/provably_fair_browser_test.go):
 *   Roll("abc123seed","client456","round-test-1") → 0.75434720333408778
 *   HashServerSeed("abc123seed") → d7e67bf1b02ad2c15e860ce392748c14279854cf69a74df10ccf11495f343c02
 */

export {
    hashServerSeed,
    roll,
    rollIndex,
    computeOutcome,
    formatMinor,
    betToMinor,
    minorToMajor,
    MONEY_SCALE,
} from '/shared/prng-money.js';

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
