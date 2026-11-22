<template>
  <div class="card floating fb-media-modal" id="mediainfo-modal">
    <div class="card-title fb-media-header">
      <div class="fb-media-title-group">
        <h2>MediaInfo</h2>
        <span class="fb-media-subtitle" :title="targetName">{{ targetName }}</span>
      </div>
      <div class="fb-media-actions" v-if="report">
        <button class="button button--flat" @click="downloadNfo">
          <i class="material-icons">download</i>
          <span>.nfo</span>
        </button>
        <button class="button" @click="copyReport">
          <i class="material-icons">{{ copied ? "check" : "content_copy" }}</i>
          <span>{{ copied ? "已复制" : "复制报告" }}</span>
        </button>
      </div>
    </div>

    <div class="card-content fb-media-body">
      <div v-if="loading" class="fb-media-loading">
        <div class="fb-spinner"></div>
        <span>正在解析媒体轨道信息…</span>
      </div>

      <div v-else-if="errorMsg" class="fb-media-error">
        <i class="material-icons">error_outline</i>
        <div>
          <p class="fb-media-error-title">MediaInfo 解析失败</p>
          <pre>{{ errorMsg }}</pre>
        </div>
      </div>

      <template v-else-if="report">
        <div class="fb-media-badges" v-if="hasSummary">
          <span v-if="report.summary.format" class="fb-pill">{{ report.summary.format }}</span>
          <span v-if="report.summary.fileSize" class="fb-pill fb-mono">{{ report.summary.fileSize }}</span>
          <span v-if="report.summary.duration" class="fb-pill fb-mono">{{ report.summary.duration }}</span>
          <span v-if="report.summary.bitRate" class="fb-pill fb-mono">{{ report.summary.bitRate }}</span>
          <span v-for="(v, idx) in report.summary.video" :key="'v' + idx" class="fb-pill fb-pill--accent">
            {{ v }}
          </span>
          <span v-if="report.summary.audio?.length" class="fb-pill">
            Audio × {{ report.summary.audio.length }}
          </span>
          <span v-if="report.summary.subtitles > 0" class="fb-pill">
            Sub × {{ report.summary.subtitles }}
          </span>
        </div>

        <pre class="fb-report-pre">{{ report.text }}</pre>
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
import { computed, onMounted, ref } from "vue";
import { useFileStore } from "@/stores/file";
import { useLayoutStore } from "@/stores/layout";
import { media as mediaApi } from "@/api";
import type { MediaInfoResponse } from "@/api/media";
import { copy } from "@/utils/clipboard";

const fileStore = useFileStore();
const layoutStore = useLayoutStore();

const loading = ref(true);
const errorMsg = ref("");
const report = ref<MediaInfoResponse | null>(null);
const copied = ref(false);

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
  if (fileStore.req && !fileStore.req.isDir) {
    return { url: fileStore.req.url, name: fileStore.req.name };
  }
  return null;
});

const targetName = computed(() => targetItem.value?.name ?? "");

const hasSummary = computed(() => {
  if (!report.value) return false;
  const s = report.value.summary;
  return !!(s.format || s.fileSize || s.duration || s.video?.length || s.audio?.length);
});

const load = async () => {
  if (!targetItem.value) {
    loading.value = false;
    errorMsg.value = "请先选择一个视频或音频文件";
    return;
  }
  loading.value = true;
  errorMsg.value = "";
  try {
    report.value = await mediaApi.getMediaInfo(targetItem.value.url);
  } catch (e: any) {
    errorMsg.value = e?.message || String(e);
  } finally {
    loading.value = false;
  }
};

const copyReport = async () => {
  if (!report.value) return;
  await copy({ text: report.value.text });
  copied.value = true;
  setTimeout(() => (copied.value = false), 1800);
};

const downloadNfo = () => {
  if (!report.value) return;
  const blob = new Blob([report.value.text], { type: "text/plain;charset=utf-8" });
  const a = document.createElement("a");
  a.href = URL.createObjectURL(blob);
  a.download = `${report.value.name}.mediainfo.txt`;
  a.click();
  URL.revokeObjectURL(a.href);
};

onMounted(load);
</script>
