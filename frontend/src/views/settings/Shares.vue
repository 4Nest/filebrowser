<template>
  <errors v-if="error" :errorCode="error.status" />
  <section v-else-if="!layoutStore.loading" class="fb-settings-section">
    <div class="fb-section-head" v-if="links.length > 0">
      <div>
        <h2>有效分享列表</h2>
      </div>
      <span class="fb-pill fb-mono">共 {{ links.length }} 条</span>
    </div>

    <div class="fb-table-container" v-if="links.length > 0">
      <table class="fb-clean-table">
        <thead>
          <tr>
            <th>{{ t("settings.path") }}</th>
            <th>{{ t("settings.shareDuration") }}</th>
            <th v-if="authStore.user?.perm.admin">创建者</th>
            <th class="fb-th-right">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="link in links" :key="link.hash">
            <td class="fb-mono">
              <a :href="buildLink(link)" target="_blank" class="fb-table-link">
                {{ link.path }}
              </a>
            </td>
            <td class="fb-mono">
              <template v-if="link.expire !== 0">{{
                humanTime(link.expire)
              }}</template>
              <template v-else>{{ t("permanent") }}</template>
            </td>
            <td v-if="authStore.user?.perm.admin">{{ link.username }}</td>
            <td class="fb-td-actions">
              <button
                type="button"
                class="fb-tool-btn"
                @click="copyToClipboard(buildLink(link))"
              >
                <i class="material-icons">content_copy</i>
                <span>复制</span>
              </button>
              <button
                type="button"
                class="fb-tool-btn fb-tool-btn--danger"
                @click="deleteLink($event, link)"
              >
                <i class="material-icons">delete_outline</i>
                <span>撤销</span>
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="fb-empty-state" v-else>
      <i class="material-icons">link_off</i>
      <p>当前没有生效中的公开分享链接</p>
    </div>
  </section>
</template>

<script setup lang="ts">
import { useAuthStore } from "@/stores/auth";
import { useLayoutStore } from "@/stores/layout";
import { share as api, users } from "@/api";
import dayjs from "dayjs";
import Errors from "@/views/Errors.vue";
import { inject, ref, onMounted } from "vue";
import { useI18n } from "vue-i18n";
import { StatusError } from "@/api/utils";
import { copy } from "@/utils/clipboard";

const $showError = inject<IToastError>("$showError")!;
const $showSuccess = inject<IToastSuccess>("$showSuccess")!;
const { t } = useI18n();

const layoutStore = useLayoutStore();
const authStore = useAuthStore();

const error = ref<StatusError | null>(null);
const links = ref<Share[]>([]);

onMounted(async () => {
  layoutStore.loading = true;
  try {
    const newLinks = await api.list();
    if (authStore.user?.perm.admin) {
      const userMap = new Map<number, string>();
      for (const user of await users.getAll())
        userMap.set(user.id, user.username);
      for (const link of newLinks) {
        if (link.userID && userMap.has(link.userID))
          link.username = userMap.get(link.userID);
      }
    }
    links.value = newLinks;
  } catch (err) {
    if (err instanceof Error) error.value = err;
  } finally {
    layoutStore.loading = false;
  }
});

const copyToClipboard = (text: string) => {
  copy({ text }).then(
    () => $showSuccess(t("success.linkCopied")),
    () =>
      copy({ text }, { permission: true }).then(
        () => $showSuccess(t("success.linkCopied")),
        (e) => $showError(e)
      )
  );
};

const deleteLink = async (event: Event, link: any) => {
  event.preventDefault();
  layoutStore.showHover({
    prompt: "share-delete",
    confirm: () => {
      layoutStore.closeHovers();
      try {
        api.remove(link.hash);
        links.value = links.value.filter((item) => item.hash !== link.hash);
        $showSuccess(t("settings.shareDeleted"));
      } catch (err) {
        if (err instanceof Error) $showError(err);
      }
    },
  });
};

const humanTime = (time: number) => dayjs(time * 1000).fromNow();
const buildLink = (share: Share) => api.getShareURL(share);
</script>
