import { hashServerSeed, roll, computeOutcome, parseProvablyFairJson } from './fair.js';

const $ = (sel) => document.querySelector(sel);

const STATE_LABELS = {
    miss: '未咬钩 (miss)',
    bite: '普通咬钩 (bite)',
    big_win: '大奖 (big_win)',
};

function setResult(html, type = '') {
    const el = $('#verify-result');
    el.innerHTML = html;
    el.className = `result-card ${type}`;
}

function almostEqual(a, b, eps = 1e-12) {
    if (a === '' || b === '' || a == null || b == null) return null;
    return Math.abs(Number(a) - Number(b)) < eps;
}

async function verify() {
    const serverSeed = $('#server-seed').value.trim();
    const serverSeedHash = $('#server-seed-hash').value.trim();
    const clientSeed = $('#client-seed').value.trim();
    const nonce = $('#nonce').value.trim();
    const betAmount = parseFloat($('#bet-amount').value) || 0;
    const expectedRoll = $('#expected-roll').value.trim();

    if (!serverSeed || !clientSeed || !nonce) {
        setResult('<p class="error">请填写 Server Seed、Client Seed 和 Nonce (Round ID)</p>', 'error');
        return;
    }

    const computedRoll = await roll(serverSeed, clientSeed, nonce);
    const computedHash = await hashServerSeed(serverSeed);
    const hashOk = !serverSeedHash || serverSeedHash.toLowerCase() === computedHash;
    const rollOk = expectedRoll === '' ? null : almostEqual(expectedRoll, computedRoll);

    let outcomeHtml = '';
    if (betAmount > 0) {
        const outcome = await computeOutcome(serverSeed, clientSeed, nonce, betAmount);
        outcomeHtml = `
      <div class="outcome-box">
        <h3>推导结果 (Bet ${betAmount})</h3>
        <dl>
          <dt>状态</dt><dd>${STATE_LABELS[outcome.fishState] || outcome.fishState}</dd>
          <dt>倍率</dt><dd>${outcome.multiplier}x</dd>
          <dt>派彩</dt><dd>${outcome.winAmount}</dd>
          <dt>动画 Key</dt><dd><code>${outcome.animationKey}</code></dd>
        </dl>
      </div>`;
    }

    const rollMatchHtml = rollOk === null
        ? '<p class="muted">未填写期望 Roll，跳过对比</p>'
        : rollOk
            ? '<p class="ok">✓ Roll 与服务端返回值一致</p>'
            : '<p class="error">✗ Roll 不匹配 — 请检查种子或 Nonce</p>';

    setResult(`
    <h3>验算结果</h3>
    <dl class="result-dl">
      <dt>Server Seed Hash</dt>
      <dd><code>${computedHash}</code></dd>
      <dt>Hash 校验</dt>
      <dd>${hashOk ? '<span class="ok">✓ 通过</span>' : '<span class="error">✗ 与填写的 Hash 不符</span>'}</dd>
      <dt>计算 Roll</dt>
      <dd><code>${computedRoll.toFixed(16)}</code></dd>
      <dt>Roll 对比</dt>
      <dd>${rollMatchHtml}</dd>
    </dl>
    ${outcomeHtml}
    <details class="formula">
      <summary>算法说明</summary>
      <pre>HMAC-SHA256(key=serverSeed, msg=clientSeed:nonce:0)
Roll = BigEndianUint64(first 8 bytes) / (2⁶⁴−1)

miss:  roll &lt; 0.55
bite:  0.55 ≤ roll &lt; 0.90  → multiplier = 1.5 + RollIndex(1) × 3.5
big:   roll ≥ 0.90          → multiplier = 50 + RollIndex(1) × 50</pre>
    </details>
  `, hashOk && rollOk !== false ? 'success' : rollOk === false || !hashOk ? 'error' : 'success');
}

function fillFromQuery() {
    const q = new URLSearchParams(window.location.search);
    const fields = [
        ['serverSeed', 'server-seed'],
        ['serverSeedHash', 'server-seed-hash'],
        ['clientSeed', 'client-seed'],
        ['nonce', 'nonce'],
        ['betAmount', 'bet-amount'],
        ['roll', 'expected-roll'],
    ];
    fields.forEach(([param, id]) => {
        const v = q.get(param);
        if (v) $(`#${id}`).value = v;
    });
}

$('#verify-form').addEventListener('submit', (e) => {
    e.preventDefault();
    verify();
});

$('#import-json-btn').addEventListener('click', () => {
    try {
        const parsed = parseProvablyFairJson($('#json-import').value.trim());
        if (parsed.serverSeed) $('#server-seed').value = parsed.serverSeed;
        if (parsed.serverSeedHash) $('#server-seed-hash').value = parsed.serverSeedHash;
        if (parsed.clientSeed) $('#client-seed').value = parsed.clientSeed;
        if (parsed.nonce) $('#nonce').value = parsed.nonce;
        if (parsed.roll !== '') $('#expected-roll').value = String(parsed.roll);
        if (parsed.betAmount !== '') $('#bet-amount').value = String(parsed.betAmount);
        setResult('<p class="ok">已从 JSON 导入字段</p>', 'success');
    } catch (err) {
        setResult(`<p class="error">JSON 解析失败: ${err.message}</p>`, 'error');
    }
});

$('#clear-btn').addEventListener('click', () => {
    $('#verify-form').reset();
    $('#json-import').value = '';
    setResult('<p class="muted">已清空</p>');
});

fillFromQuery();
