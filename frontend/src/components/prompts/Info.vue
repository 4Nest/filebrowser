<template>
  <div class="card floating fb-info-modal" id="info-modal">
    <div class="card-title fb-media-header">
      <div class="fb-media-title-group">
        <h2>{{ t("prompts.fileInfo") }}</h2>
        <span class="fb-media-subtitle" :title="displayTitle">{{
          displayTitle
        }}</span>
      </div>
      <div class="fb-media-actions" v-if="selectedCount <= 1 && targetPath">
        <button
          type="button"
          class="button button--flat"
          @click="copyPath"
        >
          <i class="material-icons">{{
            copiedKey === "path" ? "check" : "content_copy"
          }}</i>
          <span>{{ copiedKey === "path" ? "已复制路径" : "复制路径" }}</span>
        </button>
      </div>
    </div>

    <div class="card-content fb-info-body">
      <!-- 多选汇总模式 -->
      <template v-if="selectedCount > 1">
        <div class="fb-info-grid">
          <div class="fb-info-cell">
            <span class="fb-info-label">已选项目</span>
            <span class="fb-info-val fb-mono">{{ selectedCount }} 项</span>
          </div>
          <div class="fb-info-cell">
            <span class="fb-info-label">目录 / 文件分布</span>
            <span class="fb-info-val fb-mono">
              {{ multiStats.dirs }} 个目录 · {{ multiStats.files }} 个文件
            </span>
          </div>
          <div class="fb-info-cell fb-info-cell--full">
            <span class="fb-info-label">已选文件大小（不含未展开子目录）</span>
            <span class="fb-info-val fb-mono">
              {{ humanSize }} ({{ exactBytes.toLocaleString() }} bytes)
            </span>
          </div>
        </div>
      </template>

      <!-- 单项 / 当前目录详情模式 -->
      <template v-else-if="activeItem">
        <div class="fb-info-grid">
          <div class="fb-info-cell fb-info-cell--full">
            <span class="fb-info-label">完整路径</span>
            <code class="fb-info-path">{{ targetPath }}</code>
          </div>

          <div class="fb-info-cell">
            <span class="fb-info-label">类型</span>
            <span class="fb-info-val">
              <span class="fb-pill fb-pill--accent">{{ typeLabel }}</span>
              <span v-if="activeItem.isSymlink" class="fb-pill">符号链接</span>
            </span>
          </div>

          <div class="fb-info-cell">
            <span class="fb-info-label">权限位 (Mode)</span>
            <span class="fb-info-val fb-mono">{{ modeText }}</span>
          </div>

          <div class="fb-info-cell">
            <span class="fb-info-label">
              {{ isDir ? "目录总体积" : "文件大小" }}
            </span>
            <div class="fb-info-val fb-mono">
              <template v-if="!isDir">
                <span>{{ humanSize }}</span>
                <span class="fb-info-subval"
                  >{{ exactBytes.toLocaleString() }} bytes</span
                >
              </template>
              <template v-else-if="dirCalc">
                <span>{{ filesize(dirCalc.totalBytes) }}</span>
                <span class="fb-info-subval">
                  {{ dirCalc.files }} 个文件 · {{ dirCalc.dirs }} 个子目录
                </span>
              </template>
              <template v-else>
                <button
                  type="button"
                  class="fb-tool-btn"
                  :disabled="calculatingDir"
                  @click="calcDirSize"
                >
                  <i class="material-icons">{{
                    calculatingDir ? "hourglass_top" : "calculate"
                  }}</i>
                  <span>{{
                    calculatingDir ? "正在递归统计…" : "计算目录总大小"
                  }}</span>
                </button>
              </template>
            </div>
          </div>

          <div class="fb-info-cell">
            <span class="fb-info-label">最后修改时间</span>
            <div class="fb-info-val fb-mono">
              <span>{{ modTimeExact }}</span>
              <span class="fb-info-subval">{{ humanTime }}</span>
            </div>
          </div>

          <div class="fb-info-cell" v-if="resolution">
            <span class="fb-info-label">图片分辨率</span>
            <span class="fb-info-val fb-mono">
              {{ resolution.width }} × {{ resolution.height }} px
            </span>
          </div>

          <div class="fb-info-cell" v-if="isDir && selectedCount === 0 && req">
            <span class="fb-info-label">当前层级统计</span>
            <span class="fb-info-val fb-mono">
              {{ req.numDirs }} 个文件夹 · {{ req.numFiles }} 个文件
            </span>
          </div>
        </div>

        <!-- 文件哈希校验 -->
        <div class="fb-hash-section" v-if="!isDir">
          <div class="fb-hash-title">完整性校验 (Checksum)</div>
          <div class="fb-hash-list">
            <div
              v-for="algo in hashAlgos"
              :key="algo"
              class="fb-hash-row"
            >
              <span class="fb-hash-algo fb-mono">{{ algo.toUpperCase() }}</span>
              <code class="fb-hash-val">
                {{
                  hashes[algo]
                    ? hashes[algo]
                    : hashLoading[algo]
                      ? "正在计算…"
                      : "未计算"
                }}
              </code>
              <button
                v-if="hashes[algo]"
                type="button"
                class="button button--flat fb-copy-btn"
                @click="copyHash(algo)"
              >
                {{ copiedKey === algo ? "已复制" : "复制" }}
              </button>
              <button
                v-else
                type="button"
                class="button button--flat fb-copy-btn"
                :disabled="hashLoading[algo]"
                @click="computeHash(algo)"
              >
                {{ hashLoading[algo] ? "计算中" : "计算" }}
              </button>
            </div>
          </div>
        </div>
      </template>
    </div>

    <div class="card-action fb-info-footer">
      <div class="fb-info-shortcuts">
        <button
          v-if="canMediaInfo"
          type="button"
          class="fb-tool-btn fb-tool-btn--accent"
          @click="openPrompt('mediainfo')"
        >
          <i class="material-icons">analytics</i>
          <span>MediaInfo</span>
        </button>
        <button
          v-if="canScreenshot"
          type="button"
          class="fb-tool-btn fb-tool-btn--accent"
          @click="openPrompt('screenshot')"
        >
          <i class="material-icons">photo_camera</i>
          <span>截图</span>
        </button>
        <button
          v-if="canBDInfo"
          type="button"
          class="fb-tool-btn fb-tool-btn--accent"
          @click="openPrompt('bdinfo')"
        >
          <i class="material-icons">album</i>
          <span>BDInfo</span>
        </button>
      </div>
      <button
        id="focus-prompt"
        type="button"
        class="button button--flat"
        @click="layoutStore.closeHovers()"
      >
        关闭
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, inject, reactive, ref } from "vue";
import { useRoute } from "vue-router";
import { useI18n } from "vue-i18n";
import { storeToRefs } from "pinia";
import dayjs from "dayjs";
import { useFileStore } from "@/stores/file";
import { useLayoutStore } from "@/stores/layout";
import { filesize } from "@/utils";
import { files as api } from "@/api";
import { copy } from "@/utils/clipboard";

