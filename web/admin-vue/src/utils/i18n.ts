import { useI18nStoreHook } from "@/store/modules/i18n";

export function t(key: string, fallback?: string): string {
  return useI18nStoreHook().t(key, fallback);
}

export function menuTitle(meta?: { i18nKey?: string; title?: string }) {
  if (!meta) return "";
  if (meta.i18nKey) {
    return t(meta.i18nKey, meta.title);
  }
  return meta.title ?? "";
}
