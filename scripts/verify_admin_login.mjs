// 后台登录页真浏览器冒烟检查
//
// 为什么需要它：后台是 SPA，接口全部 200、静态资源全部 200，也不代表页面能用。
// 本项目踩过一次：路由排序时 v.meta.rank 抛 TypeError，而 main.ts 给 window 挂了
// unhandledrejection 监听并 preventDefault()（本意是屏蔽 Element Plus 弹窗的
// cancel/close），结果异常既不出现在控制台也逃不到页面 —— 页面永远停在
// index.html 里那个 .loader 转圈上。HTTP 层完全看不出问题，只有真浏览器能发现。
//
// 用法（在仓库根目录）：
//   node scripts/verify_admin_login.mjs [URL]
// 默认 URL: http://sponge.localhost:18000/admin/
//
// 依赖：
//   - playwright-core（默认取 DeepSeekGUI 自带的，可用环境变量 PLAYWRIGHT_CORE 覆盖）
//   - 本机 Chrome（可用环境变量 CHROME_PATH 覆盖）
//   - 网关地址需带 Host: sponge.localhost（见 docker/nginx/sponge.conf），
//     直接用 127.0.0.1:18000 会命中默认 server，/admin/ 打不开

import { createRequire } from 'node:module';
import { existsSync } from 'node:fs';

const url = process.argv[2] || 'http://sponge.localhost:18000/admin/';
const pwPath =
    process.env.PLAYWRIGHT_CORE ||
    'C:\\Users\\94350\\AppData\\Local\\Programs\\DeepSeekGUI\\resources\\dsh\\node_modules\\playwright-core';
const chromePath =
    process.env.CHROME_PATH ||
    'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe';

if (!existsSync(pwPath)) {
    console.error(`找不到 playwright-core: ${pwPath}\n请设置环境变量 PLAYWRIGHT_CORE`);
    process.exit(2);
}
if (!existsSync(chromePath)) {
    console.error(`找不到 Chrome: ${chromePath}\n请设置环境变量 CHROME_PATH`);
    process.exit(2);
}

const require = createRequire(import.meta.url);
const { chromium } = require(pwPath);

const browser = await chromium.launch({
    executablePath: chromePath,
    headless: true,
    args: ['--no-sandbox', '--disable-dev-shm-usage'],
});
const ctx = await browser.newContext();
const page = await ctx.newPage();

// 抢在页面脚本之前挂记录器：应用自己的 unhandledrejection 监听会 preventDefault
await page.addInitScript(() => {
    window.__diag = { rejections: [], errors: [] };
    window.addEventListener('unhandledrejection', e => {
        const r = e.reason;
        window.__diag.rejections.push(r?.stack || String(r));
    });
    window.addEventListener('error', e =>
        window.__diag.errors.push(`${e.message} @ ${e.filename}:${e.lineno}`),
    );
});

const badRequests = [];
page.on('response', r => {
    if (r.status() >= 400) badRequests.push(`${r.status()} ${r.url()}`);
});

let failed = false;
const fail = msg => {
    failed = true;
    console.error(`✗ ${msg}`);
};

await page.goto(url, { waitUntil: 'domcontentloaded', timeout: 30000 });

// 1) 等登录表单真正渲染出来（而不是死等固定秒数）
let rendered = true;
try {
    await page.waitForSelector('input[placeholder="用户名"]', { timeout: 20000 });
} catch {
    rendered = false;
}
await page.waitForTimeout(1000);

const beforeLogin = await page.evaluate(() => ({
    title: document.title,
    // index.html 里的 .loader 还在 => Vue 从未 mount
    stuckOnLoader: !!document.querySelector('#app .loader'),
    hasPassword: !!document.querySelector('input[type="password"]'),
    buttons: Array.from(document.querySelectorAll('button')).map(b => (b.innerText || '').trim()),
}));

console.log('=== 登录页 ===');
console.log(JSON.stringify(beforeLogin, null, 2));
if (!rendered) fail('登录表单没有渲染出来');
if (beforeLogin.stuckOnLoader) fail('页面仍停在 index.html 的 loader 上（Vue 未 mount）');
if (!beforeLogin.hasPassword) fail('没有密码输入框');

if (failed) {
    await dumpDiagnostics(page);
    await page.screenshot({ path: 'admin_login_failed.png', fullPage: true });
    console.error('\n截图: admin_login_failed.png');
    await browser.close();
    process.exit(1);
}

// 2) 真登录一次，验证前后端约定（注意第一个 input 是隐藏的主题开关，按 placeholder 定位）
await page.locator('input[placeholder="用户名"]').fill('admin');
await page.locator('input[placeholder="密码"]').fill('admin123');
await page.locator('button:has-text("登录"), button[type="submit"]').first().click();

let leftLoginPage = true;
try {
    await page.waitForFunction(() => !location.hash.includes('/login'), { timeout: 20000 });
} catch {
    leftLoginPage = false;
}
await page.waitForTimeout(2500);

const afterLogin = await page.evaluate(() => ({
    hash: location.hash,
    title: document.title,
    stuckOnLoader: !!document.querySelector('#app .loader'),
    menuItems: Array.from(
        document.querySelectorAll('.el-menu-item span, .el-sub-menu__title span'),
    )
        .map(s => (s.innerText || '').trim())
        .filter(Boolean),
}));

console.log('\n=== 登录后 ===');
console.log(JSON.stringify(afterLogin, null, 2));
if (!leftLoginPage) fail('提交后仍停在 /login');
if (afterLogin.stuckOnLoader) fail('登录后页面仍停在 loader');
if (afterLogin.menuItems.length === 0) fail('侧边栏菜单为空（handleWholeMenus 可能又抛异常了）');

if (badRequests.length) {
    console.log('\n=== 4xx/5xx 请求 ===');
    console.log(badRequests.join('\n'));
}
await dumpDiagnostics(page);

if (failed) {
    await page.screenshot({ path: 'admin_login_failed.png', fullPage: true });
    console.error('\n截图: admin_login_failed.png');
    await browser.close();
    process.exit(1);
}

console.log('\n✓ 登录页渲染正常、登录成功、菜单已生成');
await browser.close();
process.exit(0);

async function dumpDiagnostics(page) {
    const diag = await page.evaluate(() => window.__diag || {});
    console.log('\n=== 未被捕获的 Promise 异常（应用 preventDefault 抹掉的那种）===');
    console.log(diag.rejections?.length ? diag.rejections.join('\n---\n') : '(无)');
    console.log('=== window error 事件 ===');
    console.log(diag.errors?.length ? diag.errors.join('\n') : '(无)');
}
