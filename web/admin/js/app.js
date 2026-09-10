(() => {
  const TYPE_LABELS = { ip: 'IP', user_id: '用户 ID', merchant: '商户' };
  const TAB_TITLES = { blacklist: '风控黑名单', merchants: '商户密钥轮换', whitelist: 'IP 白名单', trace: 'Trace 追踪' };

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
    $('#panel-whitelist').classList.toggle('hidden', tab !== 'whitelist');
    $('#panel-trace').classList.toggle('hidden', tab !== 'trace');
    $('#page-subtitle').textContent = TAB_TITLES[tab] || '';

    if (tab === 'blacklist') loadBlacklist();
    if (tab === 'merchants') loadMerchants();
    if (tab === 'whitelist') loadWhitelist();
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
      const spans = data.spans || [];
      const txs = data.pendingTransactions || [];
      box.innerHTML = `
        <h3>Trace: <code>${escapeHtml(traceId)}</code></h3>
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
              <td>${t.betAmount}</td>
              <td>${t.winAmount}</td>
              <td>${escapeHtml(t.expectedAction)}</td>
              <td>${t.retryCount}</td>
            </tr>`).join('') : '<tr><td colspan="7" class="empty">无 pending_transactions</td></tr>'}
          </tbody>
        </table></div>`;
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

  function handleAuthError(err) {
    if (err.message.includes('登录')) show('login');
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
            ${row.status === 1 ? `<button class="btn danger btn-sm" data-delete="${row.id}" data-value="${escapeAttr(row.listValue)}">解除</button>` : '—'}
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
    } catch (err) {
      tbody.innerHTML = `<tr><td colspan="8" class="empty error">${escapeHtml(err.message)}</td></tr>`;
      handleAuthError(err);
    }
  }

  // ── Merchants ──────────────────────────────────────────

  async function loadMerchants() {
    const tbody = $('#merchant-body');
    tbody.innerHTML = '<tr><td colspan="5" class="empty">加载中…</td></tr>';

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
          <td>
            <button class="btn btn-sm" data-rotate="${m.id}" data-code="${escapeAttr(m.merchantCode)}" data-name="${escapeAttr(m.name)}" ${m.status !== 1 ? 'disabled' : ''}>
              轮换密钥
            </button>
          </td>
        </tr>
      `).join('');

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
            <button class="btn btn-sm" data-edit-whitelist="${m.id}" data-code="${escapeAttr(m.merchantCode)}" data-name="${escapeAttr(m.name)}">
              编辑白名单
            </button>
          </td>
        </tr>
      `;
      }).join('');

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

  function showKeyResult(resp) {
    $('#new-key-value').textContent = resp.newPrivateKey;
    $('#key-meta').innerHTML = `
      <li>商户编码：<code>${escapeHtml(resp.merchantCode)}</code></li>
      <li>过渡期：<strong>${resp.gracePeriodHours}</strong> 小时</li>
      <li>轮换时间：${formatTime(resp.rotatedAt)}</li>
    `;
    $('#key-result-dialog').showModal();
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
      const resp = await AdminAPI.login($('#username').value, $('#password').value);
      AdminAPI.setToken(resp.accessToken);
      $('#user-label').textContent = $('#username').value;
      show('main');
      blPage = 1;
      switchTab('blacklist');
    } catch (err) {
      errEl.textContent = err.message || '登录失败';
      errEl.classList.remove('hidden');
    }
  });

  $('#logout-btn').addEventListener('click', () => {
    AdminAPI.clearToken();
    show('login');
  });

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

  // Init
  if (AdminAPI.isLoggedIn()) {
    $('#user-label').textContent = $('#username').value || 'admin';
    show('main');
    switchTab('blacklist');
  } else {
    show('login');
  }
})();
