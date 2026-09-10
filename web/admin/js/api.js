const AdminAPI = (() => {
  const BASE = window.location.origin;

  function getToken() {
    return sessionStorage.getItem('admin_token');
  }

  function setToken(token) {
    sessionStorage.setItem('admin_token', token);
  }

  function clearToken() {
    sessionStorage.removeItem('admin_token');
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
      clearToken();
      throw new Error('登录已过期，请重新登录');
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
    clearToken,
    isLoggedIn: () => !!getToken(),

    login(username, password, totpCode = '') {
      return request('POST', '/api/v1/admin/login', { username, password, totpCode: totpCode || undefined });
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
  };
})();
