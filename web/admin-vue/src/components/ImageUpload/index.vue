<script setup lang="ts">
import { ref } from "vue";
import { ElMessage } from "element-plus";
import { Plus } from "@element-plus/icons-vue";
import { uploadAsset } from "@/utils/file";

const props = withDefaults(
  defineProps<{
    modelValue?: string;
    disabled?: boolean;
  }>(),
  { modelValue: "", disabled: false }
);

const emit = defineEmits<{
  (e: "update:modelValue", value: string): void;
}>();

const uploading = ref(false);

async function onChange(file: { raw?: File }) {
  if (!file.raw || props.disabled) return;
  uploading.value = true;
  try {
    const res = await uploadAsset(file.raw);
    emit("update:modelValue", res.url);
    ElMessage.success("上传成功");
  } catch (e: any) {
    ElMessage.error(e?.message || "上传失败");
  } finally {
    uploading.value = false;
  }
}

function onRemove() {
  emit("update:modelValue", "");
}
</script>

<template>
  <div class="image-upload">
    <el-upload
      class="uploader"
      :show-file-list="false"
      accept="image/png,image/jpeg,image/gif,image/webp,image/svg+xml"
      :disabled="disabled || uploading"
      :auto-upload="false"
      :on-change="onChange"
    >
      <img v-if="modelValue" :src="modelValue" class="preview" alt="" />
      <el-icon v-else class="placeholder"><Plus /></el-icon>
    </el-upload>
    <el-button
      v-if="modelValue && !disabled"
      link
      type="danger"
      class="remove"
      @click="onRemove"
    >
      移除
    </el-button>
  </div>
</template>

<style scoped>
.image-upload {
  display: flex;
  align-items: flex-end;
  gap: 8px;
}

.uploader :deep(.el-upload) {
  border: 1px dashed var(--el-border-color);
  border-radius: 6px;
  cursor: pointer;
  width: 96px;
  height: 96px;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
}

.preview {
  width: 96px;
  height: 96px;
  object-fit: cover;
}

.placeholder {
  font-size: 24px;
  color: var(--el-text-color-secondary);
}
</style>
