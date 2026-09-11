<script setup lang="ts">
import { useRouter } from "vue-router";
import { message } from "@/utils/message";
import { loginRules } from "./utils/rule";
import { ref, reactive, toRaw } from "vue";
import { debounce } from "@pureadmin/utils";
import { useNav } from "@/layout/hooks/useNav";
import { useEventListener } from "@vueuse/core";
import type { FormInstance } from "element-plus";
import { useLayout } from "@/layout/hooks/useLayout";
import { useUserStoreHook } from "@/store/modules/user";
import { initRouter, getTopMenu } from "@/router/utils";
import { bg, avatar, illustration } from "./utils/static";
import { useRenderIcon } from "@/components/ReIcon/src/hooks";
import { useDataThemeChange } from "@/layout/hooks/useDataThemeChange";
import { useI18nStoreHook } from "@/store/modules/i18n";

const i18nStore = useI18nStoreHook();

import dayIcon from "@/assets/svg/day.svg?component";
import darkIcon from "@/assets/svg/dark.svg?component";
import Lock from "~icons/ri/lock-fill";
import User from "~icons/ri/user-3-fill";

defineOptions({
  name: "Login"
});

const router = useRouter();
const loading = ref(false);
const disabled = ref(false);
const ruleFormRef = ref<FormInstance>();

const { initStorage } = useLayout();
initStorage();

const { dataTheme, overallStyle, dataThemeChange } = useDataThemeChange();
dataThemeChange(overallStyle.value);
const { title } = useNav();

const ruleForm = reactive({
  username: "admin",
  password: "admin123",
  totpCode: "",
  recoveryCode: ""
});

const onLogin = async (formEl: FormInstance | undefined) => {
  if (!formEl) return;
  await formEl.validate(valid => {
    if (valid) {
      loading.value = true;
      useUserStoreHook()
        .loginByUsername({
          username: ruleForm.username,
          password: ruleForm.password,
          totpCode: ruleForm.totpCode || undefined,
          recoveryCode: ruleForm.recoveryCode || undefined
        })
        .then(res => {
          if (res.success) {
            return initRouter().then(() => {
              disabled.value = true;
              router
                .push(getTopMenu(true).path)
                .then(() => {
                  message("登录成功", { type: "success" });
                })
                .finally(() => (disabled.value = false));
            });
          } else {
            message("登录失败", { type: "error" });
          }
        })
        .catch((err: Error) => {
          message(err.message || "登录失败", { type: "error" });
        })
        .finally(() => (loading.value = false));
    }
  });
};

const immediateDebounce: any = debounce(
  formRef => onLogin(formRef),
  1000,
  true
);

useEventListener(document, "keydown", ({ code }) => {
  if (
    ["Enter", "NumpadEnter"].includes(code) &&
    !disabled.value &&
    !loading.value
  )
    immediateDebounce(ruleFormRef.value);
});
</script>

<template>
  <div class="select-none">
    <img :src="bg" class="wave" />
    <div class="flex-c absolute right-5 top-3">
      <!-- 主题 -->
      <el-switch
        v-model="dataTheme"
        inline-prompt
        :active-icon="dayIcon"
        :inactive-icon="darkIcon"
        @change="dataThemeChange"
      />
    </div>
    <div class="login-container">
      <div class="img">
        <component :is="toRaw(illustration)" />
      </div>
      <div class="login-box">
        <div class="login-form">
          <avatar class="avatar" />
          <div>
            <h2 class="outline-hidden">{{ title }}</h2>
            <p class="login-subtitle">
              {{ i18nStore.t("login.subtitle", "运营管理后台") }}
            </p>
          </div>

          <el-form
            ref="ruleFormRef"
            :model="ruleForm"
            :rules="loginRules"
            size="large"
          >
            <el-form-item
                :rules="[
                  {
                    required: true,
                    message: '请输入账号',
                    trigger: 'blur'
                  }
                ]"
                prop="username"
              >
                <el-input
                  v-model="ruleForm.username"
                  clearable
                  :placeholder="i18nStore.t('label.username', '账号')"
                  :prefix-icon="useRenderIcon(User)"
                />
              </el-form-item>

            <el-form-item prop="password">
                <el-input
                  v-model="ruleForm.password"
                  clearable
                  show-password
                  :placeholder="i18nStore.t('label.password', '密码')"
                  :prefix-icon="useRenderIcon(Lock)"
                />
              </el-form-item>

            <el-form-item>
                <el-input
                  v-model="ruleForm.totpCode"
                  clearable
                  placeholder="Google Authenticator (启用 2FA 时必填)"
                />
              </el-form-item>

            <el-form-item>
                <el-input
                  v-model="ruleForm.recoveryCode"
                  clearable
                  placeholder="恢复码 (可选)"
                />
              </el-form-item>

            <el-button
                class="w-full mt-4!"
                size="default"
                type="primary"
                :loading="loading"
                :disabled="disabled"
                @click="onLogin(ruleFormRef)"
              >
                {{ i18nStore.t("btn.login", "登录") }}
              </el-button>
          </el-form>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
@import url("@/style/login.css");
</style>

<style lang="scss" scoped>
.login-subtitle {
  margin: 0 0 16px;
  color: var(--el-text-color-secondary);
  font-size: 14px;
  text-align: center;
}

:deep(.el-input-group__append, .el-input-group__prepend) {
  padding: 0;
}
</style>
