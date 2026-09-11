import { defineStore } from "pinia";
import { store } from "../index";
import { getI18nDictionary, listLocales } from "@/api/fastgame";

const LOCALE_KEY = "admin-locale";

function detectLocale(): string {
  const saved = localStorage.getItem(LOCALE_KEY);
  if (saved) return saved;
  return navigator.language.startsWith("zh") ? "zh-CN" : "en-US";
}

export const useI18nStore = defineStore("fastgame-i18n", {
  state: () => ({
    locale: detectLocale(),
    messages: {} as Record<string, string>,
    locales: [] as Array<{ code: string; name: string; nativeName: string }>,
    loaded: false
  }),
  actions: {
    async load(force = false) {
      if (this.loaded && !force) return;
      try {
        const [dict, locResp] = await Promise.all([
          getI18nDictionary("ui.admin", this.locale),
          listLocales()
        ]);
        this.messages = dict?.messages ?? {};
        this.locales = locResp?.list ?? [];
        this.loaded = true;
      } catch (err) {
        console.warn("[i18n] dictionary load failed, using fallbacks", err);
        this.loaded = true;
      }
    },
    async setLocale(code: string) {
      this.locale = code;
      localStorage.setItem(LOCALE_KEY, code);
      this.loaded = false;
      await this.load(true);
    },
    t(key: string, fallback?: string): string {
      return this.messages[key] ?? fallback ?? key;
    }
  }
});

export function useI18nStoreHook() {
  return useI18nStore(store);
}
