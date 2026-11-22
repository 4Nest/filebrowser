<template>
  <div class="card floating fb-media-modal" id="bdinfo-modal">
    <div class="card-title fb-media-header">
      <div class="fb-media-title-group">
        <h2>BDInfo</h2>
        <span class="fb-media-subtitle" :title="targetName">{{ targetName }}</span>
      </div>
      <div class="fb-media-actions" v-if="task?.status === 'done'">
        <button class="button button--flat" @click="start(true)" title="忽略缓存重新扫描">
          <i class="material-icons">refresh</i>
          <span>重扫</span>
        </button>
        <button class="button button--flat" @click="downloadTxt">
          <i class="material-icons">download</i>
          <span>.txt</span>
        </button>
        <button class="button" @click="copyCurrent">
          <i class="material-icons">{{ copied ? "check" : "content_copy" }}</i>
          <span>{{ copied ? "已复制" : tab === "quick" ? "复制 Quick Summary" : "复制完整报告" }}</span>
        </button>
      </div>
    </div>

    <div class="card-content fb-media-body">
      <div v-if="errorMsg" class="fb-media-error">
        <i class="material-icons">error_outline</i>
        <div>
          <p class="fb-media-error-title">BDInfo 扫描失败</p>
          <pre>{{ errorMsg }}</pre>
        </div>
      </div>

      <div v-else-if="!task || task.status === 'running'" class="fb-bdinfo-progress">
        <div class="fb-bdinfo-progress-top">
          <span class="fb-bdinfo-stage">{{ task?.stage || "正在初始化 BDInfo 扫描器…" }}</span>
          <span class="fb-mono">{{ task?.progress ?? 5 }}% · {{ formatSec(task?.elapsedSec ?? 0) }}</span>
        </div>
        <div class="fb-progress-track">
          <div class="fb-progress-fill" :style="{ width: `${task?.progress ?? 8}%` }"></div>
        </div>
        <p class="fb-media-hint">
          大型蓝光原盘扫描通常需要 15 秒至 2 分钟，结果将在服务端缓存 30 分钟；关闭窗口不会中断后台扫描。
        </p>
      </div>

      <template v-else-if="task.status === 'done'">
        <div class="fb-tabs">
          <button
            class="fb-tab"
            :class="{ active: tab === 'quick' }"
            @click="tab = 'quick'"
          >
            Quick Summary（PT 推荐）
          </button>
          <button
            class="fb-tab"
            :class="{ active: tab === 'full' }"
            @click="tab = 'full'"
          >
            Full Report
          </button>
          <span class="fb-tab-meta fb-mono">耗时 {{ formatSec(task.elapsedSec) }}</span>
        </div>

        <pre class="fb-report-pre">{{ currentText }}</pre>
      </template>
    </div>

    <div class="card-action">
      <button class="button button--flat" @click="layoutStore.closeHovers()">
        关闭
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import { useRoute } from "vue-router";
import { useFileStore } from "@/stores/file";
import { useLayoutStore } from "@/stores/layout";
import { media as mediaApi } from "@/api";
import type { BDInfoTask } from "@/api/media";
import { copy } from "@/utils/clipboard";

const route = useRoute();
const fileStore = useFileStore();
const layoutStore = useLayoutStore();

const task = ref<BDInfoTask | null>(null);
const errorMsg = ref("");
const tab = ref<"quick" | "full">("quick");
const copied = ref(false);
let pollTimer: number | null = null;

const targetItem = computed(() => {
  const propUrl = layoutStore.currentPrompt?.props?.url;
  const propName = layoutStore.currentPrompt?.props?.name;
  if (propUrl) {
    return { url: propUrl as string, name: (propName as string) || propUrl };
  }
  if (fileStore.selectedCount === 1 && fileStore.req) {
    const it = fileStore.req.items[fileStore.selected[0]];
    return { url: it.url, name: it.name };
  }
  return { url: route.path, name: fileStore.req?.name || route.path };
});

const targetName = computed(() => targetItem.value.name);

const currentText = computed(() => {
  if (!task.value) return "";
  return tab.value === "quick"
    ? task.value.quickSummary || task.value.fullReport || ""
    : task.value.fullReport || "";
});

const formatSec = (s: number) => `${s.toFixed(1)}s`;

const stopPoll = () => {
  if (pollTimer !== null) {
    window.clearInterval(pollTimer);
    pollTimer = null;
  }
};

const start = async (force = false) => {
  stopPoll();
  errorMsg.value = "";
  task.value = null;
  try {
    const t = await mediaApi.startBDInfoScan(targetItem.value.url, force);
    task.value = t;
    if (t.status === "error") {
      errorMsg.value = t.error || "未知错误";
      return;
    }
    if (t.status === "running") {
      pollTimer = window.setInterval(async () => {
        if (!task.value) return;
        try {
          const updated = await mediaApi.getBDInfoTask(task.value.id);
          task.value = updated;
          if (updated.status === "done") {
            stopPoll();
          } else if (updated.status === "error") {
            errorMsg.value = updated.error || "BDInfo 扫描失败";
            stopPoll();
          }
        } catch (err: any) {
          errorMsg.value = err?.message || String(err);
          stopPoll();
        }
      }, 1500);
    }
  } catch (e: any) {
    errorMsg.value = e?.message || String(e);
  }
};

const copyCurrent = async () => {
  if (!currentText.value) return;
  await copy({ text: currentText.value });
  copied.value = true;
  setTimeout(() => (copied.value = false), 1800);
};

const downloadTxt = () => {
  if (!task.value?.fullReport) return;
  const blob = new Blob([task.value.fullReport], { type: "text/plain;charset=utf-8" });
  const a = document.createElement("a");
  a.href = URL.createObjectURL(blob);
  a.download = `BDINFO.${task.value.targetPath || "disc"}.txt`;
  a.click();
  URL.revokeObjectURL(a.href);
};

onMounted(() => start(false));
onUnmounted(stopPoll);
</script>
