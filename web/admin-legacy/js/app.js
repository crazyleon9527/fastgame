(() => {
  const TYPE_LABELS = { ip: 'IP', user_id: '用户 ID', merchant: '商户' };
  const ROLE_LABELS = { admin: '超管', operator: '运维', viewer: '只读' };
  const TAB_I18N_KEYS = {
    blacklist: 'nav.blacklist',
    merchants: 'nav.merchants',
    gameconfig: 'nav.games',
    rtpreport: 'nav.rtp',
    settlement: 'nav.settlements',
    whitelist: 'nav.whitelist',
    trace: 'nav.trace',
    alerts: 'nav.alerts',
    users: 'nav.users',
  };
  const TAB_TITLES = {
    blacklist: '风控黑名单',
    merchants: '商户管理',
    gameconfig: '游戏配置',
    rtpreport: 'RTP 报表',
    settlement: '对账看板',
    whitelist: 'IP 白名单',
    trace: 'Trace 追踪',
    alerts: 'RTP 风控告警',
    users: '用户管理',
  };

  function tabTitle(tab) {
    const key = TAB_I18N_KEYS[tab];
    if (key && typeof AdminI18n !== 'undefined') {
      return AdminI18n.t(key, TAB_TITLES[tab] || tab);
    }
    return TAB_TITLES[tab] || tab;
  }

  let activeTab = 'blacklist';

  // Blacklist state
  let blPage = 1;
  const blPageSize = 20;
  let blTotal = 0;
  let pendingDeleteId = null;

  // Merchant state
  let merchantPage = 1;
  const merchantPageSize = 20;
  let merchantTotal = 0;
  let pendingRotateId = null;
  let pendingRotateLabel = '';
  let pendingEditMerchantId = null;
  let pendingEditMerchantCode = '';
  let gcMerchantId = null;
  let gcEditMode = false;
  let gcEditKey = '';
  let merchantOptionsCache = [];
  const gcConfigCache = new Map();
  let rtpReportCache = [];
  let rtpPage = 1;
  const rtpPageSize = 50;
  let settlePage = 1;
  const settlePageSize = 20;
  let settleTotal = 0;
  let usersPage = 1;
  const usersPageSize = 20;
  let usersTotal = 0;
  let pendingEditUserId = null;
  let rolesCache = [];

  // Whitelist state
  let whitelistPage = 1;
  const whitelistPageSize = 20;
  let whitelistTotal = 0;
  let pendingWhitelistId = null;
  let pendingWhitelistLabel = '';
  const whitelistIPCache = new Map();

  const $ = (sel) => document.querySelector(sel);

  function show(view) {
    $('#login-view').classList.toggle('hidden', view !== 'login');
    $('#main-view').classList.toggle('hidden', view !== 'main');
  }

  function switchTab(tab) {
    activeTab = tab;
    document.querySelectorAll('.nav-item').forEach((el) => {
      el.classList.toggle('active', el.dataset.tab === tab);
    });
    $('#panel-blacklist').classList.toggle('hidden', tab !== 'blacklist');
    $('#panel-merchants').classList.toggle('hidden', tab !== 'merchants');
    $('#panel-gameconfig').classList.toggle('hidden', tab !== 'gameconfig');
    $('#panel-rtpreport').classList.toggle('hidden', tab !== 'rtpreport');
    $('#panel-settlement').classList.toggle('hidden', tab !== 'settlement');
    $('#panel-whitelist').classList.toggle('hidden', tab !== 'whitelist');
    $('#panel-trace').classList.toggle('hidden', tab !== 'trace');
    $('#panel-alerts').classList.toggle('hidden', tab !== 'alerts');
    $('#panel-users').classList.toggle('hidden', tab !== 'users');
    $('#page-subtitle').textContent = tabTitle(tab);

    if (tab === 'blacklist') loadBlacklist();
    if (tab === 'merchants') loadMerchants();
    if (tab === 'gameconfig') initGameConfigTab();
    if (tab === 'rtpreport') initRtpReportTab();
    if (tab === 'settlement') initSettlementTab();
    if (tab === 'whitelist') loadWhitelist();
    if (tab === 'alerts') loadRiskAlerts();
    if (tab === 'users') loadAdminUsers();
  }

  async function ensureRolesCache() {
    if (rolesCache.length) return rolesCache;
    const data = await AdminAPI.listRoles();
    rolesCache = data.list || [];
    return rolesCache;
  }

  function fillRoleSelect(selectEl, selectedId = '') {
    selectEl.innerHTML = rolesCache.map((r) => {
      const sel = String(selectedId) === String(r.id) ? ' selected' : '';
      return `<option value="${r.id}"${sel}>${escapeHtml(r.name)} — ${escapeHtml(r.description || '')}</option>`;
    }).join('');
  }

  async function loadMerchantOptions() {
    if (merchantOptionsCache.length) return merchantOptionsCache;
    const data = await AdminAPI.listMerchants({ page: 1, pageSize: 200 });
    merchantOptionsCache = data.list || [];
    return merchantOptionsCache;
  }

  function fillMerchantSelect(selectEl, { includeAll = false, selectedId = '' } = {}) {
    const opts = includeAll
      ? ['<option value="">全部商户</option>']
      : ['<option value="">请选择商户</option>'];
    merchantOptionsCache.forEach((m) => {
      const sel = String(selectedId) === String(m.id) ? ' selected' : '';
      opts.push(`<option value="${m.id}"${sel}>${escapeHtml(m.merchantCode)} — ${escapeHtml(m.name)}</option>`);
    });
    selectEl.innerHTML = opts.join('');
  }

  function formatMoneyMinor(minor) {
    if (minor == null || minor === '') return '—';
    const n = Number(minor);
    if (!Number.isFinite(n)) return String(minor);
    const whole = Math.trunc(n / 10000);
    const frac = Math.abs(n % 10000);
    return `${whole}.${String(frac).padStart(4, '0')}`;
  }

  async function loadRiskAlerts() {
    const tbody = $('#alerts-tbody');
    tbody.innerHTML = '<tr><td colspan="9" class="empty">加载中…</td></tr>';
    try {
      const data = await AdminAPI.listRiskAlerts(50);
      const list = data.list || [];
      tbody.innerHTML = list.length ? list.map((a) => `
        <tr>
          <td>${formatTime(a.createdAt)}</td>
          <td>${escapeHtml(a.scopeType)}</td>
          <td><code>${escapeHtml(a.scopeValue)}</code></td>
          <td>${escapeHtml(a.gameCode || '—')}</td>
          <td>${(a.rtpPpm / 10000).toFixed(1)}%</td>
          <td>${a.sampleSize}</td>
          <td>${escapeHtml(a.actionTaken)}</td>
          <td><span class="badge inactive">${escapeHtml(a.status)}</span></td>
          <td><button class="btn btn-sm" data-perm="write" data-ack-alert="${a.id}">确认处理</button></td>
        </tr>`).join('') : '<tr><td colspan="9" class="empty">暂无 open 告警</td></tr>';

      tbody.querySelectorAll('[data-ack-alert]').forEach((btn) => {
        btn.addEventListener('click', async () => {
          try {
            await AdminAPI.ackRiskAlert(btn.dataset.ackAlert);
            toast('告警已确认，挂起/标记已清除');
            await loadRiskAlerts();
          } catch (err) {
            toast(err.message, 'error');
          }
        });
      });
      applyRoleUI();
    } catch (err) {
      tbody.innerHTML = `<tr><td colspan="9" class="empty error">${escapeHtml(err.message)}</td></tr>`;
      handleAuthError(err);
    }
  }

  function renderTraceResult(data, label) {
    const spans = data.spans || [];
    const txs = data.pendingTransactions || [];
    const traceId = data.traceId || label;
    return `
        <h3>${escapeHtml(label)} · Trace: <code>${escapeHtml(traceId)}</code>${data.roundId ? ` · Round: <code>${escapeHtml(data.roundId)}</code>` : ''}</h3>
        <h4>网络 I/O Spans (${spans.length})</h4>
        <div class="table-wrap"><table>
          <thead><tr><th>时间</th><th>服务</th><th>操作</th><th>Round</th><th>状态</th><th>耗时</th><th>详情</th></tr></thead>
          <tbody>${spans.length ? spans.map((s) => `
            <tr>
              <td>${formatTime(s.occurredAt)}</td>
              <td>${escapeHtml(s.service)}</td>
              <td><code>${escapeHtml(s.operation)}</code></td>
              <td><code>${escapeHtml(s.roundId || '—')}</code></td>
              <td><span class="badge ${s.status === 'ok' ? 'active' : 'inactive'}">${escapeHtml(s.status)}</span></td>
              <td>${s.durationMs}ms</td>
              <td>${escapeHtml(s.detail || '—')}</td>
            </tr>`).join('') : '<tr><td colspan="7" class="empty">暂无 Span 记录</td></tr>'}
          </tbody>
        </table></div>
        <h4>孤儿注单补偿 (${txs.length})</h4>
        <div class="table-wrap"><table>
          <thead><tr><th>Round</th><th>阶段</th><th>状态</th><th>下注</th><th>派彩</th><th>期望动作</th><th>重试</th></tr></thead>
          <tbody>${txs.length ? txs.map((t) => `
            <tr>
              <td><code>${escapeHtml(t.roundId)}</code></td>
              <td>${escapeHtml(t.phase)}</td>
              <td>${escapeHtml(t.status)}</td>
              <td>${formatMoneyMinor(t.betAmount)}</td>
              <td>${formatMoneyMinor(t.winAmount)}</td>
              <td>${escapeHtml(t.expectedAction)}</td>
              <td>${t.retryCount}</td>
            </tr>`).join('') : '<tr><td colspan="7" class="empty">无 pending_transactions</td></tr>'}
          </tbody>
        </table></div>`;
  }

  async function lookupTrace() {
    const traceId = $('#trace-id-input').value.trim();
    const box = $('#trace-result');
    if (!traceId) {
      box.innerHTML = '<p class="empty error">请输入 Trace ID</p>';
      return;
    }
    box.innerHTML = '<p class="empty">查询中…</p>';
    try {
      const data = await AdminAPI.lookupTrace(traceId);
      box.innerHTML = renderTraceResult(data, `Trace 查询`);
    } catch (err) {
      box.innerHTML = `<p class="empty error">${escapeHtml(err.message)}</p>`;
      handleAuthError(err);
    }
  }

  async function lookupTraceByRound() {
    const roundId = $('#trace-round-input').value.trim();
    const box = $('#trace-result');
    if (!roundId) {
      box.innerHTML = '<p class="empty error">请输入 Round ID</p>';
      return;
    }
    box.innerHTML = '<p class="empty">查询中…</p>';
    try {
      const data = await AdminAPI.lookupTraceByRound(roundId);
      box.innerHTML = renderTraceResult(data, `Round 查询`);
    } catch (err) {
      box.innerHTML = `<p class="empty error">${escapeHtml(err.message)}</p>`;
      handleAuthError(err);
    }
  }

  function toast(msg, type = 'success') {
    const el = $('#toast');
    el.textContent = msg;
    el.className = `toast ${type}`;
    clearTimeout(el._timer);
    el._timer = setTimeout(() => el.classList.add('hidden'), 3000);
  }

  function formatTime(ts) {
    if (!ts) return '—';
    const d = typeof ts === 'number' ? new Date(ts * 1000) : new Date(ts);
    return d.toLocaleString('zh-CN', { hour12: false });
  }

  function toRFC3339(localDatetime) {
    if (!localDatetime) return '';
    return new Date(localDatetime).toISOString();
  }

  function escapeHtml(s) {
    return String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
  }

  function escapeAttr(s) {
    return String(s).replace(/"/g, '&quot;');
  }

  function roleLabel(role) {
    return ROLE_LABELS[role] || role;
  }

  function applyRoleUI() {
    const role = AdminAPI.getRole();
    const canWrite = AdminAPI.canWrite();
    const isAdmin = AdminAPI.isAdmin();
    const roleEl = $('#role-label');
    if (roleEl) {
      roleEl.textContent = roleLabel(role);
      roleEl.className = `role-badge role-${role}`;
    }
    document.querySelectorAll('[data-perm="write"]').forEach((el) => {
      el.classList.toggle('hidden', !canWrite);
    });
    document.querySelectorAll('[data-perm="admin"]').forEach((el) => {
      el.classList.toggle('hidden', !isAdmin);
    });
  }

  function maybeRequireTotpSetup() {
    if (!AdminAPI.isLoggedIn() || !AdminAPI.isTotpPending()) return;
    toast('请先完成 2FA 绑定', 'error');
    $('#totp-setup-btn').click();
  }

  function handleAuthError(err) {
    if (err.message.includes('登录')) {
      AdminAPI.clearSession();
      show('login');
      return;
    }
    if (err.message.includes('2FA') || err.message.toLowerCase().includes('totp')) {
      maybeRequireTotpSetup();
      return;
    }
    if (err.message.includes('权限不足')) {
      toast(err.message, 'error');
    }
  }

  // ── Blacklist ──────────────────────────────────────────

  async function loadBlacklist() {
    const listType = $('#filter-type').value;
    const tbody = $('#blacklist-body');
    tbody.innerHTML = '<tr><td colspan="8" class="empty">加载中…</td></tr>';

    try {
      const data = await AdminAPI.listBlacklist({ listType, page: blPage, pageSize: blPageSize });
      blTotal = data.total || 0;
      $('#stat-count').textContent = (data.list || []).length;
      $('#stat-total').textContent = blTotal;
      $('#page-info').textContent = `第 ${blPage} 页 / 共 ${Math.max(1, Math.ceil(blTotal / blPageSize))} 页`;
      $('#prev-page').disabled = blPage <= 1;
      $('#next-page').disabled = blPage * blPageSize >= blTotal;

      if (!data.list || data.list.length === 0) {
        tbody.innerHTML = '<tr><td colspan="8" class="empty">暂无封禁记录</td></tr>';
        return;
      }

      tbody.innerHTML = data.list.map((row) => `
        <tr>
          <td>${row.id}</td>
          <td><span class="badge type-${row.listType}">${TYPE_LABELS[row.listType] || row.listType}</span></td>
          <td><code>${escapeHtml(row.listValue)}</code></td>
          <td>${escapeHtml(row.reason || '—')}</td>
          <td><span class="badge ${row.status === 1 ? 'active' : 'inactive'}">${row.status === 1 ? '生效' : '已解除'}</span></td>
          <td>${row.expiresAt ? formatTime(row.expiresAt) : '永久'}</td>
          <td>${formatTime(row.createdAt)}</td>
          <td>
            ${row.status === 1 ? `<button class="btn danger btn-sm" data-perm="write" data-delete="${row.id}" data-value="${escapeAttr(row.listValue)}">解除</button>` : '—'}
          </td>
        </tr>
      `).join('');

      tbody.querySelectorAll('[data-delete]').forEach((btn) => {
        btn.addEventListener('click', () => {
          pendingDeleteId = btn.dataset.delete;
          $('#confirm-text').textContent = `确定解除封禁「${btn.dataset.value}」？`;
          $('#confirm-dialog').showModal();
        });
      });
      applyRoleUI();
    } catch (err) {
      tbody.innerHTML = `<tr><td colspan="8" class="empty error">${escapeHtml(err.message)}</td></tr>`;
      handleAuthError(err);
    }
  }

  // ── Merchants ──────────────────────────────────────────

  async function loadOpenBreakers() {
    const panel = $('#breaker-panel');
    const listEl = $('#breaker-list');
    try {
      const data = await AdminAPI.listWalletBreakers();
      const list = (data.list || []).filter((b) => b.open);
      if (!list.length) {
        panel.classList.add('hidden');
        return;
      }
      panel.classList.remove('hidden');
      listEl.innerHTML = list.map((b) => `
        <li><code>${escapeHtml(b.merchantCode)}</code>
          ${b.overridden ? ' (override)' : ''}
          · ${b.openedAt ? formatTime(b.openedAt) : '—'}
        </li>`).join('');
    } catch {
      panel.classList.add('hidden');
    }
  }

  async function loadMerchants() {
    const tbody = $('#merchant-body');
    tbody.innerHTML = '<tr><td colspan="5" class="empty">加载中…</td></tr>';
    loadOpenBreakers();

    try {
      const data = await AdminAPI.listMerchants({ page: merchantPage, pageSize: merchantPageSize });
      merchantTotal = data.total || 0;
      const list = data.list || [];

      $('#merchant-stat-total').textContent = merchantTotal;
      $('#merchant-stat-active').textContent = list.filter((m) => m.status === 1).length;
      $('#merchant-page-info').textContent = `第 ${merchantPage} 页 / 共 ${Math.max(1, Math.ceil(merchantTotal / merchantPageSize))} 页`;
      $('#merchant-prev-page').disabled = merchantPage <= 1;
      $('#merchant-next-page').disabled = merchantPage * merchantPageSize >= merchantTotal;

      if (list.length === 0) {
        tbody.innerHTML = '<tr><td colspan="5" class="empty">暂无商户</td></tr>';
        return;
      }

      tbody.innerHTML = list.map((m) => `
        <tr>
          <td>${m.id}</td>
          <td><code>${escapeHtml(m.merchantCode)}</code></td>
          <td>${escapeHtml(m.name)}</td>
          <td><span class="badge ${m.status === 1 ? 'active' : 'inactive'}">${m.status === 1 ? '启用' : '禁用'}</span></td>
          <td class="actions-cell">
            <button class="btn btn-sm" data-perm="write" data-edit-merchant="${m.id}" data-code="${escapeAttr(m.merchantCode)}" data-name="${escapeAttr(m.name)}" data-status="${m.status}">编辑</button>
            <button class="btn btn-sm" data-perm="admin" data-rotate="${m.id}" data-code="${escapeAttr(m.merchantCode)}" data-name="${escapeAttr(m.name)}" ${m.status !== 1 ? 'disabled' : ''}>
              轮换密钥
            </button>
            <button class="btn btn-sm" data-perm="write" data-reset-breaker="${escapeAttr(m.merchantCode)}">解除熔断</button>
          </td>
        </tr>
      `).join('');

      tbody.querySelectorAll('[data-edit-merchant]').forEach((btn) => {
        btn.addEventListener('click', () => {
          pendingEditMerchantId = btn.dataset.editMerchant;
          pendingEditMerchantCode = btn.dataset.code;
          $('#edit-merchant-label').textContent = `商户编码：${btn.dataset.code}`;
          $('#edit-merchant-name').value = btn.dataset.name;
          $('#edit-merchant-status').value = String(btn.dataset.status) === '1' ? '1' : '2';
          $('#edit-merchant-error').classList.add('hidden');
          $('#edit-merchant-dialog').showModal();
        });
      });

      tbody.querySelectorAll('[data-rotate]').forEach((btn) => {
        btn.addEventListener('click', () => {
          pendingRotateId = btn.dataset.rotate;
          pendingRotateLabel = `${btn.dataset.code} (${btn.dataset.name})`;
          $('#rotate-merchant-label').textContent = `商户：${pendingRotateLabel}`;
          $('#rotate-grace-hours').value = '24';
          $('#rotate-error').classList.add('hidden');
          $('#rotate-dialog').showModal();
        });
      });

      tbody.querySelectorAll('[data-reset-breaker]').forEach((btn) => {
        btn.addEventListener('click', async () => {
          const code = btn.dataset.resetBreaker;
          if (!confirm(`确定解除商户 ${code} 的钱包熔断？`)) return;
          try {
            await AdminAPI.resetWalletBreaker(code);
            toast(`已解除 ${code} 熔断（10 分钟 override）`);
          } catch (err) {
            toast(err.message, 'error');
          }
        });
      });
      applyRoleUI();
    } catch (err) {
      tbody.innerHTML = `<tr><td colspan="5" class="empty error">${escapeHtml(err.message)}</td></tr>`;
      handleAuthError(err);
    }
  }

  // ── Whitelist ──────────────────────────────────────────

  async function loadWhitelist() {
    const tbody = $('#whitelist-body');
    tbody.innerHTML = '<tr><td colspan="5" class="empty">加载中…</td></tr>';

    try {
      const data = await AdminAPI.listMerchants({ page: whitelistPage, pageSize: whitelistPageSize });
      whitelistTotal = data.total || 0;
      const list = data.list || [];

      $('#whitelist-page-info').textContent = `第 ${whitelistPage} 页 / 共 ${Math.max(1, Math.ceil(whitelistTotal / whitelistPageSize))} 页`;
      $('#whitelist-prev-page').disabled = whitelistPage <= 1;
      $('#whitelist-next-page').disabled = whitelistPage * whitelistPageSize >= whitelistTotal;

      if (list.length === 0) {
        tbody.innerHTML = '<tr><td colspan="5" class="empty">暂无商户</td></tr>';
        return;
      }

      await Promise.all(list.map(async (m) => {
        if (!whitelistIPCache.has(m.id)) {
          try {
            const detail = await AdminAPI.getMerchantAllowedIPs(m.id);
            whitelistIPCache.set(m.id, detail.allowedIps || []);
          } catch {
            whitelistIPCache.set(m.id, null);
          }
        }
      }));

      tbody.innerHTML = list.map((m) => {
        const ips = whitelistIPCache.get(m.id);
        const countLabel = ips === null ? '—' : (ips.length === 0 ? '不限制' : String(ips.length));
        return `
        <tr>
          <td>${m.id}</td>
          <td><code>${escapeHtml(m.merchantCode)}</code></td>
          <td>${escapeHtml(m.name)}</td>
          <td>${countLabel}</td>
          <td>
            <button class="btn btn-sm" data-perm="write" data-edit-whitelist="${m.id}" data-code="${escapeAttr(m.merchantCode)}" data-name="${escapeAttr(m.name)}">
              编辑白名单
            </button>
          </td>
        </tr>
      `;
      }).join('');

      applyRoleUI();
      tbody.querySelectorAll('[data-edit-whitelist]').forEach((btn) => {
        btn.addEventListener('click', async () => {
          pendingWhitelistId = btn.dataset.editWhitelist;
          pendingWhitelistLabel = `${btn.dataset.code} (${btn.dataset.name})`;
          $('#whitelist-merchant-label').textContent = `商户：${pendingWhitelistLabel}`;
          $('#whitelist-error').classList.add('hidden');
          $('#whitelist-ips').value = '加载中…';
          $('#whitelist-dialog').showModal();
          try {
            const detail = await AdminAPI.getMerchantAllowedIPs(pendingWhitelistId);
            whitelistIPCache.set(pendingWhitelistId, detail.allowedIps || []);
            $('#whitelist-ips').value = (detail.allowedIps || []).join('\n');
          } catch (err) {
            $('#whitelist-ips').value = '';
            $('#whitelist-error').textContent = err.message;
            $('#whitelist-error').classList.remove('hidden');
          }
        });
      });
    } catch (err) {
      tbody.innerHTML = `<tr><td colspan="5" class="empty error">${escapeHtml(err.message)}</td></tr>`;
      handleAuthError(err);
    }
  }

  function showKeyResult(resp, kind = 'rotate') {
    if (kind === 'create') {
      $('#key-result-title').textContent = '商户已创建 — 私钥仅显示一次';
      $('#new-key-value').textContent = resp.privateKey;
      $('#key-meta').innerHTML = `
        <li>商户编码：<code>${escapeHtml(resp.merchantCode)}</code></li>
        <li>商户名称：${escapeHtml(resp.name)}</li>
      `;
    } else {
      $('#key-result-title').textContent = '新密钥已生成';
      $('#new-key-value').textContent = resp.newPrivateKey;
      $('#key-meta').innerHTML = `
        <li>商户编码：<code>${escapeHtml(resp.merchantCode)}</code></li>
        <li>过渡期：<strong>${resp.gracePeriodHours}</strong> 小时</li>
        <li>轮换时间：${formatTime(resp.rotatedAt)}</li>
      `;
    }
    $('#key-result-dialog').showModal();
  }

  // ── Game Config ────────────────────────────────────────

  async function initGameConfigTab() {
    try {
      await loadMerchantOptions();
      fillMerchantSelect($('#gc-merchant-select'), { selectedId: gcMerchantId || '' });
      if (gcMerchantId) await loadGameConfigs();
    } catch (err) {
      $('#gc-body').innerHTML = `<tr><td colspan="7" class="empty error">${escapeHtml(err.message)}</td></tr>`;
      handleAuthError(err);
    }
  }

  async function loadGameConfigs() {
    const merchantId = $('#gc-merchant-select').value;
    gcMerchantId = merchantId || null;
    $('#gc-add-btn').disabled = !merchantId;
    const tbody = $('#gc-body');
    if (!merchantId) {
      tbody.innerHTML = '<tr><td colspan="7" class="empty">请选择商户后加载</td></tr>';
      return;
    }

    tbody.innerHTML = '<tr><td colspan="7" class="empty">加载中…</td></tr>';
    const gameCode = $('#gc-game-code').value.trim();
    try {
      const data = await AdminAPI.listGameConfigs({ merchantId, gameCode });
      const list = data.list || [];
      if (!list.length) {
        tbody.innerHTML = '<tr><td colspan="7" class="empty">暂无配置，点击「新增配置」添加</td></tr>';
        return;
      }
      gcConfigCache.clear();
      tbody.innerHTML = list.map((row) => {
        gcConfigCache.set(String(row.id), row);
        return `
        <tr>
          <td>${row.id}</td>
          <td><code>${escapeHtml(row.gameCode)}</code></td>
          <td><code>${escapeHtml(row.configKey)}</code></td>
          <td><pre class="json-preview">${escapeHtml(formatJsonPreview(row.configValue))}</pre></td>
          <td>${escapeHtml(row.rtpTier || '—')}</td>
          <td><span class="badge ${row.status === 1 ? 'active' : 'inactive'}">${row.status === 1 ? '启用' : '禁用'}</span></td>
          <td>
            <button class="btn btn-sm" data-perm="write" data-gc-edit="${row.id}">编辑</button>
            <button class="btn btn-sm danger" data-perm="write" data-gc-delete="${row.id}">删除</button>
          </td>
        </tr>`;
      }).join('');

      tbody.querySelectorAll('[data-gc-edit]').forEach((btn) => {
        btn.addEventListener('click', () => {
          const row = gcConfigCache.get(btn.dataset.gcEdit);
          if (row) openGameConfigDialog(row, true);
        });
      });
      tbody.querySelectorAll('[data-gc-delete]').forEach((btn) => {
        btn.addEventListener('click', async () => {
          const row = gcConfigCache.get(btn.dataset.gcDelete);
          if (!row || !confirm(`删除配置 ${row.gameCode}/${row.configKey}？`)) return;
          try {
            await AdminAPI.deleteGameConfig(row.id);
            await loadGameConfigs();
          } catch (err) {
            alert(err.message);
            handleAuthError(err);
          }
        });
      });
      applyRoleUI();
    } catch (err) {
      tbody.innerHTML = `<tr><td colspan="7" class="empty error">${escapeHtml(err.message)}</td></tr>`;
      handleAuthError(err);
    }
  }

  function formatJsonPreview(raw) {
    try {
      return JSON.stringify(JSON.parse(raw), null, 2);
    } catch {
      return raw;
    }
  }

  function openGameConfigDialog(row = {}, edit = false) {
    gcEditMode = edit;
    gcEditKey = row.configKey || '';
    $('#gc-dialog-title').textContent = edit ? '编辑游戏配置' : '新增游戏配置';
    $('#gc-form-game').value = row.gameCode || 'fishing';
    $('#gc-form-key').value = row.configKey || '';
    $('#gc-form-key').readOnly = edit;
    $('#gc-form-value').value = row.configValue ? formatJsonPreview(row.configValue) : '';
    $('#gc-form-tier').value = row.rtpTier || '';
    $('#gc-form-status').value = String(row.status || 1) === '1' ? '1' : '2';
    $('#gc-form-error').classList.add('hidden');
    $('#gc-dialog').showModal();
  }

  // ── RTP Report ─────────────────────────────────────────

  async function initRtpReportTab() {
    try {
      await loadMerchantOptions();
      fillMerchantSelect($('#rtp-merchant-select'), { includeAll: true });
    } catch (err) {
      $('#rtp-body').innerHTML = `<tr><td colspan="7" class="empty error">${escapeHtml(err.message)}</td></tr>`;
      handleAuthError(err);
    }
  }

  async function loadRtpReport() {
    const tbody = $('#rtp-body');
    tbody.innerHTML = '<tr><td colspan="7" class="empty">查询中…</td></tr>';
    const merchantId = $('#rtp-merchant-select').value;
    const gameCode = $('#rtp-game-code').value.trim();
    const hours = parseInt($('#rtp-hours').value, 10) || 24;

    try {
      const data = await AdminAPI.getRtpReport({
        merchantId: merchantId || undefined,
        gameCode,
        hours,
      });
      rtpReportCache = data.list || [];
      rtpPage = 1;
      renderRtpReportPage();
    } catch (err) {
      tbody.innerHTML = `<tr><td colspan="7" class="empty error">${escapeHtml(err.message)}</td></tr>`;
      handleAuthError(err);
    }
  }

  function renderRtpReportPage() {
    const tbody = $('#rtp-body');
    const list = rtpReportCache;
    let totalBet = 0;
    let totalWin = 0;
    let totalRounds = 0;
    list.forEach((row) => {
      totalBet += row.totalBet || 0;
      totalWin += row.totalWin || 0;
      totalRounds += row.totalRounds || 0;
    });
    const weightedRtp = totalBet > 0 ? ((totalWin / totalBet) * 100).toFixed(2) + '%' : '—';

    $('#rtp-stat-rows').textContent = list.length;
    $('#rtp-stat-rounds').textContent = totalRounds.toLocaleString();
    $('#rtp-stat-weighted').textContent = weightedRtp;

    if (!list.length) {
      tbody.innerHTML = '<tr><td colspan="7" class="empty">该时间范围内暂无注单数据</td></tr>';
      $('#rtp-pagination').style.display = 'none';
      return;
    }

    const totalPages = Math.max(1, Math.ceil(list.length / rtpPageSize));
    if (rtpPage > totalPages) rtpPage = totalPages;
    const start = (rtpPage - 1) * rtpPageSize;
    const pageRows = list.slice(start, start + rtpPageSize);

    tbody.innerHTML = pageRows.map((row) => {
      const rtpPct = (row.actualRtp * 100).toFixed(2);
      const rtpClass = row.actualRtp > 1.05 ? 'rtp-high' : (row.actualRtp < 0.90 ? 'rtp-low' : '');
      return `
      <tr>
        <td>${escapeHtml(row.hour)}</td>
        <td>${row.merchantId}</td>
        <td><code>${escapeHtml(row.gameCode)}</code></td>
        <td>${row.totalBet.toFixed(4)}</td>
        <td>${row.totalWin.toFixed(4)}</td>
        <td>${row.totalRounds.toLocaleString()}</td>
        <td><span class="badge ${rtpClass}">${rtpPct}%</span></td>
      </tr>`;
    }).join('');

    $('#rtp-pagination').style.display = list.length > rtpPageSize ? 'flex' : 'none';
    $('#rtp-page-info').textContent = `第 ${rtpPage} / ${totalPages} 页`;
    $('#rtp-prev-page').disabled = rtpPage <= 1;
    $('#rtp-next-page').disabled = rtpPage >= totalPages;
  }

  function exportRtpCsv() {
    if (!rtpReportCache.length) {
      alert('请先查询报表数据');
      return;
    }
    const header = ['hour', 'merchantId', 'gameCode', 'totalBet', 'totalWin', 'totalRounds', 'actualRtp'];
    const lines = [header.join(',')];
    rtpReportCache.forEach((row) => {
      lines.push([
        row.hour,
        row.merchantId,
        row.gameCode,
        row.totalBet,
        row.totalWin,
        row.totalRounds,
        row.actualRtp,
      ].join(','));
    });
    const blob = new Blob([lines.join('\n')], { type: 'text/csv;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `rtp-report-${new Date().toISOString().slice(0, 10)}.csv`;
    a.click();
    URL.revokeObjectURL(url);
  }

  // ── Daily Settlement ───────────────────────────────────

  async function initSettlementTab() {
    try {
      await loadMerchantOptions();
      fillMerchantSelect($('#settle-merchant-select'), { includeAll: true });
      await loadDailySettlements();
    } catch (err) {
      $('#settle-body').innerHTML = `<tr><td colspan="8" class="empty error">${escapeHtml(err.message)}</td></tr>`;
      handleAuthError(err);
    }
  }

  async function loadDailySettlements() {
    const tbody = $('#settle-body');
    tbody.innerHTML = '<tr><td colspan="8" class="empty">加载中…</td></tr>';
    const merchantId = $('#settle-merchant-select').value;
    try {
      const data = await AdminAPI.listDailySettlements({
        merchantId: merchantId || undefined,
        page: settlePage,
        pageSize: settlePageSize,
      });
      settleTotal = data.total || 0;
      const list = data.list || [];
      const totalPages = Math.max(1, Math.ceil(settleTotal / settlePageSize));
      $('#settle-page-info').textContent = `第 ${settlePage} / ${totalPages} 页（共 ${settleTotal} 条）`;
      $('#settle-prev-page').disabled = settlePage <= 1;
      $('#settle-next-page').disabled = settlePage >= totalPages;

      if (!list.length) {
        tbody.innerHTML = '<tr><td colspan="8" class="empty">暂无对账数据，可点击「从 ClickHouse 同步」</td></tr>';
        return;
      }

      tbody.innerHTML = list.map((row) => {
        const rtpPct = (row.actualRtp * 100).toFixed(2);
        const statusLabel = row.status === 1 ? '已确认' : '待确认';
        const statusClass = row.status === 1 ? 'badge ok' : 'badge warn';
        const confirmBtn = row.status === 1 || !AdminAPI.canWrite()
          ? ''
          : `<button class="btn small" data-settle-confirm="${row.id}">确认</button>`;
        return `
        <tr>
          <td>${escapeHtml(row.settleDate)}</td>
          <td><code>${escapeHtml(row.merchantCode || row.merchantId)}</code></td>
          <td>${row.totalBet.toFixed(4)}</td>
          <td>${row.totalWin.toFixed(4)}</td>
          <td>${row.totalRounds.toLocaleString()}</td>
          <td>${rtpPct}%</td>
          <td><span class="${statusClass}">${statusLabel}</span></td>
          <td>${confirmBtn}</td>
        </tr>`;
      }).join('');

      tbody.querySelectorAll('[data-settle-confirm]').forEach((btn) => {
        btn.addEventListener('click', async () => {
          const id = btn.getAttribute('data-settle-confirm');
          if (!confirm('确认锁定该日对账单？确认后不可被同步覆盖。')) return;
          try {
            await AdminAPI.confirmDailySettlement(id);
            await loadDailySettlements();
          } catch (err) {
            alert(err.message);
            handleAuthError(err);
          }
        });
      });
    } catch (err) {
      tbody.innerHTML = `<tr><td colspan="8" class="empty error">${escapeHtml(err.message)}</td></tr>`;
      handleAuthError(err);
    }
  }

  async function syncDailySettlements() {
    const merchantId = $('#settle-merchant-select').value;
    const days = parseInt($('#settle-sync-days').value, 10) || 7;
    const btn = $('#settle-sync-btn');
    btn.disabled = true;
    btn.textContent = '同步中…';
    try {
      const data = await AdminAPI.syncDailySettlements({ merchantId: merchantId || undefined, days });
      alert(`已同步 ${data.synced || 0} 条对账记录`);
      settlePage = 1;
      await loadDailySettlements();
    } catch (err) {
      alert(err.message);
      handleAuthError(err);
    } finally {
      btn.disabled = false;
      btn.textContent = '从 ClickHouse 同步';
    }
  }

  // ── Admin Users ────────────────────────────────────────

  async function loadAdminUsers() {
    const tbody = $('#users-body');
    tbody.innerHTML = '<tr><td colspan="5" class="empty">加载中…</td></tr>';
    try {
      await ensureRolesCache();
      const data = await AdminAPI.listAdminUsers({ page: usersPage, pageSize: usersPageSize });
      usersTotal = data.total || 0;
      const list = data.list || [];
      $('#users-page-info').textContent = `第 ${usersPage} 页 / 共 ${Math.max(1, Math.ceil(usersTotal / usersPageSize))} 页`;
      $('#users-prev-page').disabled = usersPage <= 1;
      $('#users-next-page').disabled = usersPage * usersPageSize >= usersTotal;

      if (!list.length) {
        tbody.innerHTML = '<tr><td colspan="5" class="empty">暂无用户</td></tr>';
        return;
      }

      tbody.innerHTML = list.map((u) => `
        <tr>
          <td>${u.id}</td>
          <td><code>${escapeHtml(u.username)}</code></td>
          <td><span class="badge role-${escapeAttr(u.roleName)}">${escapeHtml(roleLabel(u.roleName))}</span></td>
          <td><span class="badge ${u.status === 1 ? 'active' : 'inactive'}">${u.status === 1 ? '启用' : '禁用'}</span></td>
          <td>
            <button class="btn btn-sm" data-perm="admin" data-edit-user="${u.id}"
              data-username="${escapeAttr(u.username)}"
              data-role="${u.roleId}"
              data-status="${u.status}">编辑</button>
          </td>
        </tr>
      `).join('');

      tbody.querySelectorAll('[data-edit-user]').forEach((btn) => {
        btn.addEventListener('click', () => {
          pendingEditUserId = btn.dataset.editUser;
          $('#edit-user-label').textContent = `用户：${btn.dataset.username}`;
          fillRoleSelect($('#edit-user-role'), btn.dataset.role);
          $('#edit-user-status').value = String(btn.dataset.status) === '1' ? '1' : '2';
          $('#edit-user-password').value = '';
          $('#edit-user-error').classList.add('hidden');
          $('#edit-user-dialog').showModal();
        });
      });
      applyRoleUI();
    } catch (err) {
      tbody.innerHTML = `<tr><td colspan="5" class="empty error">${escapeHtml(err.message)}</td></tr>`;
      handleAuthError(err);
    }
  }

  // ── Event bindings ─────────────────────────────────────

  document.querySelectorAll('.nav-item[data-tab]').forEach((btn) => {
    btn.addEventListener('click', () => switchTab(btn.dataset.tab));
  });

  $('#login-form').addEventListener('submit', async (e) => {
    e.preventDefault();
    const errEl = $('#login-error');
    errEl.classList.add('hidden');
    try {
      const resp = await AdminAPI.login(
        $('#username').value,
        $('#password').value,
        $('#totp-code').value.trim(),
        $('#recovery-code').value.trim(),
      );
      AdminAPI.persistAuth(resp, $('#username').value);
      $('#user-label').textContent = $('#username').value;
      applyRoleUI();
      show('main');
      if (resp.requiresTotpSetup) {
        maybeRequireTotpSetup();
        return;
      }
      blPage = 1;
      switchTab('blacklist');
    } catch (err) {
      errEl.textContent = err.message || '登录失败';
      errEl.classList.remove('hidden');
    }
  });

  $('#logout-btn').addEventListener('click', () => {
    AdminAPI.clearSession();
    show('login');
  });

  window.addEventListener('admin:totp-required', () => maybeRequireTotpSetup());

  // Blacklist toolbar
  $('#refresh-btn').addEventListener('click', () => loadBlacklist());
  $('#filter-type').addEventListener('change', () => { blPage = 1; loadBlacklist(); });
  $('#prev-page').addEventListener('click', () => { if (blPage > 1) { blPage--; loadBlacklist(); } });
  $('#next-page').addEventListener('click', () => { if (blPage * blPageSize < blTotal) { blPage++; loadBlacklist(); } });

  $('#add-btn').addEventListener('click', () => {
    $('#add-form').reset();
    $('#add-error').classList.add('hidden');
    $('#add-dialog').showModal();
  });
  $('#cancel-add').addEventListener('click', () => $('#add-dialog').close());
  $('#add-form').addEventListener('submit', async (e) => {
    e.preventDefault();
    const errEl = $('#add-error');
    errEl.classList.add('hidden');
    const payload = {
      listType: $('#add-type').value,
      listValue: $('#add-value').value.trim(),
      reason: $('#add-reason').value.trim(),
    };
    const expires = toRFC3339($('#add-expires').value);
    if (expires) payload.expiresAt = expires;
    try {
      await AdminAPI.createBlacklist(payload);
      $('#add-dialog').close();
      toast('封禁已添加');
      blPage = 1;
      await loadBlacklist();
    } catch (err) {
      errEl.textContent = err.message;
      errEl.classList.remove('hidden');
    }
  });

  $('#cancel-delete').addEventListener('click', () => $('#confirm-dialog').close());
  $('#confirm-delete').addEventListener('click', async () => {
    if (!pendingDeleteId) return;
    try {
      await AdminAPI.deleteBlacklist(pendingDeleteId);
      $('#confirm-dialog').close();
      toast('封禁已解除');
      await loadBlacklist();
    } catch (err) {
      toast(err.message, 'error');
    }
    pendingDeleteId = null;
  });

  // Merchants toolbar
  $('#merchant-refresh-btn').addEventListener('click', () => loadMerchants());
  $('#breaker-refresh-btn').addEventListener('click', () => loadOpenBreakers());
  $('#merchant-prev-page').addEventListener('click', () => { if (merchantPage > 1) { merchantPage--; loadMerchants(); } });
  $('#merchant-next-page').addEventListener('click', () => { if (merchantPage * merchantPageSize < merchantTotal) { merchantPage++; loadMerchants(); } });

  $('#cancel-rotate').addEventListener('click', () => $('#rotate-dialog').close());
  $('#rotate-form').addEventListener('submit', async (e) => {
    e.preventDefault();
    const errEl = $('#rotate-error');
    errEl.classList.add('hidden');
    const graceHours = parseInt($('#rotate-grace-hours').value, 10) || 24;
    try {
      const resp = await AdminAPI.rotateMerchantKey(pendingRotateId, graceHours);
      $('#rotate-dialog').close();
      toast('密钥轮换成功');
      showKeyResult(resp);
    } catch (err) {
      errEl.textContent = err.message;
      errEl.classList.remove('hidden');
    }
  });

  $('#copy-key-btn').addEventListener('click', async () => {
    const key = $('#new-key-value').textContent;
    try {
      await navigator.clipboard.writeText(key);
      toast('已复制到剪贴板');
    } catch {
      toast('复制失败，请手动选择复制', 'error');
    }
  });

  $('#close-key-result').addEventListener('click', () => {
    $('#key-result-dialog').close();
    $('#new-key-value').textContent = '';
  });

  // Whitelist toolbar
  $('#whitelist-refresh-btn').addEventListener('click', () => {
    whitelistIPCache.clear();
    loadWhitelist();
  });
  $('#whitelist-prev-page').addEventListener('click', () => { if (whitelistPage > 1) { whitelistPage--; loadWhitelist(); } });
  $('#whitelist-next-page').addEventListener('click', () => { if (whitelistPage * whitelistPageSize < whitelistTotal) { whitelistPage++; loadWhitelist(); } });

  $('#trace-search-btn').addEventListener('click', lookupTrace);
  $('#trace-id-input').addEventListener('keydown', (e) => { if (e.key === 'Enter') lookupTrace(); });
  $('#trace-round-search-btn').addEventListener('click', lookupTraceByRound);
  $('#trace-round-input').addEventListener('keydown', (e) => { if (e.key === 'Enter') lookupTraceByRound(); });

  function renderTotpQR(uri) {
    const box = $('#totp-qr');
    box.innerHTML = '';
    if (!uri || typeof QRCode === 'undefined') return;
    new QRCode(box, {
      text: uri,
      width: 220,
      height: 220,
      colorDark: '#0f1419',
      colorLight: '#ffffff',
      correctLevel: QRCode.CorrectLevel.M,
    });
  }

  $('#totp-setup-btn').addEventListener('click', async () => {
    $('#totp-error').classList.add('hidden');
    $('#totp-confirm-code').value = '';
    $('#recovery-box').classList.add('hidden');
    $('#recovery-codes').innerHTML = '';
    try {
      const resp = await AdminAPI.totpSetup();
      $('#totp-uri').textContent = resp.provisioningUri || '';
      $('#totp-secret').textContent = resp.secret || '';
      renderTotpQR(resp.provisioningUri || '');
      $('#totp-dialog').showModal();
    } catch (err) {
      toast(err.message, 'error');
    }
  });
  $('#cancel-totp').addEventListener('click', () => $('#totp-dialog').close());
  $('#totp-form').addEventListener('submit', async (e) => {
    e.preventDefault();
    const errEl = $('#totp-error');
    errEl.classList.add('hidden');
    try {
      const resp = await AdminAPI.totpConfirm($('#totp-confirm-code').value.trim());
      AdminAPI.persistAuth(resp);
      AdminAPI.setTotpPending(false);
      applyRoleUI();
      const box = $('#recovery-box');
      const ul = $('#recovery-codes');
      if (resp.recoveryCodes && resp.recoveryCodes.length) {
        box.classList.remove('hidden');
        ul.innerHTML = resp.recoveryCodes.map((c) => `<li><code>${escapeHtml(c)}</code></li>`).join('');
        toast('2FA 已绑定 — 请立即保存恢复码');
      } else {
        $('#totp-dialog').close();
        toast('2FA 已绑定');
      }
    } catch (err) {
      errEl.textContent = err.message;
      errEl.classList.remove('hidden');
    }
  });

  $('#merchant-create-btn').addEventListener('click', () => {
    $('#create-merchant-form').reset();
    $('#create-merchant-status').value = '1';
    $('#create-merchant-error').classList.add('hidden');
    $('#create-merchant-dialog').showModal();
  });
  $('#cancel-create-merchant').addEventListener('click', () => $('#create-merchant-dialog').close());
  $('#create-merchant-form').addEventListener('submit', async (e) => {
    e.preventDefault();
    const errEl = $('#create-merchant-error');
    errEl.classList.add('hidden');
    try {
      const resp = await AdminAPI.createMerchant({
        merchantCode: $('#create-merchant-code').value.trim(),
        name: $('#create-merchant-name').value.trim(),
        status: parseInt($('#create-merchant-status').value, 10),
      });
      $('#create-merchant-dialog').close();
      merchantOptionsCache = [];
      toast('商户创建成功');
      showKeyResult(resp, 'create');
      merchantPage = 1;
      await loadMerchants();
    } catch (err) {
      errEl.textContent = err.message;
      errEl.classList.remove('hidden');
    }
  });

  $('#cancel-edit-merchant').addEventListener('click', () => $('#edit-merchant-dialog').close());
  $('#edit-merchant-form').addEventListener('submit', async (e) => {
    e.preventDefault();
    const errEl = $('#edit-merchant-error');
    errEl.classList.add('hidden');
    try {
      await AdminAPI.updateMerchant(pendingEditMerchantId, {
        name: $('#edit-merchant-name').value.trim(),
        status: parseInt($('#edit-merchant-status').value, 10),
      });
      $('#edit-merchant-dialog').close();
      merchantOptionsCache = [];
      toast('商户信息已更新');
      await loadMerchants();
    } catch (err) {
      errEl.textContent = err.message;
      errEl.classList.remove('hidden');
    }
  });

  $('#gc-merchant-select').addEventListener('change', () => loadGameConfigs());
  $('#gc-refresh-btn').addEventListener('click', () => loadGameConfigs());
  $('#gc-game-code').addEventListener('keydown', (e) => { if (e.key === 'Enter') loadGameConfigs(); });
  $('#gc-add-btn').addEventListener('click', () => openGameConfigDialog({}, false));
  $('#cancel-gc').addEventListener('click', () => $('#gc-dialog').close());
  $('#gc-form').addEventListener('submit', async (e) => {
    e.preventDefault();
    const errEl = $('#gc-form-error');
    errEl.classList.add('hidden');
    const merchantId = parseInt($('#gc-merchant-select').value, 10);
    if (!merchantId) {
      errEl.textContent = '请先选择商户';
      errEl.classList.remove('hidden');
      return;
    }
    let configValue = $('#gc-form-value').value.trim();
    try {
      configValue = JSON.stringify(JSON.parse(configValue));
    } catch {
      errEl.textContent = '配置值必须是合法 JSON';
      errEl.classList.remove('hidden');
      return;
    }
    try {
      await AdminAPI.upsertGameConfig({
        merchantId,
        gameCode: $('#gc-form-game').value.trim(),
        configKey: $('#gc-form-key').value.trim(),
        configValue,
        rtpTier: $('#gc-form-tier').value.trim(),
        status: parseInt($('#gc-form-status').value, 10),
      });
      $('#gc-dialog').close();
      toast('配置已保存');
      await loadGameConfigs();
    } catch (err) {
      errEl.textContent = err.message;
      errEl.classList.remove('hidden');
    }
  });

  $('#rtp-refresh-btn').addEventListener('click', () => loadRtpReport());
  $('#rtp-export-btn').addEventListener('click', () => exportRtpCsv());
  $('#rtp-prev-page').addEventListener('click', () => { if (rtpPage > 1) { rtpPage--; renderRtpReportPage(); } });
  $('#rtp-next-page').addEventListener('click', () => {
    const totalPages = Math.max(1, Math.ceil(rtpReportCache.length / rtpPageSize));
    if (rtpPage < totalPages) { rtpPage++; renderRtpReportPage(); }
  });
  $('#rtp-game-code').addEventListener('keydown', (e) => { if (e.key === 'Enter') loadRtpReport(); });

  $('#settle-refresh-btn').addEventListener('click', () => { settlePage = 1; loadDailySettlements(); });
  $('#settle-sync-btn').addEventListener('click', () => syncDailySettlements());
  $('#settle-prev-page').addEventListener('click', () => { if (settlePage > 1) { settlePage--; loadDailySettlements(); } });
  $('#settle-next-page').addEventListener('click', () => {
    const totalPages = Math.max(1, Math.ceil(settleTotal / settlePageSize));
    if (settlePage < totalPages) { settlePage++; loadDailySettlements(); }
  });

  $('#users-refresh-btn').addEventListener('click', () => loadAdminUsers());
  $('#users-prev-page').addEventListener('click', () => { if (usersPage > 1) { usersPage--; loadAdminUsers(); } });
  $('#users-next-page').addEventListener('click', () => { if (usersPage * usersPageSize < usersTotal) { usersPage++; loadAdminUsers(); } });
  $('#users-create-btn').addEventListener('click', async () => {
    try {
      await ensureRolesCache();
      fillRoleSelect($('#create-user-role'));
      $('#create-user-form').reset();
      $('#create-user-status').value = '1';
      $('#create-user-error').classList.add('hidden');
      $('#create-user-dialog').showModal();
    } catch (err) {
      toast(err.message, 'error');
    }
  });
  $('#cancel-create-user').addEventListener('click', () => $('#create-user-dialog').close());
  $('#create-user-form').addEventListener('submit', async (e) => {
    e.preventDefault();
    const errEl = $('#create-user-error');
    errEl.classList.add('hidden');
    try {
      await AdminAPI.createAdminUser({
        username: $('#create-user-name').value.trim(),
        password: $('#create-user-password').value,
        roleId: parseInt($('#create-user-role').value, 10),
        status: parseInt($('#create-user-status').value, 10),
      });
      $('#create-user-dialog').close();
      toast('用户已创建');
      usersPage = 1;
      await loadAdminUsers();
    } catch (err) {
      errEl.textContent = err.message;
      errEl.classList.remove('hidden');
    }
  });
  $('#cancel-edit-user').addEventListener('click', () => $('#edit-user-dialog').close());
  $('#edit-user-form').addEventListener('submit', async (e) => {
    e.preventDefault();
    const errEl = $('#edit-user-error');
    errEl.classList.add('hidden');
    const payload = {
      roleId: parseInt($('#edit-user-role').value, 10),
      status: parseInt($('#edit-user-status').value, 10),
    };
    const pwd = $('#edit-user-password').value;
    if (pwd) payload.password = pwd;
    try {
      await AdminAPI.updateAdminUser(pendingEditUserId, payload);
      $('#edit-user-dialog').close();
      toast('用户信息已更新');
      await loadAdminUsers();
    } catch (err) {
      errEl.textContent = err.message;
      errEl.classList.remove('hidden');
    }
  });

  $('#cancel-whitelist').addEventListener('click', () => $('#whitelist-dialog').close());
  $('#whitelist-form').addEventListener('submit', async (e) => {
    e.preventDefault();
    const errEl = $('#whitelist-error');
    errEl.classList.add('hidden');
    const allowedIps = $('#whitelist-ips').value
      .split('\n')
      .map((line) => line.trim())
      .filter(Boolean);
    try {
      const resp = await AdminAPI.updateMerchantAllowedIPs(pendingWhitelistId, allowedIps);
      whitelistIPCache.set(pendingWhitelistId, resp.allowedIps || []);
      $('#whitelist-dialog').close();
      toast('IP 白名单已保存');
      await loadWhitelist();
    } catch (err) {
      errEl.textContent = err.message;
      errEl.classList.remove('hidden');
    }
  });

  async function boot() {
    await AdminI18n.init({
      onChange: () => {
        $('#page-subtitle').textContent = tabTitle(activeTab);
      },
    });
    if (AdminAPI.isLoggedIn()) {
      $('#user-label').textContent = AdminAPI.getUsername() || 'admin';
      applyRoleUI();
      show('main');
      if (AdminAPI.isTotpPending()) {
        maybeRequireTotpSetup();
      } else {
        switchTab('blacklist');
      }
    } else {
      show('login');
    }
  }

  boot();
})();
