<script setup lang="ts">
import { onMounted, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import {
  listMerchants,
  createMerchant,
  updateMerchant,
  rotateMerchantKey,
  listWalletBreakers,
  resetWalletBreaker
} from "@/api/fastgame";
import { canWrite, isAdmin } from "@/utils/role";
import { confirmAction, withRequest } from "@/utils/request";
import ExcelActions from "@/components/ExcelActions/index.vue";

defineOptions({ name: "MerchantList" });

const loading = ref(false);
const table = ref<any[]>([]);
const total = ref(0);
const page = ref(1);
const breakers = ref<string[]>([]);
const dialog = ref(false);
const form = ref({ merchantCode: "", name: "", status: 1 });

async function load() {
  loading.value = true;
  try {
    const data = await withRequest(
      () => listMerchants(page.value, 20),
      "加载商户失败"
    );
    table.value = data?.list ?? [];
    total.value = data?.total ?? 0;
    const b = await withRequest(() => listWalletBreakers(), "加载熔断状态失败");
    const list = Array.isArray(b?.list) ? b.list : Array.isArray(b) ? b : [];
    breakers.value = list
      .filter((x: any) => x?.open !== false)
      .map((x: any) => x.merchantCode ?? x);
  } finally {
    loading.value = false;
  }
}

async function onCreate() {
  const ok = await withRequest(() => createMerchant(form.value), "创建失败");
  if (!ok) return;
  ElMessage.success("商户已创建");
  dialog.value = false;
  load();
}

async function toggleStatus(row: any) {
  if (!canWrite()) return;
  const status = row.status === 1 ? 0 : 1;
  const ok = await withRequest(
    () => updateMerchant(row.id, { name: row.name, status }),
    "更新失败"
  );
  if (!ok) return;
  ElMessage.success("状态已更新");
  load();
}

async function onRotate(row: any) {
  if (!isAdmin()) return;
  if (
    !(await confirmAction(
      `确认轮换商户 ${row.merchantCode} 的密钥？`,
      "密钥轮换"
    ))
  ) {
    return;
  }
  const res = await withRequest(() => rotateMerchantKey(row.id), "轮换失败");
  if (!res) return;
  ElMessageBox.alert(
    res?.newPrivateKey ?? res?.privateKey ?? "请从响应中复制新密钥",
    "新私钥（仅展示一次）"
  );
  load();
}

async function resetBreaker(code: string) {
  const ok = await withRequest(() => resetWalletBreaker(code), "重置失败");
  if (!ok) return;
  ElMessage.success("熔断已重置");
  load();
}

onMounted(load);
</script>

<template>
  <div>
    <el-alert
      v-if="breakers.length"
      type="error"
      :closable="false"
      class="mb-4"
      title="钱包熔断中"
    >
      <el-space wrap>
        <el-tag v-for="c in breakers" :key="c" type="danger">
          {{ c }}
          <el-button v-if="canWrite()" link type="primary" @click="resetBreaker(c)">
            重置
          </el-button>
        </el-tag>
      </el-space>
    </el-alert>
    <el-card shadow="never">
      <template #header>
        <div class="flex justify-between items-center">
          <span>商户列表</span>
          <el-space>
            <el-button v-if="isAdmin()" type="primary" @click="dialog = true">
              新建商户
            </el-button>
            <ExcelActions export-path="/export/merchants" @imported="load" />
          </el-space>
        </div>
      </template>
      <el-table v-loading="loading" :data="table" stripe>
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="merchantCode" label="编码" />
        <el-table-column prop="name" label="名称" />
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : 'info'">
              {{ row.status === 1 ? "启用" : "禁用" }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="220">
          <template #default="{ row }">
            <el-button v-if="canWrite()" link type="primary" @click="toggleStatus(row)">
              {{ row.status === 1 ? "禁用" : "启用" }}
            </el-button>
            <el-button v-if="isAdmin()" link type="warning" @click="onRotate(row)">
              轮换密钥
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
    <el-dialog v-model="dialog" title="新建商户" width="420px">
      <el-form label-width="88px">
        <el-form-item label="商户编码">
          <el-input v-model="form.merchantCode" />
        </el-form-item>
        <el-form-item label="名称">
          <el-input v-model="form.name" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialog = false">取消</el-button>
        <el-button type="primary" @click="onCreate">创建</el-button>
      </template>
    </el-dialog>
  </div>
</template>
