import { hashServerSeed, roll, computeOutcome, parseProvablyFairJson, formatMinor } from './fair.js';

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

function showToast(msg, type = 'success') {
    const el = $('#toast');
    el.textContent = msg;
    el.className = `toast ${type}`;
    clearTimeout(el._timer);
    el._timer = setTimeout(() => el.classList.add('hidden'), 2800);
}

function buildShareUrl() {
    const serverSeed = $('#server-seed').value.trim();
    const serverSeedHash = $('#server-seed-hash').value.trim();
    const clientSeed = $('#client-seed').value.trim();
    const nonce = $('#nonce').value.trim();
    const expectedRoll = $('#expected-roll').value.trim();
    const betAmount = $('#bet-amount').value.trim();

    if (!serverSeed || !clientSeed || !nonce) {
        return null;
    }

    const params = new URLSearchParams();
    params.set('serverSeed', serverSeed);
    params.set('clientSeed', clientSeed);
    params.set('nonce', nonce);
    if (serverSeedHash) params.set('serverSeedHash', serverSeedHash);
    if (expectedRoll) params.set('roll', expectedRoll);
    if (betAmount) params.set('betAmount', betAmount);

    return `${window.location.origin}/verify/?${params.toString()}`;
}

function updateSharePreview() {
    const url = buildShareUrl();
    const preview = $('#share-link-preview');
    if (!url) {
        preview.classList.add('hidden');
        preview.textContent = '';
        return;
    }
    preview.textContent = url;
    preview.classList.remove('hidden');
}

async function copyShareLink() {
    const url = buildShareUrl();
    if (!url) {
        showToast('请先填写 Server Seed、Client Seed 和 Nonce', 'error');
        return;
    }

    try {
        await navigator.clipboard.writeText(url);
    } catch {
        const ta = document.createElement('textarea');
        ta.value = url;
        ta.style.position = 'fixed';
        ta.style.left = '-9999px';
        document.body.appendChild(ta);
        ta.select();
        document.execCommand('copy');
        document.body.removeChild(ta);
    }

    window.history.replaceState({}, '', url);
    updateSharePreview();
    showToast('分享链接已复制，可直接发送给其他玩家验算');
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
        <h3>推导结果 (Bet ${betAmount}, minor=${formatMinor(outcome.betAmountMinor)})</h3>
        <dl>
          <dt>状态</dt><dd>${STATE_LABELS[outcome.fishState] || outcome.fishState}</dd>
          <dt>倍率</dt><dd>${outcome.multiplier}x (${outcome.multiplierMinor} minor)</dd>
          <dt>派彩</dt><dd>${outcome.winAmount} (${formatMinor(outcome.winAmountMinor)} minor)</dd>
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

阈值与 pkg/prng/engine.go 一致（uint64 整数 roll + Scale=10000 定点倍率）
miss: roll &lt; 55%
bite: 55%–90% → multiplier 15000–50000 minor (1.5x–5.0x)
big:  roll ≥ 90% → multiplier 500000–1000000 minor (50x–100x)
win = betMinor × multMinor / 10000</pre>
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
        updateSharePreview();
        setResult('<p class="ok">已从 JSON 导入字段</p>', 'success');
    } catch (err) {
        setResult(`<p class="error">JSON 解析失败: ${err.message}</p>`, 'error');
    }
});

$('#clear-btn').addEventListener('click', () => {
    $('#verify-form').reset();
    $('#json-import').value = '';
    updateSharePreview();
    setResult('<p class="muted">已清空</p>');
});

$('#copy-link-btn').addEventListener('click', () => copyShareLink());

['server-seed', 'server-seed-hash', 'client-seed', 'nonce', 'expected-roll', 'bet-amount'].forEach((id) => {
    $(`#${id}`).addEventListener('input', updateSharePreview);
});

fillFromQuery();
updateSharePreview();
