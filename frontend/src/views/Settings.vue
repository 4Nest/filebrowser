<template>
  <div class="fb-settings-page dashboard">
    <div class="fb-page-header">
      <div class="fb-page-header-left">
        <button
          type="button"
          class="fb-icon-btn fb-mobile-menu"
          @click="layoutStore.showHover('sidebar')"
        >
          <i class="material-icons">menu</i>
        </button>
        <div>
          <h1>{{ pageMeta.title }}</h1>
          <p class="fb-page-sub">{{ pageMeta.sub }}</p>
        </div>
      </div>
    </div>

    <div v-if="loading" class="fb-loading-state">
      <div class="fb-spinner"></div>
      <span>{{ t("files.loading") }}</span>
    </div>

    <router-view></router-view>
  </div>
</template>

<script setup lang="ts">
import { useLayoutStore } from "@/stores/layout";
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import { useRoute } from "vue-router";

const { t } = useI18n();
const route = useRoute();
const layoutStore = useLayoutStore();

const loading = computed(() => layoutStore.loading);

const pageMeta = computed(() => {
  const p = route.path;
  if (p === "/settings/profile") {
    return {
      title: t("settings.profileSettings"),
      sub: "自定义外观浏览习惯、媒体与高速下载偏好以及登录密码",
    };
  }
  if (p === "/settings/shares") {
    return {
      title: t("settings.shareManagement"),
      sub: "管理所有已创建的公开分享链接，随时复制或撤销",
    };
  }
  if (p === "/settings/global") {
    return {
      title: t("settings.globalSettings"),
      sub: "实例名称、注册与安全策略、分片传输及高级访问规则",
    };
  }
  if (p.startsWith("/settings/users")) {
    return {
      title: t("settings.userManagement"),
      sub: "管理成员账号、根目录范围与操作权限",
    };
  }
  return {
    title: t("sidebar.settings"),
    sub: "",
  };
});
</script>
