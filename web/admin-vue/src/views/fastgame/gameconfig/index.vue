<script setup lang="ts">
import { ref } from "vue";
import { ElMessage } from "element-plus";
import {
  listGameConfigs,
  upsertGameConfig,
  deleteGameConfig
} from "@/api/fastgame";
import { canWrite } from "@/utils/role";
import { withRequest } from "@/utils/request";
import { t } from "@/utils/i18n";
import MerchantSelect from "@/components/MerchantSelect/index.vue";

defineOptions({ name: "GameConfig" });

const loading = ref(false);
const table = ref<any[]>([]);
const merchantId = ref<number>();
const gameCode = ref("");
const jsonVal = ref('{"rtpTier":"default"}');

async function load() {
  if (!merchantId.value) return;
  loading.value = true;
  try {
    const data = await withRequest(
      () => listGameConfigs(merchantId.value!, gameCode.value),
      "查询失败"
    );
    table.value = data?.list ?? [];
  } finally {
    loading.value = false;
  }
}

async function onSave() {
  if (!merchantId.value) {
    ElMessage.warning("请先选择商户");
    return;
  }
  let parsed: string;
  try {
    parsed = JSON.stringify(JSON.parse(jsonVal.value));
  } catch {
    ElMessage.error("JSON 格式无效，请检查配置内容");
    return;
  }
  const ok = await withRequest(
    () =>
      upsertGameConfig({
        merchantId: merchantId.value,
        gameCode: gameCode.value || "fishing",
        configKey: "params",
        configValue: parsed,
        status: 1
      }),
    "保存失败"
  );
  if (!ok) return;
  ElMessage.success("已保存");
  load();
}

async function onDelete(row: any) {
  const ok = await withRequest(() => deleteGameConfig(row.id), "删除失败");
  if (!ok) return;
  ElMessage.success("已删除");
  load();
}
</script>

<template>
  <el-card shadow="never" :header="t('nav.games', '游戏参数配置')">
    <el-form inline class="mb-4">
      <el-form-item label="商户">
        <MerchantSelect v-model="merchantId" @change="load" />
      </el-form-item>
      <el-form-item label="游戏">
        <el-input v-model="gameCode" placeholder="fishing" style="width: 140px" />
      </el-form-item>
      <el-form-item>
        <el-button type="primary" @click="load">查询</el-button>
      </el-form-item>
    </el-form>
    <el-input
      v-if="canWrite()"
      v-model="jsonVal"
      type="textarea"
      :rows="4"
      class="mb-4"
      placeholder="JSON 配置"
    />
    <el-button v-if="canWrite()" type="primary" class="mb-4" @click="onSave">
      {{ t("btn.save", "保存配置") }}
    </el-button>
    <el-table v-loading="loading" :data="table" stripe>
      <el-table-column prop="gameCode" label="游戏" />
      <el-table-column prop="configKey" label="键" />
      <el-table-column prop="configValue" label="值" show-overflow-tooltip />
      <el-table-column label="操作" width="90">
        <template #default="{ row }">
          <el-button v-if="canWrite()" link type="danger" @click="onDelete(row)">
            删除
          </el-button>
        </template>
      </el-table-column>
    </el-table>
  </el-card>
</template>
