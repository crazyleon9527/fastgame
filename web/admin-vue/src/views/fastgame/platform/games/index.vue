<script setup lang="ts">
import { onMounted, ref } from "vue";
import { ElMessage } from "element-plus";
import {
  listGameCategories,
  listPlatformGames,
  createPlatformGame,
  updatePlatformGame
} from "@/api/fastgame";
import { canWrite, isAdmin } from "@/utils/role";
import { withRequest } from "@/utils/request";
import { formatMinor, formatRtpPpm } from "@/utils/minorMoney";
import { t } from "@/utils/i18n";
import ImageUpload from "@/components/ImageUpload/index.vue";
import ExcelActions from "@/components/ExcelActions/index.vue";

defineOptions({ name: "PlatformGames" });

const loading = ref(false);
const table = ref<any[]>([]);
const total = ref(0);
const page = ref(1);
const gameType = ref("");
const categories = ref<any[]>([]);
const dialog = ref(false);
const editing = ref(false);
const form = ref({
  id: 0,
  gameCode: "",
  name: "",
  categoryId: undefined as number | undefined,
  gameType: "slot",
  defaultRtpPpm: 960000,
  volatility: "medium",
  minBetMinor: 10000,
  maxBetMinor: 10000000,
  clientVersion: "1.0.0",
  thumbnailUrl: "",
  status: 1
});

const statusLabel: Record<number, string> = {
  0: "下架",
  1: "上架",
  2: "维护"
};

async function loadCategories() {
  const data = await withRequest(() => listGameCategories(), "加载分类失败");
  categories.value = data?.list ?? [];
}

async function load() {
  loading.value = true;
  try {
    const params: any = { page: page.value, pageSize: 20 };
    if (gameType.value) params.gameType = gameType.value;
    const data = await withRequest(() => listPlatformGames(params), "加载失败");
    if (!data) return;
    table.value = data?.list ?? [];
    total.value = data?.total ?? 0;
  } finally {
    loading.value = false;
  }
}

function openCreate() {
  editing.value = false;
  form.value = {
    id: 0,
    gameCode: "",
    name: "",
    categoryId: categories.value[0]?.id,
    gameType: "slot",
    defaultRtpPpm: 960000,
    volatility: "medium",
    minBetMinor: 10000,
    maxBetMinor: 10000000,
    clientVersion: "1.0.0",
    thumbnailUrl: "",
    status: 1
  };
  dialog.value = true;
}

function openEdit(row: any) {
  editing.value = true;
  form.value = {
    id: row.id,
    gameCode: row.gameCode,
    name: row.name,
    categoryId: row.categoryId || undefined,
    gameType: row.gameType,
    defaultRtpPpm: row.defaultRtpPpm,
    volatility: row.volatility || "medium",
    minBetMinor: row.minBetMinor,
    maxBetMinor: row.maxBetMinor,
    clientVersion: row.clientVersion || "",
    thumbnailUrl: row.thumbnailUrl || "",
    status: row.status
  };
  dialog.value = true;
}

async function onSubmit() {
  if (editing.value) {
    const ok = await withRequest(
      () =>
        updatePlatformGame(form.value.id, {
          name: form.value.name,
          categoryId: form.value.categoryId,
          defaultRtpPpm: form.value.defaultRtpPpm,
          volatility: form.value.volatility,
          minBetMinor: form.value.minBetMinor,
          maxBetMinor: form.value.maxBetMinor,
          clientVersion: form.value.clientVersion,
          thumbnailUrl: form.value.thumbnailUrl || undefined,
          status: form.value.status
        }),
      "更新失败"
    );
    if (!ok) return;
    ElMessage.success("已更新");
  } else {
    const ok = await withRequest(
      () => createPlatformGame(form.value),
      "创建失败"
    );
    if (!ok) return;
    ElMessage.success("游戏已上架");
  }
  dialog.value = false;
  load();
}

onMounted(async () => {
  await loadCategories();
  load();
});
</script>