const { t } = useI18n();
const route = useRoute();
const fileStore = useFileStore();
const layoutStore = useLayoutStore();
const $showError = inject<IToastError>("$showError")!;

const { req, selected, selectedCount, isListing } = storeToRefs(fileStore);

const hashAlgos = ["md5", "sha1", "sha256", "sha512"] as const;
const hashes = reactive<Record<string, string>>({});
const hashLoading = reactive<Record<string, boolean>>({});
const copiedKey = ref("");
const calculatingDir = ref(false);
const dirCalc = ref<{ totalBytes: number; files: number; dirs: number } | null>(
  null
);

const activeItem = computed<any>(() => {
  if (!req.value) return null;
  if (selectedCount.value === 1) {
    return req.value.items[selected.value[0]] ?? null;
  }
  return req.value;
});

const isDir = computed(() => Boolean(activeItem.value?.isDir));

const displayTitle = computed(() => {
  if (selectedCount.value > 1) return `已选择 ${selectedCount.value} 个项目`;
  return activeItem.value?.name || "/";
});

const targetUrl = computed(() => {
  if (selectedCount.value === 1 && activeItem.value?.url) {
    return activeItem.value.url as string;
  }
  return route.path;
});

const targetPath = computed(() => {
  if (!activeItem.value) return "";
  if (activeItem.value.path) return activeItem.value.path as string;
  return decodeURIComponent(targetUrl.value.replace(/^\/files/, "") || "/");
});

const exactBytes = computed(() => {
  if (!req.value) return 0;
  if (selectedCount.value === 0 || !isListing.value) {
    return req.value.size || 0;
  }
  return selected.value.reduce(
    (acc, idx) => acc + (req.value!.items[idx]?.size || 0),
    0
  );
});

const humanSize = computed(() => filesize(exactBytes.value));

