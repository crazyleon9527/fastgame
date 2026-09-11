<script setup lang="ts">
import { onMounted, ref } from "vue";
import { ElMessage } from "element-plus";
import { listBlacklist, createBlacklist, deleteBlacklist } from "@/api/fastgame";
import { canWrite } from "@/utils/role";
import { confirmAction, withRequest } from "@/utils/request";

defineOptions({ name: "RiskBlacklist" });

const loading = ref(false);
const table = ref<any[]>([]);
const total = ref(0);
const page = ref(1);
const listType = ref("");
const dialog = ref(false);
const form = ref({
  listType: "ip",
  listValue: "",
  reason: "",
  expiresAt: ""
});

async function load() {
  loading.value = true;
  try {
    const data = await withRequest(
      () =>
        listBlacklist({
          page: page.value,
          pageSize: 20,
          listType: listType.value || undefined
        }),
      "加载失败"
    );
    table.value = data?.list ?? [];
    total.value = data?.total ?? 0;
  } finally {
    loading.value = false;
  }
}

async function onAdd() {
  const ok = await withRequest(() => createBlacklist(form.value), "添加失败");
  if (!ok) return;
  ElMessage.success("已添加");
  dialog.value = false;
  load();
}

async function onDelete(row: any) {
  if (!(await confirmAction("确认解除封禁？", "提示"))) return;
  const ok = await withRequest(() => deleteBlacklist(row.id), "删除失败");
  if (!ok) return;
  ElMessage.success("已删除");
  load();
}

onMounted(load);
</script>

<template>
  <el-card shadow="never">
    <template #header>
      <div class="flex justify-between items-center flex-wrap gap-2">
        <el-space>
          <span>风控黑名单</span>
          <el-select v-model="listType" clearable placeholder="类型" style="width: 120px" @change="load">
            <el-option label="IP" value="ip" />
            <el-option label="用户" value="user_id" />
            <el-option label="商户" value="merchant" />
          </el-select>
        </el-space>
        <el-button v-if="canWrite()" type="primary" @click="dialog = true">添加</el-button>
      </div>
    </template>
    <el-table v-loading="loading" :data="table" stripe>
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="listType" label="类型" width="100" />
      <el-table-column prop="listValue" label="值" />
      <el-table-column prop="reason" label="原因" show-overflow-tooltip />
      <el-table-column prop="status" label="状态" width="90" />
      <el-table-column label="操作" width="100">
        <template #default="{ row }">
          <el-button v-if="canWrite()" link type="danger" @click="onDelete(row)">解除</el-button>
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
  <el-dialog v-model="dialog" title="添加封禁" width="440px">
    <el-form label-width="80px">
      <el-form-item label="类型">
        <el-select v-model="form.listType">
          <el-option label="IP" value="ip" />
          <el-option label="用户 ID" value="user_id" />
          <el-option label="商户" value="merchant" />
        </el-select>
      </el-form-item>
      <el-form-item label="值"><el-input v-model="form.listValue" /></el-form-item>
      <el-form-item label="原因"><el-input v-model="form.reason" /></el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="dialog = false">取消</el-button>
      <el-button type="primary" @click="onAdd">确定</el-button>
    </template>
  </el-dialog>
</template>
