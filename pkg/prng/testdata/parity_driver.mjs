// 客户端验算口径比对驱动（由 pkg/par/js_parity_test.go 通过 go:embed 取出并交给 node 执行）
//
// 用法: node parity_driver.mjs <prng-money.js 绝对路径> <cases.json 绝对路径>
//
// 逐局用 web/shared/prng-money.js 复现服务端给出的倍率与派彩，任一不一致即失败。
import { pathToFileURL } from 'node:url';
import { readFileSync } from 'node:fs';

const [, , modulePath, casesPath] = process.argv;
const mod = await import(pathToFileURL(modulePath).href);
const cases = JSON.parse(readFileSync(casesPath, 'utf8'));

const theoretical = mod.theoreticalRTP();
if (Math.abs(theoretical - 0.96) > 1e-12) {
    console.error(`JS 侧 PAR 表 RTP=${theoretical}，期望 0.96 —— 表已与服务端漂移`);
    process.exit(2);
}

const failures = [];
for (const c of cases) {
    const got = await mod.computeOutcome('parity-server-seed', 'parity-client-seed', c.nonce, c.betMinor);
    if (got.multiplierMinor !== c.multiplierMinor || got.winAmountMinor !== c.winMinor) {
        failures.push(
            `nonce=${c.nonce} roll=${c.roll} JS(mult=${got.multiplierMinor},win=${got.winAmountMinor}) ` +
                `Go(mult=${c.multiplierMinor},win=${c.winMinor})`,
        );
    }
}

if (failures.length > 0) {
    console.error(`客户端验算与服务端不一致（${failures.length}/${cases.length}）：`);
    for (const f of failures.slice(0, 10)) console.error('  ' + f);
    process.exit(1);
}
console.log(`JS/Go 口径一致，比对 ${cases.length} 局`);
