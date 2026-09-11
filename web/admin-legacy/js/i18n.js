const AdminI18n = (() => {
  const STORAGE_KEY = 'admin_locale';
  const BUNDLE = 'ui.admin';
  let locale = localStorage.getItem(STORAGE_KEY) || 'zh-CN';
  let messages = {};
  let locales = [];

  function isBrokenText(value) {
    return typeof value === 'string' && /^[?\uFFFD\s]+$/.test(value);
  }

  function t(key, fallback = '') {
    const val = messages[key];
    if (val && !isBrokenText(val)) return val;
    if (fallback) return fallback;
    return key;
  }

  function getLocale() {
    return locale;
  }

  async function fetchJson(path) {
    const resp = await fetch(`${window.location.origin}${path}`);
    if (!resp.ok) throw new Error(`i18n fetch failed: ${resp.status}`);
    return resp.json();
  }

  async function loadLocales() {
    const data = await fetchJson('/api/v1/admin/locales');
    locales = data.list || [];
    return locales;
  }

  async function loadDictionary(nextLocale = locale) {
    locale = nextLocale || locale;
    localStorage.setItem(STORAGE_KEY, locale);
    const params = new URLSearchParams({ bundle: BUNDLE, locale });
    const data = await fetchJson(`/api/v1/admin/i18n/dictionary?${params}`);
    messages = data.messages || {};
    document.documentElement.lang = locale;
    return messages;
  }

  function apply(root = document) {
    root.querySelectorAll('[data-i18n]').forEach((el) => {
      const key = el.dataset.i18n;
      const fallback = el.getAttribute('data-i18n-fallback') || el.textContent.trim();
      const val = t(key, fallback);
      if (!val || val === key) return;
      if (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA') {
        if (el.placeholder !== undefined && el.dataset.i18nTarget === 'placeholder') {
          el.placeholder = val;
        }
      } else {
        el.textContent = val;
      }
    });
    root.querySelectorAll('[data-i18n-placeholder]').forEach((el) => {
      el.placeholder = t(el.dataset.i18nPlaceholder, el.placeholder);
    });
  }

  function fillLocaleSelect(selectEl) {
    if (!selectEl || !locales.length) return;
    selectEl.innerHTML = locales.map((loc) => {
      const sel = loc.code === locale ? ' selected' : '';
      return `<option value="${loc.code}"${sel}>${loc.nativeName || loc.name}</option>`;
    }).join('');
  }

  async function init({ onChange } = {}) {
    try {
      await loadLocales();
      await loadDictionary(locale);
    } catch (err) {
      console.warn('AdminI18n init failed, using built-in labels', err);
      return;
    }
    document.querySelectorAll('#locale-select, #login-locale-select').forEach((sel) => {
      fillLocaleSelect(sel);
      sel.addEventListener('change', async () => {
        await loadDictionary(sel.value);
        apply();
        if (typeof onChange === 'function') onChange();
      });
    });
    apply();
  }

  return { t, getLocale, init, apply, loadDictionary, loadLocales };
})();
