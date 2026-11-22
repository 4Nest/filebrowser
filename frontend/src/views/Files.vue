<template>
  <div class="fb-files-container">
    <errors v-if="error" :errorCode="error.status" />
    <component v-else-if="currentView" :is="currentView"></component>
    <div v-else class="fb-loading-state">
      <div class="fb-spinner"></div>
      <span>{{ t("files.loading") }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import {
  computed,
  defineAsyncComponent,
  onBeforeUnmount,
  onMounted,
  onUnmounted,
  ref,
  watch,
} from "vue";
import { files as api } from "@/api";
import { storeToRefs } from "pinia";
import { useFileStore } from "@/stores/file";
import { useLayoutStore } from "@/stores/layout";

import Errors from "@/views/Errors.vue";
import { useI18n } from "vue-i18n";
import { useRoute, useRouter } from "vue-router";
import FileListing from "@/views/files/FileListing.vue";
import { StatusError } from "@/api/utils";
import { name } from "../utils/constants";
import { resolveDefaultFilesRoute } from "@/utils/prefs";

const Editor = defineAsyncComponent(() => import("@/views/files/Editor.vue"));
const Preview = defineAsyncComponent(() => import("@/views/files/Preview.vue"));

const layoutStore = useLayoutStore();
const fileStore = useFileStore();

const { reload } = storeToRefs(fileStore);

const route = useRoute();
const router = useRouter();

const { t } = useI18n({});

let fetchDataController = new AbortController();

const error = ref<StatusError | null>(null);

const currentView = computed(() => {
  if (fileStore.req?.type === undefined) {
    return null;
  }

  if (fileStore.req.isDir) {
    return FileListing;
  } else if (fileStore.req.extension.toLowerCase() === ".csv") {
    if (route.query.edit === "true") {
      return Editor;
    }
    return Preview;
  } else if (
    fileStore.req.type === "text" ||
    fileStore.req.type === "textImmutable"
  ) {
    return Editor;
  } else {
    return Preview;
  }
});

onMounted(() => {
  if (
    (route.path === "/files" || route.path === "/files/") &&
    route.query.root !== "1"
  ) {
    const def = resolveDefaultFilesRoute();
    if (def !== "/files/") {
      router.replace({ path: def });
      return;
    }
  }
  fetchData();
  fileStore.isFiles = true;
  window.addEventListener("keydown", keyEvent);
});

onBeforeUnmount(() => {
  window.removeEventListener("keydown", keyEvent);
});

onUnmounted(() => {
  fileStore.isFiles = false;
  if (layoutStore.showShell) {
    layoutStore.toggleShell();
  }
  fileStore.updateRequest(null);
});

watch(route, (to, from) => {
  if (from.path.endsWith("/")) {
    if (to.path.endsWith("/")) {
      window.sessionStorage.setItem("listFrozen", "false");
    } else {
      window.sessionStorage.setItem("listFrozen", "true");
    }
    fetchData();
  } else if (to.path.endsWith("/")) {
    fileStore.$reset();
    fetchData();
  } else {
    fetchData();
  }
});
watch(reload, (newValue) => {
  newValue && fetchData();
});

const fetchData = async () => {
  fileStore.selected = [];
  fileStore.multiple = false;
  layoutStore.closeHovers();

  if (
    window.sessionStorage.getItem("listFrozen") !== "true" &&
    !layoutStore.loading
  ) {
    layoutStore.loading = true;
  }

  let url = route.path;
  if (url === "") url = "/";
  if (url[0] !== "/") url = "/" + url;

  try {
    fetchDataController.abort();
    fetchDataController = new AbortController();
    const res = await api.fetch(url, fetchDataController.signal);

    let title = res.name;
    if (title == "") {
      title = t("files.home");
    }
    document.title = `${title} - ${t("files.files")} - ${name}`;

    fileStore.updateRequest(res);
    error.value = null;
  } catch (e: any) {
    if (e instanceof StatusError && e.is_canceled) {
      return;
    }
    error.value = e;
  } finally {
    layoutStore.loading = false;
    fileStore.reload = false;
  }
};

const keyEvent = (event: KeyboardEvent) => {
  if (event.key === "F1") {
    event.preventDefault();
    layoutStore.showHover("help");
  }
};
</script>
