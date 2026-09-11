<script setup lang="ts">
import { onMounted, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import {
  listPlatformGames,
  listMerchantGames,
  createMerchantGame,
  updateMerchantGame,
  deleteMerchantGame,
  listGameRtpTiers
} from "@/api/fastgame";
import { canWrite } from "@/utils/role";
import { confirmAction, withRequest } from "@/utils/request";
import { formatMinor, formatRtpPpm } from "@/utils/minorMoney";
import { t } from "@/utils/i18n";
import MerchantSelect from "@/components/MerchantSelect/index.vue";
import ExcelActions from "@/components/ExcelActions/index.vue";

defineOptions({ name: "MerchantGames" });

const loading = ref(false);
const table = ref<any[]>([]);
const merchantId = ref<number>();
const platformGames = ref<any[]>([]);
const rtpTiers = ref<any[]>([]);
const dialog = ref(false);
const editDialog = ref(false);
const form = ref({
  gameId: undefined as number | undefined,
  rtpTierCode: "default",
  sortOrder: 10,
  minBetMinor: undefined as number | undefined,
  maxBetMinor: undefined as number | undefined
});
const editForm = ref({
  id: 0,
  gameName: "",
  rtpTierCode: "default",
  sortOrder: 10,
  minBetMinor: undefined as number | undefined,
  maxBetMinor: undefined as number | undefined,
  status: 1
});

async function loadPlatformGames() {
  const data = await withRequest(
    () => listPlatformGames({ page: 1, pageSize: 100 }),
    "加载平台游戏失败"
  );
  platformGames.value = (data?.list ?? []).filter((g: any) => g.status === 1);
}

async function load() {
  if (!merchantId.value) {
    table.value = [];
    return;
  }
  loading.value = true;
  try {
    const data = await withRequest(
      () => listMerchantGames(merchantId.value!, 1, 100),
      "加载失败"
    );
    table.value = data?.list ?? [];
  } finally {
    loading.value = false;
  }
}

const availableGames = ref<any[]>([]);

function refreshAvailable() {
  const opened = new Set(table.value.map(r => r.gameId));
  availableGames.value = platformGames.value.filter(
    (g: any) => !opened.has(g.id)
  );
}

async function openAdd() {
  if (!merchantId.value) {
    ElMessage.warning("请先选择商户");
    return;
  }
  refreshAvailable();
  if (!availableGames.value.length) {
    ElMessage.info("该商户已开通全部平台游戏");
    return;
  }
  form.value = {
    gameId: availableGames.value[0]?.id,
    rtpTierCode: "default",
    sortOrder: (table.value.length + 1) * 10,
    minBetMinor: undefined,
    maxBetMinor: undefined
  };
  if (form.value.gameId) await loadTiers(form.value.gameId);
  dialog.value = true;
}

async function loadTiers(gameId: number) {
  const data = await withRequest(
    () => listGameRtpTiers(gameId),
    "加载 RTP 档位失败"
  );
  rtpTiers.value = data?.list ?? [];
  if (
    rtpTiers.value.length &&
    !rtpTiers.value.find(t => t.tierCode === form.value.rtpTierCode)
  ) {
    form.value.rtpTierCode = rtpTiers.value[0].tierCode;
  }
}

watch(
  () => form.value.gameId,
  id => {
    if (id) loadTiers(id);
  }
);

async function onAdd() {
  if (!merchantId.value || !form.value.gameId) return;
  const payload: any = {
    merchantId: merchantId.value,
    gameId: form.value.gameId,
    rtpTierCode: form.value.rtpTierCode,
    sortOrder: form.value.sortOrder
  };
  if (form.value.minBetMinor) payload.minBetMinor = form.value.minBetMinor;
  if (form.value.maxBetMinor) payload.maxBetMinor = form.value.maxBetMinor;
  const ok = await withRequest(() => createMerchantGame(payload), "开通失败");
  if (!ok) return;
  ElMessage.success("游戏已开通");
  dialog.value = false;
  load();
}

function openEdit(row: any) {
  editForm.value = {
    id: row.id,
    gameName: row.gameName,
    rtpTierCode: row.rtpTierCode || "default",
    sortOrder: row.sortOrder,
    minBetMinor: row.minBetMinor || undefined,
    maxBetMinor: row.maxBetMinor || undefined,
    status: row.status
  };
  loadTiers(row.gameId);
  editDialog.value = true;
}

async function onSaveEdit() {
  const ok = await withRequest(
    () =>
      updateMerchantGame(editForm.value.id, {
        rtpTierCode: editForm.value.rtpTierCode,
        sortOrder: editForm.value.sortOrder,
        minBetMinor: editForm.value.minBetMinor,
        maxBetMinor: editForm.value.maxBetMinor,
        status: editForm.value.status
      }),
    "更新失败"
  );
  if (!ok) return;
  ElMessage.success("已更新");
  editDialog.value = false;
  load();
}

async function onRemove(row: any) {
  if (
    !(await confirmAction(`确认关闭商户游戏 ${row.gameName}？`, "提示"))
  ) {
    return;
  }
  const ok = await withRequest(() => deleteMerchantGame(row.id), "删除失败");
  if (!ok) return;
  ElMessage.success("已关闭");
  load();
}

onMounted(loadPlatformGames);
</script>

<template>
  <el-card shadow="never">
    <template #header>
      <div class="flex justify-between items-center flex-wrap gap-2">
        <span>{{ t("nav.merchant_games", "商户游戏大厅") }}</span>
        <el-space wrap>
          <MerchantSelect v-model="merchantId" @change="load" />
          <el-button v-if="canWrite()" type="primary" @click="openAdd">
            开通游戏
          </el-button>
          <ExcelActions
            export-path="/export/merchant-games"
            :export-params="merchantId ? { merchantId } : undefined"
            import-path="/import/merchant-games"
            :disabled="!merchantId"
            @imported="load"
          />
          <el-button @click="load">刷新</el-button>
        </el-space>
      </div>
    </template>

    <el-empty v-if="!merchantId" description="请选择商户查看已开通游戏" />
    <el-table v-else v-loading="loading" :data="table" stripe>
      <el-table-column prop="sortOrder" label="排序" width="70" />
      <el-table-column prop="gameCode" label="编码" width="120" />
      <el-table-column prop="gameName" label="游戏" min-width="140" />
      <el-table-column prop="gameType" label="类型" width="90" />
      <el-table-column prop="rtpTierCode" label="RTP 档位" width="100" />
      <el-table-column label="平台 RTP" width="100">
        <template #default="{ row }">{{ formatRtpPpm(row.defaultRtpPpm) }}</template>
      </el-table-column>
      <el-table-column label="注额覆盖" min-width="140">
        <template #default="{ row }">
          <span v-if="row.minBetMinor || row.maxBetMinor">
            {{ formatMinor(row.minBetMinor) }} – {{ formatMinor(row.maxBetMinor) }}
          </span>
          <span v-else class="muted">默认</span>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'info'">
            {{ row.status === 1 ? "开通" : "关闭" }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="140">
        <template #default="{ row }">
          <el-button v-if="canWrite()" link type="primary" @click="openEdit(row)">
            编辑
          </el-button>
          <el-button v-if="canWrite()" link type="danger" @click="onRemove(row)">
            关闭
          </el-button>
        </template>
      </el-table-column>
    </el-table>
  </el-card>

  <el-dialog v-model="dialog" title="为商户开通游戏" width="480px">
    <el-form label-width="96px">
      <el-form-item label="游戏">
        <el-select v-model="form.gameId" style="width: 100%">
          <el-option
            v-for="g in availableGames"
            :key="g.id"
            :label="`${g.gameCode} · ${g.name}`"
            :value="g.id"
          />
        </el-select>
      </el-form-item>
      <el-form-item label="RTP 档位">
        <el-select v-model="form.rtpTierCode" style="width: 100%">
          <el-option
            v-for="tier in rtpTiers"
            :key="tier.tierCode"
            :label="`${tier.tierCode} (${formatRtpPpm(tier.targetRtpPpm)})`"
            :value="tier.tierCode"
          />
        </el-select>
      </el-form-item>
      <el-form-item label="排序">
        <el-input-number v-model="form.sortOrder" :min="0" />
      </el-form-item>
      <el-form-item label="最小注覆盖">
        <el-input-number v-model="form.minBetMinor" :min="0" placeholder="留空=默认" />
      </el-form-item>
      <el-form-item label="最大注覆盖">
        <el-input-number v-model="form.maxBetMinor" :min="0" placeholder="留空=默认" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="dialog = false">{{ t("btn.cancel", "取消") }}</el-button>
      <el-button type="primary" @click="onAdd">{{ t("btn.save", "开通") }}</el-button>
    </template>
  </el-dialog>

  <el-dialog v-model="editDialog" title="编辑商户游戏" width="480px">
    <el-form label-width="96px">
      <el-form-item label="游戏">
        <el-input :model-value="editForm.gameName" disabled />
      </el-form-item>
      <el-form-item label="RTP 档位">
        <el-select v-model="editForm.rtpTierCode" style="width: 100%">
          <el-option
            v-for="tier in rtpTiers"
            :key="tier.tierCode"
            :label="`${tier.tierCode} (${formatRtpPpm(tier.targetRtpPpm)})`"
            :value="tier.tierCode"
          />
        </el-select>
      </el-form-item>
      <el-form-item label="排序">
        <el-input-number v-model="editForm.sortOrder" :min="0" />
      </el-form-item>
      <el-form-item label="状态">
        <el-radio-group v-model="editForm.status">
          <el-radio :value="1">开通</el-radio>
          <el-radio :value="0">关闭</el-radio>
        </el-radio-group>
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="editDialog = false">{{ t("btn.cancel", "取消") }}</el-button>
      <el-button type="primary" @click="onSaveEdit">{{ t("btn.save", "保存") }}</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.muted {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
</style>
