<script setup lang="ts">
import { onMounted, ref } from "vue";
import { ElMessage } from "element-plus";
import { listRiskAlerts, ackRiskAlert } from "@/api/fastgame";
import { canWrite } from "@/utils/role";
import { withRequest } from "@/utils/request";

defineOptions({ name: "RiskAlerts" });

const loading = ref(false);
const table = ref<any[]>([]);

async function load() {
  loading.value = true;
  try {
    const data = await withRequest(() => listRiskAlerts(100), "加载失败");
    table.value = data?.list ?? [];
  } finally {
    loading.value = false;
  }
}

async function onAck(row: any) {
  const ok = await withRequest(() => ackRiskAlert(row.id), "确认失败");
  if (!ok) return;
  ElMessage.success("已确认");
  load();
}

onMounted(load);
</script>

<template>
  <el-card shadow="never" header="RTP 风控告警">
    <el-table v-loading="loading" :data="table" stripe>
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="alertType" label="类型" width="120" />
      <el-table-column prop="merchantCode" label="商户" width="100" />
      <el-table-column prop="gameCode" label="游戏" width="100" />
      <el-table-column prop="rtpPpm" label="RTP(ppm)" width="110" />
      <el-table-column prop="status" label="状态" width="90" />
      <el-table-column label="操作" width="100">
        <template #default="{ row }">
          <el-button
            v-if="canWrite() && row.status === 'open'"
            link
            type="primary"
            @click="onAck(row)"
          >
            确认
          </el-button>
        </template>
      </el-table-column>
    </el-table>
  </el-card>
</template>