<template>
  <el-card shadow="never">
    <template #header>
      <div class="flex justify-between items-center flex-wrap gap-2">
        <span>{{ t("nav.platform_games", "平台游戏目录") }}</span>
        <el-space wrap>
          <el-select
            v-model="gameType"
            clearable
            placeholder="游戏类型"
            style="width: 130px"
            @change="load"
          >
            <el-option label="fishing" value="fishing" />
            <el-option label="slot" value="slot" />
            <el-option label="crash" value="crash" />
            <el-option label="table" value="table" />
          </el-select>
          <el-button v-if="isAdmin()" type="primary" @click="openCreate">
            上架游戏
          </el-button>
          <ExcelActions
            export-path="/export/platform-games"
            :import-path="isAdmin() ? '/import/platform-games' : undefined"
            @imported="load"
          />
          <el-button @click="load">刷新</el-button>
        </el-space>
      </div>
    </template>
    <el-table v-loading="loading" :data="table" stripe>
      <el-table-column label="缩略图" width="72">
        <template #default="{ row }">
          <img
            v-if="row.thumbnailUrl"
            :src="row.thumbnailUrl"
            alt=""
            class="thumb"
          />
          <span v-else class="muted">—</span>
        </template>
      </el-table-column>
      <el-table-column prop="gameCode" label="编码" width="120" />
      <el-table-column prop="name" label="名称" min-width="140" />
      <el-table-column prop="categoryName" label="分类" width="100" />
      <el-table-column prop="gameType" label="类型" width="90" />
      <el-table-column label="RTP" width="90">
        <template #default="{ row }">{{ formatRtpPpm(row.defaultRtpPpm) }}</template>
      </el-table-column>
      <el-table-column label="注额范围" min-width="160">
        <template #default="{ row }">
          {{ formatMinor(row.minBetMinor) }} – {{ formatMinor(row.maxBetMinor) }}
        </template>
      </el-table-column>
      <el-table-column prop="clientVersion" label="版本" width="90" />
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-tag
            :type="row.status === 1 ? 'success' : row.status === 2 ? 'warning' : 'info'"
          >
            {{ statusLabel[row.status] ?? row.status }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="90">
        <template #default="{ row }">
          <el-button v-if="canWrite()" link type="primary" @click="openEdit(row)">
            编辑
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

  <el-dialog
    v-model="dialog"
    :title="editing ? '编辑游戏' : '上架新游戏'"
    width="520px"
  >
    <el-form label-width="100px">
      <el-form-item v-if="!editing" label="游戏编码">
        <el-input v-model="form.gameCode" placeholder="slot-demo" />
      </el-form-item>
      <el-form-item label="名称">
        <el-input v-model="form.name" />
      </el-form-item>
      <el-form-item label="分类">
        <el-select v-model="form.categoryId" style="width: 100%">
          <el-option
            v-for="c in categories"
            :key="c.id"
            :label="c.name"
            :value="c.id"
          />
        </el-select>
      </el-form-item>
      <el-form-item label="类型">
        <el-select v-model="form.gameType" style="width: 100%">
          <el-option label="fishing" value="fishing" />
          <el-option label="slot" value="slot" />
          <el-option label="crash" value="crash" />
          <el-option label="table" value="table" />
        </el-select>
      </el-form-item>
      <el-form-item label="RTP (ppm)">
        <el-input-number v-model="form.defaultRtpPpm" :min="800000" :max="990000" />
        <span class="hint">{{ formatRtpPpm(form.defaultRtpPpm) }}</span>
      </el-form-item>
      <el-form-item label="最小注">
        <el-input-number v-model="form.minBetMinor" :min="1" :step="1000" />
        <span class="hint">{{ formatMinor(form.minBetMinor) }}</span>
      </el-form-item>
      <el-form-item label="最大注">
        <el-input-number v-model="form.maxBetMinor" :min="1" :step="10000" />
        <span class="hint">{{ formatMinor(form.maxBetMinor) }}</span>
      </el-form-item>
      <el-form-item label="客户端版本">
        <el-input v-model="form.clientVersion" />
      </el-form-item>
      <el-form-item label="缩略图">
        <ImageUpload v-model="form.thumbnailUrl" />
      </el-form-item>
      <el-form-item label="状态">
        <el-radio-group v-model="form.status">
          <el-radio :value="1">上架</el-radio>
          <el-radio :value="0">下架</el-radio>
          <el-radio :value="2">维护</el-radio>
        </el-radio-group>
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="dialog = false">{{ t("btn.cancel", "取消") }}</el-button>
      <el-button type="primary" @click="onSubmit">
        {{ t("btn.save", "保存") }}
      </el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.hint {
  margin-left: 8px;
  color: var(--el-text-color-secondary);
  font-size: 12px;
}

.thumb {
  width: 40px;
  height: 40px;
  object-fit: cover;
  border-radius: 4px;
}

.muted {
  color: var(--el-text-color-secondary);
}
</style>
