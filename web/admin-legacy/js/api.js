const AdminAPI = (() => {
  const BASE = window.location.origin;

  function getToken() {
    return sessionStorage.getItem('admin_token');
  }

  function setToken(token) {
    sessionStorage.setItem('admin_token', token);
  }

  function getRole() {
    return sessionStorage.getItem('admin_role') || 'admin';
  }

  function setRole(role) {
    if (role) sessionStorage.setItem('admin_role', role);
  }

  function setUsername(name) {
    if (name) sessionStorage.setItem('admin_username', name);
  }

  function getUsername() {
    return sessionStorage.getItem('admin_username') || '';
  }

  function setTotpPending(pending) {
    sessionStorage.setItem('totp_pending', pending ? '1' : '0');
  }

  function isTotpPending() {
    return sessionStorage.getItem('totp_pending') === '1';
  }

  function clearSession() {
    sessionStorage.removeItem('admin_token');
    sessionStorage.removeItem('admin_role');
    sessionStorage.removeItem('admin_username');
    sessionStorage.removeItem('totp_pending');
  }

  function persistAuth(resp, username) {
    if (resp?.accessToken) setToken(resp.accessToken);
    if (resp?.roleName) setRole(resp.roleName);
    if (username) setUsername(username);
    if (typeof resp?.requiresTotpSetup === 'boolean') {
      setTotpPending(resp.requiresTotpSetup);
    } else if (resp?.accessToken && resp.roleName && !('requiresTotpSetup' in resp)) {
      // TOTP 确认成功后新 token 不再携带 pending
      setTotpPending(false);
    }
  }

  async function request(method, path, body) {
    const headers = { 'Content-Type': 'application/json' };
    const token = getToken();
    if (token) headers['Authorization'] = `Bearer ${token}`;

    const resp = await fetch(`${BASE}${path}`, {
      method,
      headers,
      body: body ? JSON.stringify(body) : undefined,
    });

    if (resp.status === 401) {
      clearSession();
      const text401 = await resp.text();
      if (text401.includes('roleName')) {
        throw new Error('登录凭证已过期，请重新登录');
      }
      throw new Error('登录已过期，请重新登录');
    }

    if (resp.status === 403) {
      const text = await resp.text();
      if (text.includes('totp setup required')) {
        setTotpPending(true);
        window.dispatchEvent(new CustomEvent('admin:totp-required'));
        throw new Error('请先完成 2FA 绑定');
      }
      if (text.includes('permission denied')) {
        throw new Error('权限不足，无法执行此操作');
      }
      throw new Error(text || '请求被拒绝 (403)');
    }

    if (!resp.ok) {
      const text = await resp.text();
      throw new Error(text || `请求失败 (${resp.status})`);
    }

    if (resp.status === 204 || resp.headers.get('content-length') === '0') {
      return null;
    }
    return resp.json();
  }

  return {
    getToken,
    setToken,
    getRole,
    setRole,
    getUsername,
    setUsername,
    setTotpPending,
    isTotpPending,
    clearSession,
    persistAuth,
    isLoggedIn: () => !!getToken(),
    canWrite: () => getRole() !== 'viewer',
    isAdmin: () => getRole() === 'admin',

    login(username, password, totpCode = '', recoveryCode = '') {
      return request('POST', '/api/v1/admin/login', {
        username,
        password,
        totpCode: totpCode || undefined,
        recoveryCode: recoveryCode || undefined,
      });
    },

    listRiskAlerts(limit = 50) {
      return request('GET', `/api/v1/admin/risk-alerts?limit=${limit}`);
    },

    listBlacklist({ listType = '', page = 1, pageSize = 20 } = {}) {
      const params = new URLSearchParams({ page, pageSize });
      if (listType) params.set('listType', listType);
      return request('GET', `/api/v1/admin/risk-blacklist?${params}`);
    },

    createBlacklist(data) {
      return request('POST', '/api/v1/admin/risk-blacklist', data);
    },

    deleteBlacklist(id) {
      return request('DELETE', `/api/v1/admin/risk-blacklist/${id}`);
    },

    listMerchants({ page = 1, pageSize = 20 } = {}) {
      const params = new URLSearchParams({ page, pageSize });
      return request('GET', `/api/v1/admin/merchants?${params}`);
    },

    createMerchant({ merchantCode, name, status = 1 }) {
      return request('POST', '/api/v1/admin/merchants', { merchantCode, name, status });
    },

    updateMerchant(id, { name, status }) {
      return request('PUT', `/api/v1/admin/merchants/${id}`, { name, status });
    },

    listGameConfigs({ merchantId, gameCode = '' } = {}) {
      const params = new URLSearchParams({ merchantId });
      if (gameCode) params.set('gameCode', gameCode);
      return request('GET', `/api/v1/admin/game-configs?${params}`);
    },

    upsertGameConfig(data) {
      return request('POST', '/api/v1/admin/game-configs', data);
    },

    deleteGameConfig(id) {
      return request('DELETE', `/api/v1/admin/game-configs/${id}`);
    },

    getRtpReport({ merchantId, gameCode = '', hours = 24 } = {}) {
      const params = new URLSearchParams({ hours });
      if (merchantId) params.set('merchantId', merchantId);
      if (gameCode) params.set('gameCode', gameCode);
      return request('GET', `/api/v1/admin/reports/rtp?${params}`);
    },

    listDailySettlements({ merchantId, page = 1, pageSize = 20 } = {}) {
      const params = new URLSearchParams({ page, pageSize });
      if (merchantId) params.set('merchantId', merchantId);
      return request('GET', `/api/v1/admin/reports/daily-settlements?${params}`);
    },

    syncDailySettlements({ merchantId, days = 7 } = {}) {
      const body = { days };
      if (merchantId) body.merchantId = merchantId;
      return request('POST', '/api/v1/admin/reports/daily-settlements/sync', body);
    },

    confirmDailySettlement(id) {
      return request('POST', `/api/v1/admin/reports/daily-settlements/${id}/confirm`);
    },

    rotateMerchantKey(id, gracePeriodHours = 24) {
      return request('POST', `/api/v1/admin/merchants/${id}/rotate-key`, { gracePeriodHours });
    },

    getMerchantAllowedIPs(id) {
      return request('GET', `/api/v1/admin/merchants/${id}/allowed-ips`);
    },

    updateMerchantAllowedIPs(id, allowedIps) {
      return request('PUT', `/api/v1/admin/merchants/${id}/allowed-ips`, { allowedIps });
    },

    lookupTrace(traceId) {
      return request('GET', `/api/v1/admin/traces/${encodeURIComponent(traceId)}`);
    },

    lookupTraceByRound(roundId) {
      return request('GET', `/api/v1/admin/traces/by-round/${encodeURIComponent(roundId)}`);
    },

    ackRiskAlert(id) {
      return request('POST', `/api/v1/admin/risk-alerts/${id}/ack`);
    },

    totpSetup() {
      return request('POST', '/api/v1/admin/totp/setup');
    },

    totpConfirm(totpCode) {
      return request('POST', '/api/v1/admin/totp/confirm', { totpCode });
    },

    resetWalletBreaker(merchantCode) {
      return request('POST', '/api/v1/admin/wallet/breaker/reset', { merchantCode });
    },

    listWalletBreakers() {
      return request('GET', '/api/v1/admin/wallet/breakers');
    },

    listAdminUsers({ page = 1, pageSize = 20 } = {}) {
      const params = new URLSearchParams({ page, pageSize });
      return request('GET', `/api/v1/admin/users?${params}`);
    },

    createAdminUser({ username, password, roleId, status = 1 }) {
      return request('POST', '/api/v1/admin/users', { username, password, roleId, status });
    },

    updateAdminUser(id, { roleId, status, password }) {
      const body = {};
      if (roleId) body.roleId = roleId;
      if (status) body.status = status;
      if (password) body.password = password;
      return request('PUT', `/api/v1/admin/users/${id}`, body);
    },

    listRoles() {
      return request('GET', '/api/v1/admin/roles');
    },
  };
})();
