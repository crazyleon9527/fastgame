<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref } from "vue";
import { ElMessage } from "element-plus";
import { totpSetup, totpConfirm } from "@/api/fastgame";
import { setToken } from "@/utils/auth";
import { useUserStoreHook } from "@/store/modules/user";
import {
  isTotpPending,
  setTotpPending,
  totpBus,
  TOTP_ENFORCE
} from "@/utils/totp";

defineOptions({ name: "TotpSetupDialog" });

const visible = ref(false);
const loading = ref(false);
const confirmCode = ref("");
const secret = ref("");
const provisioningUri = ref("");
const recoveryCodes = ref<string[]>([]);
const qrRef = ref<HTMLElement | null>(null);
const forceOpen = ref(false);

declare global {
  interface Window {
    QRCode?: new (
      el: HTMLElement,
      opts: {
        text: string;
        width?: number;
        height?: number;
        colorDark?: string;
        colorLight?: string;
        correctLevel?: number;
      }
    ) => { clear: () => void };
  }
}

function loadQrScript(): Promise<void> {
  if (window.QRCode) return Promise.resolve();
  return new Promise((resolve, reject) => {
    const script = document.createElement("script");
    script.src = `${import.meta.env.BASE_URL}js/qrcode.min.js`;
    script.onload = () => resolve();
    script.onerror = () => reject(new Error("二维码脚本加载失败"));
    document.head.appendChild(script);
  });
}

function renderQr(uri: string) {
  if (!qrRef.value || !uri || !window.QRCode) return;
  qrRef.value.innerHTML = "";
  new window.QRCode(qrRef.value, {
    text: uri,
    width: 220,
    height: 220,
    colorDark: "#0f1419",
    colorLight: "#ffffff",
    correctLevel: (window.QRCode as any)?.CorrectLevel?.M ?? 1
  });
}

async function startSetup() {
  loading.value = true;
  recoveryCodes.value = [];
  confirmCode.value = "";
  try {
    await loadQrScript();
    const resp = await totpSetup();
    secret.value = resp?.secret ?? "";
    provisioningUri.value = resp?.provisioningUri ?? "";
    visible.value = true;
    forceOpen.value = TOTP_ENFORCE && isTotpPending();
    await nextTick();
    renderQr(provisioningUri.value);
  } catch (err: any) {
    ElMessage.error(err?.message || "2FA 初始化失败");
  } finally {
    loading.value = false;
  }
}

async function onConfirm() {
  if (!confirmCode.value.trim()) {
    ElMessage.warning("请输入 6 位验证码");
    return;
  }
  loading.value = true;
  try {
    const resp = await totpConfirm(confirmCode.value.trim());
    const expireMs =
      resp.expireAt > 1e12 ? resp.expireAt : (resp.expireAt ?? 0) * 1000;
    const roleName = resp.roleName || useUserStoreHook().roles[0] || "admin";
    setToken({
      accessToken: resp.accessToken,
      refreshToken: resp.accessToken,
      expires: new Date(expireMs || Date.now() + 86400000),
      username: useUserStoreHook().username,
      nickname: useUserStoreHook().nickname,
      roles: [roleName],
      permissions: useUserStoreHook().permissions
    });
    setTotpPending(false);
    forceOpen.value = false;
    if (resp.recoveryCodes?.length) {
      recoveryCodes.value = resp.recoveryCodes;
      ElMessage.success("2FA 已绑定，请立即保存恢复码");
    } else {
      visible.value = false;
      ElMessage.success("2FA 已绑定");
    }
  } catch (err: any) {
    ElMessage.error(err?.message || "验证码错误");
  } finally {
    loading.value = false;
  }
}

function onClose() {
  if (TOTP_ENFORCE && (forceOpen.value || isTotpPending())) {
    ElMessage.warning("请先完成 2FA 绑定后再使用后台功能");
    visible.value = true;
  }
}

onMounted(() => {
  totpBus.on("open", startSetup);
  if (TOTP_ENFORCE && isTotpPending()) {
    startSetup();
  }
});

onBeforeUnmount(() => {
  totpBus.off("open");
});

defineExpose({ open: startSetup });
</script>

<template>
  <el-dialog
    v-model="visible"
    title="Google Authenticator 绑定"
    width="480px"
    class="totp-dialog"
    :close-on-click-modal="!forceOpen"
    :close-on-press-escape="!forceOpen"
    :show-close="!forceOpen"
    @close="onClose"
  >
    <p class="hint">
      使用 Google Authenticator 扫描下方二维码，或手动输入 Secret，然后输入 6
      位验证码确认。
    </p>
    <div ref="qrRef" class="qr-wrap" />
    <p v-if="secret" class="secret">
      Secret: <code>{{ secret }}</code>
    </p>
    <details v-if="provisioningUri" class="uri-fallback">
      <summary>无法扫码？查看 provisioning URI</summary>
      <code>{{ provisioningUri }}</code>
    </details>
    <el-form v-if="!recoveryCodes.length" class="mt-4" label-width="88px">
      <el-form-item label="验证码">
        <el-input
          v-model="confirmCode"
          inputmode="numeric"
          autocomplete="one-time-code"
          maxlength="8"
          placeholder="Authenticator 中的 6 位数字"
          @keyup.enter="onConfirm"
        />
      </el-form-item>
    </el-form>
    <div v-else class="recovery-box">
      <p class="recovery-title">请立即保存以下恢复码（仅展示一次）：</p>
      <ul>
        <li v-for="code in recoveryCodes" :key="code">
          <code>{{ code }}</code>
        </li>
      </ul>
    </div>
    <template #footer>
      <el-button
        v-if="recoveryCodes.length"
        type="primary"
        @click="visible = false"
      >
        我已保存恢复码
      </el-button>
      <template v-else>
        <el-button v-if="!forceOpen" @click="visible = false">取消</el-button>
        <el-button type="primary" :loading="loading" @click="onConfirm">
          确认绑定
        </el-button>
      </template>
    </template>
  </el-dialog>
</template>

<style scoped>
.hint {
  margin: 0 0 12px;
  color: var(--el-text-color-secondary);
  font-size: 13px;
  line-height: 1.5;
}
.qr-wrap {
  display: flex;
  justify-content: center;
  min-height: 220px;
  padding: 12px;
  border-radius: 8px;
  background: #fff;
}
.secret {
  margin: 12px 0 0;
  font-size: 13px;
}
.uri-fallback {
  margin-top: 8px;
  font-size: 12px;
}
.uri-fallback code {
  display: block;
  margin-top: 6px;
  word-break: break-all;
}
.recovery-box {
  margin-top: 8px;
  padding: 12px;
  border-radius: 8px;
  background: var(--el-fill-color-light);
}
.recovery-title {
  margin: 0 0 8px;
  font-weight: 600;
}
.recovery-box ul {
  margin: 0;
  padding-left: 20px;
}

@media screen and (width <= 768px) {
  :global(.totp-dialog) {
    width: 92vw !important;
  }

  .qr-wrap {
    min-height: 180px;
    padding: 8px;
  }
}
</style>
