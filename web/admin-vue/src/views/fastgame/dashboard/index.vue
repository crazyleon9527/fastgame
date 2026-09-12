<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { ElMessage } from "element-plus";
import { isTotpPending } from "@/utils/totp";
import {
  listMerchants,
  listRiskAlerts,
  listDailySettlements,
  listWalletBreakers
} from "@/api/fastgame";
import { withRequest } from "@/utils/request";
import { useI18nStoreHook } from "@/store/modules/i18n";
import { useRenderIcon } from "@/components/ReIcon/src/hooks";
import Shop from "~icons/ep/office-building";
import Warning from "~icons/ep/warning";
import Money from "~icons/ep/money";
import Switch from "~icons/ep/switch-button";

defineOptions({ name: "Dashboard" });

const router = useRouter();
const i18n = useI18nStoreHook();
const loading = ref(true);
const stats = ref({
  merchants: 0,
  activeMerchants: 0,
  openAlerts: 0,
  pendingSettlements: 0,
  breakers: 0
});

onMounted(async () => {
  loading.value = true;
  try {
    const [merchants, alerts, settlements, breakers] = await Promise.all([
      withRequest(() => listMerchants(1, 100), "", true),
      withRequest(() => listRiskAlerts(100), "", true),
      withRequest(() => listDailySettlements({ page: 1, pageSize: 50 }), "", true),
      withRequest(() => listWalletBreakers(), "", true)
    ]);
    if (merchants) {
      const mList = merchants.list ?? [];
      stats.value.merchants = merchants.total ?? mList.length;
      stats.value.activeMerchants = mList.filter((m: any) => m.status === 1).length;
    }
    if (alerts) {
      stats.value.openAlerts = (alerts.list ?? []).filter(
        (a: any) => a.status === "open"
      ).length;
    }
    if (settlements) {
      stats.value.pendingSettlements = (settlements.list ?? []).filter(
        (s: any) => s.status !== "confirmed"
      ).length;
    }
    if (breakers) {
      const list = Array.isArray(breakers?.list)
        ? breakers.list
        : Array.isArray(breakers)
          ? breakers
          : [];
      stats.value.breakers = list.filter((x: any) => x?.open !== false).length;
    }
    const failed = [merchants, alerts, settlements, breakers].filter(v => !v).length;
    if (failed && !isTotpPending()) {
      ElMessage.warning(`部分统计数据加载失败（${failed} 项）`);
    }
  } finally {
    loading.value = false;
  }
});

const cards = computed(() => [
  {
    key: "merchants",
    title: i18n.t("dash.merchants", "商户总数"),
    field: "merchants",
    sub: "activeMerchants",
    subLabel: i18n.t("dash.active", "已启用"),
    icon: Shop,
    color: "#6366f1",
    path: "/merchant/list"
  },
  {
    key: "alerts",
    title: i18n.t("dash.alerts", "RTP 告警"),
    field: "openAlerts",
    sub: null,
    subLabel: "",
    icon: Warning,
    color: "#f59e0b",
    path: "/risk/alerts"
  },
  {
    key: "settlement",
    title: i18n.t("dash.settlements", "待确认结算"),
    field: "pendingSettlements",
    sub: null,
    subLabel: "",
    icon: Money,
    color: "#10b981",
    path: "/finance/settlement"
  },
  {
    key: "breaker",
    title: i18n.t("dash.breakers", "钱包熔断"),
    field: "breakers",
    sub: null,
    subLabel: "",
    icon: Switch,
    color: "#ef4444",
    path: "/merchant/list"
  }
]);
</script>

<template>
  <div v-loading="loading" class="dashboard">
    <el-alert
      v-if="isTotpPending()"
      type="warning"
      :closable="false"
      show-icon
      class="mb-4"
      title="账号尚未完成 2FA 绑定，请先完成 Google Authenticator 绑定后再使用各功能模块。"
    />
    <div class="hero">
      <div>
        <h1>{{ i18n.t("dash.title", "FastGame 运营控制台") }}</h1>
        <p>{{ i18n.t("dash.subtitle", "多商户 · 自研游戏 · 实时风控与对账") }}</p>
      </div>
    </div>
    <el-row :gutter="16" class="stat-row">
      <el-col v-for="c in cards" :key="c.key" :xs="24" :sm="12" :lg="6">
        <el-card
          shadow="hover"
          class="stat-card"
          @click="router.push(c.path)"
        >
          <div class="stat-inner">
            <div
              class="stat-icon"
              :style="{ background: c.color + '22', color: c.color }"
            >
              <component :is="useRenderIcon(c.icon)" />
            </div>
            <div>
              <div class="stat-title">{{ c.title }}</div>
              <div class="stat-value">{{ stats[c.field as keyof typeof stats] }}</div>
              <div v-if="c.sub" class="stat-sub">
                {{ c.subLabel }}：{{ stats[c.sub as keyof typeof stats] }}
              </div>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>
    <el-row :gutter="16">
      <el-col :span="24">
        <el-card :header="i18n.t('dash.shortcuts', '快捷入口')">
          <el-space wrap>
            <el-button type="primary" @click="router.push('/merchant/list')">
              {{ i18n.t("nav.merchants", "商户管理") }}
            </el-button>
            <el-button @click="router.push('/finance/rtp')">
              {{ i18n.t("nav.rtp", "RTP 报表") }}
            </el-button>
            <el-button @click="router.push('/risk/blacklist')">
              {{ i18n.t("nav.blacklist", "风控黑名单") }}
            </el-button>
            <el-button @click="router.push('/ops/trace')">
              {{ i18n.t("nav.trace", "Trace 排查") }}
            </el-button>
          </el-space>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<style scoped>
.dashboard {
  padding: 8px;
}
.hero {
  margin-bottom: 20px;
  padding: 24px 28px;
  border-radius: 12px;
  background: linear-gradient(135deg, #6366f1 0%, #8b5cf6 50%, #a855f7 100%);
  color: #fff;
}
.hero h1 {
  margin: 0 0 8px;
  font-size: 1.5rem;
  font-weight: 600;
}
.hero p {
  margin: 0;
  opacity: 0.9;
}
.stat-row {
  margin-bottom: 16px;
}
.stat-card {
  cursor: pointer;
  border-radius: 12px;
  margin-bottom: 16px;
}
.stat-inner {
  display: flex;
  gap: 16px;
  align-items: center;
}
.stat-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 48px;
  height: 48px;
  border-radius: 12px;
  font-size: 22px;
}
.stat-title {
  color: var(--el-text-color-secondary);
  font-size: 13px;
}
.stat-value {
  font-size: 28px;
  font-weight: 700;
  line-height: 1.2;
}
.stat-sub {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
</style>
