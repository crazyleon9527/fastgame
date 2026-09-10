import { computeReplayScene, betAmountFromReplay } from './fair.js';

const $ = (sel) => document.querySelector(sel);
const canvas = $('#stage');
const ctx = canvas.getContext('2d');
const BASE = window.location.origin;

const WEATHER_COLORS = {
    clear: ['#87CEEB', '#4a90d9'],
    cloudy: ['#b0bec5', '#78909c'],
    rain: ['#607d8b', '#37474f'],
    storm: ['#263238', '#1a237e'],
};

function drawBackground(weather) {
    const [top, bottom] = WEATHER_COLORS[weather] || WEATHER_COLORS.clear;
    const g = ctx.createLinearGradient(0, 0, 0, canvas.height);
    g.addColorStop(0, top);
    g.addColorStop(1, bottom);
    ctx.fillStyle = g;
    ctx.fillRect(0, 0, canvas.width, canvas.height);

    ctx.fillStyle = 'rgba(255,255,255,0.15)';
    ctx.fillRect(0, canvas.height * 0.72, canvas.width, canvas.height * 0.28);
}

function drawFish(x, y, species, scale = 1) {
    ctx.save();
    ctx.translate(x, y);
    ctx.scale(scale, scale);
    ctx.fillStyle = species === 'shark' ? '#546e7a' : '#ff7043';
    ctx.beginPath();
    ctx.ellipse(0, 0, 28, 14, 0, 0, Math.PI * 2);
    ctx.fill();
    ctx.beginPath();
    ctx.moveTo(-28, 0);
    ctx.lineTo(-40, -10);
    ctx.lineTo(-40, 10);
    ctx.closePath();
    ctx.fill();
    ctx.restore();
}

function wait(ms) {
    return new Promise((r) => setTimeout(r, ms));
}

async function animateReplay(scene, outcome) {
    drawBackground(scene.weather);
    ctx.fillStyle = '#fff';
    ctx.font = '14px sans-serif';
    ctx.fillText(`天气: ${scene.weather} | 鱼种: ${scene.fishSpecies} | 饵: ${scene.biteProp}`, 12, 24);

    // Cast phase
    ctx.strokeStyle = '#fff';
    ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.moveTo(80, canvas.height - 60);
    ctx.quadraticCurveTo(200, 80, canvas.width * 0.5, canvas.height * 0.45);
    ctx.stroke();
    await wait(scene.castDurationMs);

    // Fish path
    const w = canvas.width;
    const h = canvas.height;
    let px = w * 0.1;
    let py = h * 0.5;
    drawFish(px, py, scene.fishSpecies);
    const stepMs = 300 / scene.fishSpeed;

    for (const pt of scene.fishPath) {
        const nx = pt.x * w;
        const ny = pt.y * h;
        const frames = 20;
        for (let f = 1; f <= frames; f++) {
            drawBackground(scene.weather);
            ctx.fillStyle = '#fff';
            ctx.font = '14px sans-serif';
            ctx.fillText(`天气: ${scene.weather} | 鱼种: ${scene.fishSpecies} | 饵: ${scene.biteProp}`, 12, 24);
            const t = f / frames;
            const cx = px + (nx - px) * t;
            const cy = py + (ny - py) * t;
            drawFish(cx, cy, scene.fishSpecies);
            await wait(stepMs / frames);
        }
        px = nx;
        py = ny;
    }

    // Result
    drawBackground(scene.weather);
    const color = outcome.fishState === 'big_win' ? '#ffd54f' : outcome.fishState === 'bite' ? '#81c784' : '#ef9a9a';
    ctx.fillStyle = color;
    ctx.font = 'bold 22px sans-serif';
    ctx.fillText(`${outcome.animationKey} | ${outcome.multiplier}x | 赢 ${outcome.winAmount}`, 12, canvas.height - 24);
    drawFish(px, py, scene.fishSpecies, outcome.fishState === 'big_win' ? 1.8 : 1);
}

async function loadFromRoundId(roundId) {
    const resp = await fetch(`${BASE}/api/v1/game/replay/${encodeURIComponent(roundId)}`);
    if (!resp.ok) throw new Error(await resp.text());
    return resp.json();
}

async function runReplay() {
    const errEl = $('#replay-error');
    errEl.classList.add('hidden');
    try {
        const roundId = $('#round-id').value.trim();
        let serverSeed = $('#server-seed').value.trim();
        let clientSeed = $('#client-seed').value.trim();
        let nonce = $('#nonce').value.trim();
        let betAmount = parseFloat($('#bet-amount').value) || 10;

        if (roundId) {
            const data = await loadFromRoundId(roundId);
            serverSeed = data.replay.inputs.serverSeed;
            clientSeed = data.replay.inputs.clientSeed;
            nonce = data.replay.inputs.nonce;
            betAmount = betAmountFromReplay(data);
            $('#server-seed').value = serverSeed;
            $('#client-seed').value = clientSeed;
            $('#nonce').value = nonce;
            $('#bet-amount').value = betAmount;
            await animateReplay(data.replay.scene, {
                fishState: data.fishState,
                animationKey: data.animationKey,
                multiplier: data.multiplier,
                winAmount: data.winAmount,
            });
            return;
        }

        if (!serverSeed || !clientSeed || !nonce) {
            throw new Error('请填写 Round ID 或完整种子参数');
        }

        const { scene, outcome } = await computeReplayScene(serverSeed, clientSeed, nonce, betAmount);
        await animateReplay(scene, outcome);
    } catch (err) {
        errEl.textContent = err.message;
        errEl.classList.remove('hidden');
    }
}

$('#replay-btn').addEventListener('click', runReplay);

const params = new URLSearchParams(window.location.search);
if (params.get('roundId')) {
    $('#round-id').value = params.get('roundId');
    runReplay();
}
