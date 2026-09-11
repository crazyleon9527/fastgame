import mitt from "mitt";
import { storageLocal } from "@pureadmin/utils";

/** 上线前改为 true，启用登录后强制 2FA 绑定弹窗 */
export const TOTP_ENFORCE = false;

export const totpBus = mitt<{ open: void }>();

const TOTP_PENDING_KEY = "totp-pending";

export function isTotpPending(): boolean {
  if (!TOTP_ENFORCE) return false;
  return storageLocal().getItem<boolean>(TOTP_PENDING_KEY) === true;
}

export function setTotpPending(pending: boolean) {
  if (!TOTP_ENFORCE) {
    storageLocal().removeItem(TOTP_PENDING_KEY);
    return;
  }
  if (pending) {
    storageLocal().setItem(TOTP_PENDING_KEY, true);
  } else {
    storageLocal().removeItem(TOTP_PENDING_KEY);
  }
}

/** manual=true 时导航栏仍可手动打开 2FA 设置 */
export function openTotpSetup(manual = false) {
  if (!TOTP_ENFORCE && !manual) return;
  totpBus.emit("open");
}
