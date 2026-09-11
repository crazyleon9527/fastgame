<script setup lang="ts">
import { onMounted, ref } from "vue";
import { ElMessage } from "element-plus";
import {
  listAdminUsers,
  createAdminUser,
  updateAdminUser,
  listRoles
} from "@/api/fastgame";
import { confirmAction, withRequest } from "@/utils/request";
import { t } from "@/utils/i18n";

defineOptions({ name: "AdminUsers" });

const loading = ref(false);
const table = ref<any[]>([]);
const roles = ref<any[]>([]);
const createDialog = ref(false);
const editDialog = ref(false);
const createForm = ref({
  username: "",
  password: "",
  roleId: 1,
  status: 1
});
const editForm = ref({
  id: 0,
  username: "",
  roleId: 1,
  status: 1,
  password: ""
});

async function load() {
  loading.value = true;
  try {
    const [users, roleData] = await Promise.all([
      withRequest(() => listAdminUsers(1, 50), "加载用户失败"),
      withRequest(() => listRoles(), "加载角色失败")
    ]);
    table.value = users?.list ?? [];
    roles.value = roleData?.list ?? [];
  } finally {
    loading.value = false;
  }
}

async function onCreate() {
  const ok = await withRequest(
    () => createAdminUser(createForm.value),
    "创建失败"
  );
  if (!ok) return;
  ElMessage.success("用户已创建");
  createDialog.value = false;
  createForm.value = { username: "", password: "", roleId: 1, status: 1 };
  load();
}

function openEdit(row: any) {
  editForm.value = {
    id: row.id,
    username: row.username,
    roleId: row.roleId,
    status: row.status,
    password: ""
  };
  editDialog.value = true;
}

async function onSaveEdit() {
  const payload: any = {
    roleId: editForm.value.roleId,
    status: editForm.value.status
  };
  if (editForm.value.password) {
    payload.password = editForm.value.password;
  }
  const ok = await withRequest(
    () => updateAdminUser(editForm.value.id, payload),
    "更新失败"
  );
  if (!ok) return;
  ElMessage.success("已更新");
  editDialog.value = false;
  load();
}

async function toggleStatus(row: any) {
  const next = row.status === 1 ? 2 : 1;
  const action = next === 2 ? "禁用" : "启用";
  if (!(await confirmAction(`确认${action}用户 ${row.username}？`, "提示"))) {
    return;
  }
  const ok = await withRequest(
    () => updateAdminUser(row.id, { status: next }),
    "操作失败"
  );
  if (!ok) return;
  ElMessage.success(`已${action}`);
  load();
}

onMounted(load);
</script>

<template>
  <el-card shadow="never">
    <template #header>
      <div class="flex justify-between items-center">
        <span>{{ t("nav.users", "管理员") }}</span>
        <el-button type="primary" @click="createDialog = true">
          新建用户
        </el-button>
      </div>
    </template>
    <el-table v-loading="loading" :data="table" stripe>
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="username" label="用户名" />
      <el-table-column prop="roleName" label="角色" width="120" />
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'info'">
            {{ row.status === 1 ? "启用" : "禁用" }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="180">
        <template #default="{ row }">
          <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
          <el-button link type="warning" @click="toggleStatus(row)">
            {{ row.status === 1 ? "禁用" : "启用" }}
          </el-button>
        </template>
      </el-table-column>
    </el-table>
  </el-card>

  <el-dialog v-model="createDialog" title="新建管理员" width="420px">
    <el-form label-width="80px">
      <el-form-item :label="t('label.username', '用户名')">
        <el-input v-model="createForm.username" />
      </el-form-item>
      <el-form-item :label="t('label.password', '密码')">
        <el-input v-model="createForm.password" type="password" show-password />
      </el-form-item>
      <el-form-item label="角色">
        <el-select v-model="createForm.roleId">
          <el-option
            v-for="r in roles"
            :key="r.id"
            :label="r.name"
            :value="r.id"
          />
        </el-select>
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="createDialog = false">
        {{ t("btn.cancel", "取消") }}
      </el-button>
      <el-button type="primary" @click="onCreate">
        {{ t("btn.save", "保存") }}
      </el-button>
    </template>
  </el-dialog>

  <el-dialog v-model="editDialog" title="编辑管理员" width="420px">
    <el-form label-width="80px">
      <el-form-item label="用户名">
        <el-input v-model="editForm.username" disabled />
      </el-form-item>
      <el-form-item label="角色">
        <el-select v-model="editForm.roleId">
          <el-option
            v-for="r in roles"
            :key="r.id"
            :label="r.name"
            :value="r.id"
          />
        </el-select>
      </el-form-item>
      <el-form-item label="状态">
        <el-radio-group v-model="editForm.status">
          <el-radio :value="1">启用</el-radio>
          <el-radio :value="2">禁用</el-radio>
        </el-radio-group>
      </el-form-item>
      <el-form-item label="新密码">
        <el-input
          v-model="editForm.password"
          type="password"
          show-password
          placeholder="留空则不修改"
        />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="editDialog = false">
        {{ t("btn.cancel", "取消") }}
      </el-button>
      <el-button type="primary" @click="onSaveEdit">
        {{ t("btn.save", "保存") }}
      </el-button>
    </template>
  </el-dialog>
</template>
