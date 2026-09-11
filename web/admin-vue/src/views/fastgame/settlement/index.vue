<script setup lang="ts">
import { onMounted, ref } from "vue";
import { ElMessage } from "element-plus";
import {
  listDailySettlements,
  syncDailySettlements,
  confirmDailySettlement
} from "@/api/fastgame";
import { canWrite } from "@/utils/role";
import { formatMoney } from "@/utils/money";
import { withRequest } from "@/utils/request";
import { t } from "@/utils/i18n";
import MerchantSelect from "@/components/MerchantSelect/index.vue";

defineOptions({ name: "DailySettlement" });

const loading = ref(false);
const table = ref<any[]>([]);
const total = ref(0);
const page = ref(1);
const merchantId = ref<number>();

async function load() {
  loading.value = true;
  try {
    const params: any = { page: page.value, pageSize: 20 };
    if (merchantId.value) params.merchantId = merchantId.value;
    const data = await withRequest(
      () => listDailySettlements(params),
      "加载失败"
    );
    if (!data) return;
    table.value = data?.list ?? [];
    total.value = data?.total ?? 0;
  } finally {
    loading.value = false;
  }
}

async function onSync() {
  const res = await withRequest(
    () => syncDailySettlements({ days: 7, merchantId: merchantId.value }),
    "同步失败"
  );
  if (!res) return;
  ElMessage.success(`已同步 ${res?.synced ?? 0} 条`);
  load();
}

async function onConfirm(row: any) {
  const ok = await withRequest(
    () => confirmDailySettlement(row.id),
    "确认失败"
  );
  if (!ok) return;
  ElMessage.success("已确认");
  load();
}

onMounted(load);
</script>

<template>
  <el-card shadow="never">
    <template #header>
      <div class="flex justify-between items-center flex-wrap gap-2">
        <span>{{ t("nav.settlements", "日结算对账") }}</span>
        <el-space wrap>
          <MerchantSelect
            v-model="merchantId"
            placeholder="全部商户"
            @change="load"
          />
          <el-button v-if="canWrite()" type="primary" @click="onSync">
            同步近 7 天
          </el-button>
          <el-button @click="load">刷新</el-button>
        </el-space>
      </div>
    </template>
    <el-table v-loading="loading" :data="table" stripe>
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="merchantId" label="商户" width="90" />
      <el-table-column prop="settleDate" label="日期" width="120" />
      <el-table-column prop="totalBet" label="总投注">
        <template #default="{ row }">{{ formatMoney(row.totalBet) }}</template>
      </el-table-column>
      <el-table-column prop="totalWin" label="总派彩">
        <template #default="{ row }">{{ formatMoney(row.totalWin) }}</template>
      </el-table-column>
      <el-table-column prop="status" label="状态" width="110" />
      <el-table-column label="操作" width="100">
        <template #default="{ row }">
          <el-button
            v-if="canWrite() && row.status !== 'confirmed'"
            link
            type="primary"
            @click="onConfirm(row)"
          >
            确认
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
