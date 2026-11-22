<template>
  <errors v-if="error" :errorCode="error.status" />
  <section v-else-if="!layoutStore.loading" class="fb-settings-section">
    <div class="fb-section-head">
      <div>
        <h2>成员列表</h2>
      </div>
      <router-link to="/settings/users/new" class="button">
        <i class="material-icons">person_add</i>
        <span>新建用户</span>
      </router-link>
    </div>

    <div class="fb-table-container">
      <table class="fb-clean-table">
        <thead>
          <tr>
            <th>{{ t("settings.username") }}</th>
            <th>角色</th>
            <th>{{ t("settings.scope") }}</th>
            <th class="fb-th-right">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="user in users" :key="user.id">
            <td>
              <div class="fb-member-cell">
                <span class="fb-member-avatar">{{
                  user.username.slice(0, 1).toUpperCase()
                }}</span>
                <strong>{{ user.username }}</strong>
              </div>
            </td>
            <td>
              <span
                class="fb-pill"
                :class="{ 'fb-pill--accent': user.perm.admin }"
              >
                {{ user.perm.admin ? "管理员" : "普通用户" }}
              </span>
            </td>
            <td>
              <code class="fb-scope-code">{{ user.scope || "/" }}</code>
            </td>
            <td class="fb-td-actions">
              <router-link :to="'/settings/users/' + user.id" class="fb-tool-btn">
                <i class="material-icons">edit</i>
                <span>编辑</span>
              </router-link>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>

<script setup lang="ts">
import { useLayoutStore } from "@/stores/layout";
import { users as api } from "@/api";
import Errors from "@/views/Errors.vue";
import { onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { StatusError } from "@/api/utils";

const error = ref<StatusError | null>(null);
const users = ref<IUser[]>([]);

const layoutStore = useLayoutStore();
const { t } = useI18n();

onMounted(async () => {
  layoutStore.loading = true;
  try {
    users.value = await api.getAll();
  } catch (err) {
    if (err instanceof Error) error.value = err;
  } finally {
    layoutStore.loading = false;
  }
});
</script>
