<script setup lang="ts">
import { computed, ref } from "vue";
import dayjs from "dayjs";
import { lookupTrace, lookupTraceByRound } from "@/api/fastgame";
import { withRequest } from "@/utils/request";
import { formatMoney } from "@/utils/money";
import { t } from "@/utils/i18n";

defineOptions({ name: "TraceLookup" });

const traceId = ref("");
const roundId = ref("");
const result = ref<any>(null);
const loading = ref(false);

const spans = computed(() => {
  const list = result.value?.spans ?? [];
  return [...list].sort(
    (a: any, b: any) => (a.occurredAt ?? 0) - (b.occurredAt ?? 0)
  );
});

const pendingTx = computed(() => result.value?.pendingTransactions ?? []);

function fmtTime(ts?: number) {
  if (!ts) return "-";
  const ms = ts > 1e12 ? ts : ts * 1000;
  return dayjs(ms).format("YYYY-MM-DD HH:mm:ss.SSS");
}

function statusType(status?: string) {
  if (!status) return "info";
  const s = status.toLowerCase();
  if (s.includes("ok") || s.includes("success")) return "success";
  if (s.includes("fail") || s.includes("error")) return "danger";
  if (s.includes("pending")) return "warning";
  return "info";
}

async function searchTrace() {
  if (!traceId.value.trim()) return;
  loading.value = true;
  result.value = null;
  const data = await withRequest(
    () => lookupTrace(traceId.value.trim()),
    "Trace 查询失败"
  );
  result.value = data;
  loading.value = false;
}

async function searchRound() {
  if (!roundId.value.trim()) return;
  loading.value = true;
  result.value = null;
  const data = await withRequest(
    () => lookupTraceByRound(roundId.value.trim()),
    "Round 查询失败"
  );
  result.value = data;
  loading.value = false;
}
</script>

<template>
  <el-card shadow="never" :header="t('nav.trace', 'Trace 追踪')">
    <el-space direction="vertical" fill style="width: 100%" :size="16">
      <el-input v-model="traceId" placeholder="Trace ID" clearable>
        <template #append>
          <el-button :loading="loading" type="primary" @click="searchTrace">
            查询
          </el-button>
        </template>
      </el-input>
      <el-input v-model="roundId" placeholder="Round ID" clearable>
        <template #append>
          <el-button :loading="loading" @click="searchRound">
            按 Round 查
          </el-button>
        </template>
      </el-input>

      <template v-if="result">
        <el-descriptions :column="2" border size="small">
          <el-descriptions-item label="Trace ID">
            {{ result.traceId || "-" }}
          </el-descriptions-item>
          <el-descriptions-item label="Round ID">
            {{ result.roundId || "-" }}
          </el-descriptions-item>
          <el-descriptions-item label="Span 数">
            {{ spans.length }}
          </el-descriptions-item>
          <el-descriptions-item label="Pending Tx">
            {{ pendingTx.length }}
          </el-descriptions-item>
        </el-descriptions>

        <el-card shadow="never" header="调用链 Span">
          <el-empty v-if="!spans.length" description="无 Span 记录" />
          <el-timeline v-else>
            <el-timeline-item
              v-for="(span, idx) in spans"
              :key="span.spanId || idx"
              :timestamp="fmtTime(span.occurredAt)"
              placement="top"
            >
              <div class="span-card">
                <div class="span-head">
                  <el-tag size="small" type="primary">{{
                    span.service
                  }}</el-tag>
                  <strong>{{ span.operation }}</strong>
                  <el-tag size="small" :type="statusType(span.status)">
                    {{ span.status }}
                  </el-tag>
                  <span v-if="span.durationMs" class="muted">
                    {{ span.durationMs }} ms
                  </span>
                </div>
                <div v-if="span.roundId" class="muted">
                  Round: {{ span.roundId }}
                </div>
                <div v-if="span.detail" class="detail">{{ span.detail }}</div>
              </div>
            </el-timeline-item>
          </el-timeline>
        </el-card>

        <el-card shadow="never" header="Pending 事务">
          <el-empty v-if="!pendingTx.length" description="无待补偿事务" />
          <el-table v-else :data="pendingTx" stripe size="small">
            <el-table-column prop="roundId" label="Round" min-width="140" />
            <el-table-column prop="phase" label="阶段" width="100" />
            <el-table-column prop="status" label="状态" width="100">
              <template #default="{ row }">
                <el-tag size="small" :type="statusType(row.status)">
                  {{ row.status }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="投注" width="120">
              <template #default="{ row }">
                {{ formatMoney(row.betAmount / 10000) }}
              </template>
            </el-table-column>
            <el-table-column label="派彩" width="120">
              <template #default="{ row }">
                {{ formatMoney(row.winAmount / 10000) }}
              </template>
            </el-table-column>
            <el-table-column
              prop="expectedAction"
              label="期望动作"
              width="110"
            />
            <el-table-column prop="retryCount" label="重试" width="70" />
            <el-table-column
              prop="lastError"
              label="最后错误"
              show-overflow-tooltip
            />
            <el-table-column label="创建时间" width="170">
              <template #default="{ row }">
                {{ fmtTime(row.createdAt) }}
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </template>
    </el-space>
  </el-card>
</template>

<style scoped>
.span-card {
  padding: 4px 0;
}
.span-head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
}
.muted {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
.detail {
  margin-top: 4px;
  font-size: 13px;
  word-break: break-all;
}
</style>
