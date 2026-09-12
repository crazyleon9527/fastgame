<script setup lang="ts">
import { onMounted, ref } from "vue";
import { listAuditLogs } from "@/api/fastgame";
import { withRequest } from "@/utils/request";
import { useI18nStoreHook } from "@/store/modules/i18n";

defineOptions({ name: "AuditLogs" });

const i18n = useI18nStoreHook();
const loading = ref(false);
const table = ref<any[]>([]);
const total = ref(0);
const page = ref(1);
const action = ref("");
const username = ref("");

async function load() {
  loading.value = true;
  try {
    const data = await withRequest(
      () =>
        listAuditLogs({
          page: page.value,
          pageSize: 20,
          action: action.value || undefined,
          username: username.value || undefined
        }),
      i18n.t("err.audit_load", "加载审计日志失败")
    );
    table.value = data?.list ?? [];
    total.value = data?.total ?? 0;
  } finally {
    loading.value = false;
  }
}

onMounted(load);
</script>

<template>
  <el-card>
    <template #header>
      <div class="flex justify-between items-center flex-wrap gap-2">
        <span>{{ i18n.t("nav.audit", "审计日志") }}</span>
        <el-space wrap>
          <el-input
            v-model="username"
            clearable
            placeholder="用户名"
            style="width: 120px"
            @keyup.enter="load"
          />
          <el-input
            v-model="action"
            clearable
            placeholder="操作类型"
            style="width: 140px"
            @keyup.enter="load"
          />
          <el-button type="primary" @click="load">查询</el-button>
        </el-space>
      </div>
    </template>
    <el-table v-loading="loading" :data="table" stripe>
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="createdAt" label="时间" width="170">
        <template #default="{ row }">
          {{ new Date(row.createdAt).toLocaleString() }}
        </template>
      </el-table-column>
      <el-table-column prop="username" label="用户" width="100" />
      <el-table-column prop="roleName" label="角色" width="90" />
      <el-table-column prop="action" label="操作" width="140" />
      <el-table-column prop="resourceType" label="资源类型" width="100" />
      <el-table-column prop="resourceId" label="资源 ID" min-width="100" />
      <el-table-column prop="requestPath" label="路径" min-width="160" show-overflow-tooltip />
      <el-table-column prop="clientIp" label="IP" width="120" />
      <el-table-column prop="statusCode" label="状态" width="70" />
    </el-table>
    <el-pagination
      v-model:current-page="page"
      class="mt-4"
      layout="total, prev, pager, next"
      :total="total"
      :page-size="20"
      @current-change="load"
    />
  </el-card>
</template>
