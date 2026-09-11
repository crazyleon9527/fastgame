<script setup lang="ts">
import { onMounted, ref } from "vue";
import { getRtpReport } from "@/api/fastgame";
import { formatMoney } from "@/utils/money";
import { withRequest } from "@/utils/request";

defineOptions({ name: "RtpReport" });

const loading = ref(false);
const table = ref<any[]>([]);
const hours = ref(24);

async function load() {
  loading.value = true;
  try {
    const data = await withRequest(
      () => getRtpReport({ hours: hours.value }),
      "加载失败"
    );
    table.value = data?.list ?? data ?? [];
  } finally {
    loading.value = false;
  }
}

onMounted(load);
</script>

<template>
  <el-card shadow="never">
    <template #header>
      <div class="flex justify-between items-center">
        <span>RTP 报表（ClickHouse）</span>
        <el-space>
          <el-select v-model="hours" style="width: 120px" @change="load">
            <el-option :value="6" label="近 6 小时" />
            <el-option :value="24" label="近 24 小时" />
            <el-option :value="168" label="近 7 天" />
          </el-select>
          <el-button @click="load">刷新</el-button>
        </el-space>
      </div>
    </template>
    <el-table v-loading="loading" :data="table" stripe max-height="560">
      <el-table-column prop="merchantId" label="商户" width="90" />
      <el-table-column prop="gameCode" label="游戏" width="120" />
      <el-table-column prop="totalBet" label="总投注">
        <template #default="{ row }">{{ formatMoney(row.totalBet) }}</template>
      </el-table-column>
      <el-table-column prop="totalWin" label="总派彩">
        <template #default="{ row }">{{ formatMoney(row.totalWin) }}</template>
      </el-table-column>
      <el-table-column prop="totalRounds" label="局数" width="100" />
      <el-table-column prop="rtpPpm" label="RTP(ppm)" width="110" />
    </el-table>
  </el-card>
</template>