const multiStats = computed(() => {
  let dirs = 0;
  let files = 0;
  if (req.value) {
    for (const idx of selected.value) {
      if (req.value.items[idx]?.isDir) dirs++;
      else files++;
    }
  }
  return { dirs, files };
});

const humanTime = computed(() => {
  const m = activeItem.value?.modified;
  return m ? dayjs(m).fromNow() : "—";
});

const modTimeExact = computed(() => {
  const m = activeItem.value?.modified;
  return m ? dayjs(m).format("YYYY-MM-DD HH:mm:ss") : "—";
});

const resolution = computed(() => {
  if (activeItem.value?.type === "image" && activeItem.value.resolution) {
    return activeItem.value.resolution;
  }
  return null;
});

const fileExt = computed(() => {
  const n = activeItem.value?.name || "";
  const idx = n.lastIndexOf(".");
  return idx >= 0 ? n.slice(idx + 1).toLowerCase() : "";
});

const typeLabel = computed(() => {
  if (!activeItem.value) return "—";
  if (activeItem.value.isDir) {
    return activeItem.value.isBluray ||
      activeItem.value.name?.toUpperCase() === "BDMV"
      ? "BDMV 蓝光原盘"
      : "目录";
  }
  const ext = fileExt.value ? `.${fileExt.value}` : "";
  const typ = activeItem.value.type || "file";
  return ext ? `${typ} (${ext})` : typ;
});

const modeText = computed(() => {
  const mode = activeItem.value?.mode;
  if (typeof mode !== "number") return "—";
  const perm = mode & 0o777;
  const oct = "0" + perm.toString(8).padStart(3, "0");
  const rwx = [6, 3, 0]
    .map((shift) => {
      const v = (perm >> shift) & 7;
      return `${v & 4 ? "r" : "-"}${v & 2 ? "w" : "-"}${v & 1 ? "x" : "-"}`;
    })
    .join("");
  return `${oct} (${isDir.value ? "d" : "-"}${rwx})`;
});

const VIDEO_EXTS = new Set([
  "mkv",
  "mp4",
  "m2ts",
  "ts",
  "mov",
  "avi",
  "webm",
  "m4v",
]);
const AUDIO_EXTS = new Set([
  "flac",
  "wav",
  "aac",
  "mp3",
  "mka",
  "dts",
  "ac3",
  "eac3",
  "thd",
]);

const canMediaInfo = computed(
  () =>
    selectedCount.value <= 1 &&
    !isDir.value &&
    (VIDEO_EXTS.has(fileExt.value) || AUDIO_EXTS.has(fileExt.value))
);

const canBDInfo = computed(() => {
  if (selectedCount.value > 1 || !activeItem.value) return false;
  return (
    Boolean(activeItem.value.isBluray) ||
    fileExt.value === "iso" ||
    (isDir.value && activeItem.value.name?.toUpperCase() === "BDMV")
  );
});

const canScreenshot = computed(
  () =>
    selectedCount.value <= 1 &&
    ((!isDir.value && VIDEO_EXTS.has(fileExt.value)) ||
      (isDir.value && canBDInfo.value))
);

const calcDirSize = async () => {
  calculatingDir.value = true;
  try {
    const entries = await api.fetchAll(targetUrl.value);
    let totalBytes = 0;
    let files = 0;
    let dirs = 0;
    for (const e of entries) {
      if (e.isDir) dirs++;
      else {
        files++;
        totalBytes += e.size || 0;
      }
    }
    dirCalc.value = { totalBytes, files, dirs };
  } catch (e: any) {
    $showError(e);
  } finally {
    calculatingDir.value = false;
  }
};

const computeHash = async (algo: ChecksumAlg) => {
  hashLoading[algo] = true;
  try {
    hashes[algo] = await api.checksum(targetUrl.value, algo);
  } catch (e: any) {
    $showError(e);
  } finally {
    hashLoading[algo] = false;
  }
};

const copyText = async (text: string, key: string) => {
  await copy({ text });
  copiedKey.value = key;
  setTimeout(() => {
    if (copiedKey.value === key) copiedKey.value = "";
  }, 1800);
};

const copyPath = () => copyText(targetPath.value, "path");
const copyHash = (algo: string) => copyText(hashes[algo], algo);

const openPrompt = (name: string) => {
  layoutStore.showHover({
    prompt: name,
    props: { url: targetUrl.value, name: displayTitle.value },
  });
};
</script>
