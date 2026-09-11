<script setup lang="ts">
import { onMounted, ref } from "vue";
import { listMerchants } from "@/api/fastgame";
import { withRequest } from "@/utils/request";

defineOptions({ name: "MerchantSelect" });

const props = withDefaults(
  defineProps<{
    modelValue?: number;
    placeholder?: string;
    clearable?: boolean;
    width?: string;
  }>(),
  {
    clearable: true,
    width: "240px"
  }
);

const emit = defineEmits<{
  "update:modelValue": [value: number | undefined];
  change: [value: number | undefined];
}>();

const merchants = ref<any[]>([]);
const loading = ref(false);

onMounted(async () => {
  loading.value = true;
  try {
    const data = await withRequest(() => listMerchants(1, 200), "加载商户失败");
    merchants.value = data?.list ?? [];
    if (!props.modelValue && merchants.value.length === 1) {
      emit("update:modelValue", merchants.value[0].id);
      emit("change", merchants.value[0].id);
    }
  } finally {
    loading.value = false;
  }
});

function onChange(val: number | undefined) {
  emit("update:modelValue", val);
  emit("change", val);
}
</script>

<template>
  <el-select
    :model-value="modelValue"
    :placeholder="placeholder ?? '选择商户'"
    :clearable="clearable"
    filterable
    :loading="loading"
    :style="{ width }"
    @update:model-value="onChange"
  >
    <el-option
      v-for="m in merchants"
      :key="m.id"
      :label="`${m.merchantCode} · ${m.name}`"
      :value="m.id"
    />
  </el-select>
</template>
