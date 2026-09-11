<script setup lang="ts">
import { ref } from "vue";
import { ElMessage } from "element-plus";
import { getMerchantAllowedIPs, updateMerchantAllowedIPs } from "@/api/fastgame";
import { canWrite } from "@/utils/role";
import { withRequest } from "@/utils/request";
import { t } from "@/utils/i18n";
import MerchantSelect from "@/components/MerchantSelect/index.vue";

defineOptions({ name: "MerchantWhitelist" });

const merchantId = ref<number>();
const ipsText = ref("");

async function load() {
  if (!merchantId.value) return;
  const data = await withRequest(
    () => getMerchantAllowedIPs(merchantId.value!),
    "加载失败"
  );
  if (!data) return;
  ipsText.value = (data?.allowedIps ?? []).join("\n");
}

async function save() {
  if (!merchantId.value) {
    ElMessage.warning("请先选择商户");
    return;
  }
  const allowedIps = ipsText.value
    .split("\n")
    .map(s => s.trim())
    .filter(Boolean);
  const ok = await withRequest(
    () => updateMerchantAllowedIPs(merchantId.value!, allowedIps),
    "保存失败"
  );
  if (!ok) return;
  ElMessage.success("已保存");
}
</script>

<template>
  <el-card shadow="never" :header="t('nav.whitelist', '商户 IP 白名单')">
    <el-form label-width="88px" style="max-width: 560px">
      <el-form-item label="商户">
        <MerchantSelect v-model="merchantId" @change="load" />
      </el-form-item>
      <el-form-item label="IP 列表">
        <el-input
          v-model="ipsText"
          type="textarea"
          :rows="10"
          placeholder="每行一个 IP 或 CIDR"
        />
      </el-form-item>
      <el-form-item>
        <el-button @click="load">加载</el-button>
        <el-button v-if="canWrite()" type="primary" @click="save">
          {{ t("btn.save", "保存") }}
        </el-button>
      </el-form-item>
    </el-form>
  </el-card>
</template>
