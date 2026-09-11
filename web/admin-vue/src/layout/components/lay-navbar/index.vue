<script setup lang="ts">
import { computed } from "vue";
import { useRoute } from "vue-router";
import { useNav } from "@/layout/hooks/useNav";
import { useI18nStoreHook } from "@/store/modules/i18n";
import LaySearch from "../lay-search/index.vue";
import LayNotice from "../lay-notice/index.vue";
import LayNavMix from "../lay-sidebar/NavMix.vue";
import LaySidebarFullScreen from "../lay-sidebar/components/SidebarFullScreen.vue";
import LaySidebarBreadCrumb from "../lay-sidebar/components/SidebarBreadCrumb.vue";
import LaySidebarTopCollapse from "../lay-sidebar/components/SidebarTopCollapse.vue";

import LogoutCircleRLine from "~icons/ri/logout-circle-r-line";
import Setting from "~icons/ri/settings-3-line";
import ShieldKeyholeLine from "~icons/ri/shield-keyhole-line";
import { openTotpSetup } from "@/utils/totp";

const i18nStore = useI18nStoreHook();
const route = useRoute();
const pageTitle = computed(() => {
  const key = route.meta?.i18nKey as string | undefined;
  const title = route.meta?.title as string | undefined;
  return key ? i18nStore.t(key, title ?? "") : (title ?? "");
});

const {
  layout,
  device,
  logout,
  onPanel,
  pureApp,
  username,
  userAvatar,
  avatarsStyle,
  toggleSideBar
} = useNav();
</script>

<template>
  <div class="navbar bg-[#fff] shadow-xs shadow-[rgba(0,21,41,0.08)]">
    <LaySidebarTopCollapse
      v-if="device === 'mobile'"
      class="hamburger-container"
      :is-active="pureApp.sidebar.opened"
      @toggleClick="toggleSideBar"
    />

    <LaySidebarBreadCrumb
      v-if="layout !== 'mix' && device !== 'mobile'"
      class="breadcrumb-container"
    />
    <span v-if="device === 'mobile'" class="mobile-page-title">{{ pageTitle }}</span>

    <LayNavMix v-if="layout === 'mix'" />

    <div v-if="layout === 'vertical'" class="vertical-header-right">
      <LaySearch v-if="device !== 'mobile'" id="header-search" class="header-mobile-hidden" />
      <LaySidebarFullScreen
        v-if="device !== 'mobile'"
        id="full-screen"
        class="header-mobile-hidden"
      />
      <el-select
        v-if="i18nStore.locales.length && device !== 'mobile'"
        v-model="i18nStore.locale"
        size="small"
        class="locale-select header-mobile-hidden"
        @change="(v: string) => i18nStore.setLocale(v)"
      >
        <el-option
          v-for="loc in i18nStore.locales"
          :key="loc.code"
          :label="loc.nativeName || loc.name"
          :value="loc.code"
        />
      </el-select>
      <span
        class="set-icon navbar-bg-hover"
        :title="i18nStore.t('btn.totp', '2FA 设置')"
        @click="openTotpSetup(true)"
      >
        <IconifyIconOffline :icon="ShieldKeyholeLine" />
      </span>
      <LayNotice v-if="device !== 'mobile'" id="header-notice" class="header-mobile-hidden" />
      <el-dropdown trigger="click">
        <span class="el-dropdown-link navbar-bg-hover select-none">
          <img :src="userAvatar" :style="avatarsStyle" />
          <p v-if="username" class="dark:text-white">{{ username }}</p>
        </span>
        <template #dropdown>
          <el-dropdown-menu class="logout">
            <el-dropdown-item @click="logout">
              <IconifyIconOffline
                :icon="LogoutCircleRLine"
                style="margin: 5px"
              />
              {{ i18nStore.t("btn.logout", "退出系统") }}
            </el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>
      <span
        v-if="device !== 'mobile'"
        class="set-icon navbar-bg-hover header-mobile-hidden"
        title="打开系统配置"
        @click="onPanel"
      >
        <IconifyIconOffline :icon="Setting" />
      </span>
    </div>
  </div>
</template>

<style lang="scss" scoped>
.navbar {
  width: 100%;
  height: 48px;
  overflow: hidden;

  .hamburger-container {
    float: left;
    height: 100%;
    line-height: 48px;
    cursor: pointer;
  }

  .locale-select {
    width: 108px;
    margin-right: 4px;
  }

  .vertical-header-right {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    min-width: 320px;
    height: 48px;
    color: #000000d9;

    .el-dropdown-link {
      display: flex;
      align-items: center;
      justify-content: space-around;
      height: 48px;
      padding: 10px;
      color: #000000d9;
      cursor: pointer;

      p {
        font-size: 14px;
      }

      img {
        width: 22px;
        height: 22px;
        border-radius: 50%;
      }
    }
  }

  .breadcrumb-container {
    float: left;
    margin-left: 16px;
  }

  .mobile-page-title {
    float: left;
    max-width: calc(100vw - 180px);
    margin-left: 8px;
    overflow: hidden;
    font-size: 15px;
    font-weight: 600;
    line-height: 48px;
    color: var(--el-text-color-primary);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
}

.logout {
  width: 120px;

  ::v-deep(.el-dropdown-menu__item) {
    display: inline-flex;
    flex-wrap: wrap;
    min-width: 100%;
  }
}
</style>
