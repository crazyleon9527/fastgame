<script setup lang="ts">
import { ref } from "vue";
import { ElMessage } from "element-plus";
import { downloadExcel, importExcel } from "@/utils/file";

const props = defineProps<{
  exportPath: string;
  exportParams?: Record<string, unknown>;
  exportLabel?: string;
  importPath?: string;
  importLabel?: string;
  disabled?: boolean;
}>();

const emit = defineEmits<{
  (e: "imported"): void;
}>();

const importing = ref(false);
const exporting = ref(false);

async function onExport() {
  exporting.value = true;
  try {
    await downloadExcel(props.exportPath, props.exportParams);
    ElMessage.success("导出成功");
  } catch (e: any) {
    ElMessage.error(e?.message || "导出失败");
  } finally {
    exporting.value = false;
  }
}

async function onImport(file: { raw?: File }) {
  if (!props.importPath || !file.raw) return;
  importing.value = true;
  try {
    const res = await importExcel(props.importPath, file.raw);
    ElMessage.success(`已导入 ${res.imported ?? 0} 条`);
    emit("imported");
  } catch (e: any) {
    ElMessage.error(e?.message || "导入失败");
  } finally {
    importing.value = false;
  }
}
</script>

<template>
  <el-space wrap>
    <el-button :loading="exporting" :disabled="disabled" @click="onExport">
      {{ exportLabel || "导出 Excel" }}
    </el-button>
    <el-upload
      v-if="importPath"
      :show-file-list="false"
      accept=".xlsx,.xls"
      :disabled="disabled || importing"
      :auto-upload="false"
      :on-change="onImport"
    >
      <el-button :loading="importing" :disabled="disabled">
        {{ importLabel || "导入 Excel" }}
      </el-button>
    </el-upload>
  </el-space>
</template>
