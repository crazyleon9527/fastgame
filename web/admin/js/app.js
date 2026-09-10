(() => {
  const TYPE_LABELS = { ip: 'IP', user_id: '用户 ID', merchant: '商户' };

  let page = 1;
  const pageSize = 20;
  let total = 0;
  let pendingDeleteId = null;

  const $ = (sel) => document.querySelector(sel);

  function show(view) {
    $('#login-view').classList.toggle('hidden', view !== 'login');
    $('#main-view').classList.toggle('hidden', view !== 'main');
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
    const d = new Date(localDatetime);
    return d.toISOString();
  }

  async function loadBlacklist() {
    const listType = $('#filter-type').value;
    const tbody = $('#blacklist-body');
    tbody.innerHTML = '<tr><td colspan="8" class="empty">加载中…</td></tr>';

    try {
      const data = await AdminAPI.listBlacklist({ listType, page, pageSize });
      total = data.total || 0;
      $('#stat-count').textContent = (data.list || []).length;
      $('#stat-total').textContent = total;
      $('#page-info').textContent = `第 ${page} 页 / 共 ${Math.max(1, Math.ceil(total / pageSize))} 页`;
      $('#prev-page').disabled = page <= 1;
      $('#next-page').disabled = page * pageSize >= total;

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
      if (err.message.includes('登录')) show('login');
    }
  }

  function escapeHtml(s) {
    return String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
  }

  function escapeAttr(s) {
    return String(s).replace(/"/g, '&quot;');
  }

  // Login
  $('#login-form').addEventListener('submit', async (e) => {
    e.preventDefault();
    const errEl = $('#login-error');
    errEl.classList.add('hidden');
    try {
      const resp = await AdminAPI.login($('#username').value, $('#password').value);
      AdminAPI.setToken(resp.accessToken);
      $('#user-label').textContent = $('#username').value;
      show('main');
      page = 1;
      await loadBlacklist();
    } catch (err) {
      errEl.textContent = err.message || '登录失败';
      errEl.classList.remove('hidden');
    }
  });

  $('#logout-btn').addEventListener('click', () => {
    AdminAPI.clearToken();
    show('login');
  });

  // Toolbar
  $('#refresh-btn').addEventListener('click', () => loadBlacklist());
  $('#filter-type').addEventListener('change', () => { page = 1; loadBlacklist(); });
  $('#prev-page').addEventListener('click', () => { if (page > 1) { page--; loadBlacklist(); } });
  $('#next-page').addEventListener('click', () => { if (page * pageSize < total) { page++; loadBlacklist(); } });

  // Add
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
      page = 1;
      await loadBlacklist();
    } catch (err) {
      errEl.textContent = err.message;
      errEl.classList.remove('hidden');
    }
  });

  // Delete
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

  // Init
  if (AdminAPI.isLoggedIn()) {
    $('#user-label').textContent = $('#username').value || 'admin';
    show('main');
    loadBlacklist();
  } else {
    show('login');
  }
})();
