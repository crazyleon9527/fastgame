<script setup lang="ts">
import { onMounted, ref } from "vue";
import { ElMessage } from "element-plus";
import {
  listSettlementPeriods,
  rollupSettlementPeriod
} from "@/api/fastgame";
import { canWrite } from "@/utils/role";
import { formatMoney } from "@/utils/money";
import { withRequest } from "@/utils/request";
import { t } from "@/utils/i18n";
import MerchantSelect from "@/components/MerchantSelect/index.vue";

defineOptions({ name: "SettlementPeriods" });

const loading = ref(false);
const table = ref<any[]>([]);
const total = ref(0);
const page = ref(1);
const merchantId = ref<number>();
const periodType = ref<string>("weekly");

const periodTypes = [
  { value: "", label: "全部" },
  { value: "weekly", label: "周结算" },
  { value: "monthly", label: "月结算" }
];

const statusMap: Record<string, string> = {
  draft: "草稿",
  pending_review: "待审核",
  confirmed: "已确认",
  invoiced: "已开票",
  paid: "已付款",
  cancelled: "已取消"
};

async function load() {
  loading.value = true;
  try {
    const params: any = { page: page.value, pageSize: 20 };
    if (merchantId.value) params.merchantId = merchantId.value;
    if (periodType.value) params.periodType = periodType.value;
    const data = await withRequest(
      () => listSettlementPeriods(params),
      "加载失败"
    );
    if (!data) return;
    table.value = data?.list ?? [];
    total.value = data?.total ?? 0;
  } finally {
    loading.value = false;
  }
}

async function onRollup(row: any) {
  if (row.status !== "draft") {
    ElMessage.warning("仅草稿状态的周期可重新汇总");
    return;
  }
  const ok = await withRequest(
    () => rollupSettlementPeriod(row.id),
    "汇总失败"
  );
  if (!ok) return;
  ElMessage.success("已从已确认日结算重新汇总");
  load();
}

onMounted(load);
</script>

<template>
  <el-card shadow="never">
    <template #header>
      <div class="flex justify-between items-center flex-wrap gap-2">
        <span>{{ t("nav.settlement_periods", "结算周期") }}</span>
        <el-space wrap>
          <MerchantSelect
            v-model="merchantId"
            placeholder="全部商户"
            @change="load"
          />
          <el-select
            v-model="periodType"
            class="w-32"
            @change="load"
          >
            <el-option
              v-for="item in periodTypes"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </el-select>
          <el-button @click="load">刷新</el-button>
        </el-space>
      </div>
    </template>
    <el-table v-loading="loading" :data="table" stripe>
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="merchantCode" label="商户" min-width="110" />
      <el-table-column label="周期类型" width="90">
        <template #default="{ row }">
          {{ row.periodType === "monthly" ? "月" : "周" }}
        </template>
      </el-table-column>
      <el-table-column label="周期" min-width="180">
        <template #default="{ row }">
          {{ row.periodStart }} ~ {{ row.periodEnd }}
        </template>
      </el-table-column>
      <el-table-column prop="totalBet" label="总投注">
        <template #default="{ row }">{{ formatMoney(row.totalBet) }}</template>
      </el-table-column>
      <el-table-column prop="totalWin" label="总派彩">
        <template #default="{ row }">{{ formatMoney(row.totalWin) }}</template>
      </el-table-column>
      <el-table-column prop="ggr" label="GGR">
        <template #default="{ row }">{{ formatMoney(row.ggr) }}</template>
      </el-table-column>
      <el-table-column prop="netPayable" label="应付净值">
        <template #default="{ row }">{{ formatMoney(row.netPayable) }}</template>
      </el-table-column>
      <el-table-column prop="totalRounds" label="注单数" width="90" />
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-tag
            :type="row.status === 'confirmed' || row.status === 'paid' ? 'success' : row.status === 'draft' ? 'info' : 'warning'"
          >
            {{ statusMap[row.status] ?? row.status }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="110">
        <template #default="{ row }">
          <el-button
            v-if="canWrite() && row.status === 'draft'"
            link
            type="primary"
            @click="onRollup(row)"
          >
            重新汇总
          </el-button>
        </template>
      </el-table-column>
    </el-table>
    <el-pagination
      v-model:current-page="page"
      class="mt-4"
      layout="prev, pager, next, total"
      :total="total"
      @current-change="load"
    />
  </el-card>
</template>
