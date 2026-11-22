<template>
  <div class="card floating fb-media-modal fb-shot-modal" id="screenshot-modal">
    <div class="card-title fb-media-header">
      <div class="fb-media-title-group">
        <h2>视频截图与图床上传</h2>
        <span class="fb-media-subtitle" :title="targetName">{{ targetName }}</span>
      </div>
      <div class="fb-media-actions">
        <button
          class="button"
          type="button"
          :disabled="capturing"
          @click="runCapture"
        >
          <i class="material-icons">{{ capturing ? "hourglass_top" : "photo_camera" }}</i>
          <span>{{ capturing ? "截图中…" : captureRes ? "重新截图" : "开始截图" }}</span>
        </button>
      </div>
    </div>

    <div class="card-content fb-media-body">
      <!-- 参数配置条 -->
      <div class="fb-shot-params">
        <div class="fb-param-item">
          <label>张数</label>
          <select v-model.number="params.count">
            <option v-for="n in [2, 3, 4, 6, 8, 10, 12]" :key="n" :value="n">
              {{ n }} 张
            </option>
          </select>
        </div>

        <div class="fb-param-item">
          <label>格式</label>
          <select v-model="params.format">
            <option value="png">PNG (无损)</option>
            <option value="jpg">JPG</option>
            <option value="webp">WebP</option>
          </select>
        </div>

        <div class="fb-param-item" v-if="params.format !== 'png'">
          <label>质量</label>
          <select v-model.number="params.quality">
            <option :value="95">95%</option>
            <option :value="92">92%</option>
            <option :value="85">85%</option>
            <option :value="80">80%</option>
          </select>
        </div>

        <div class="fb-param-item">
          <label>HDR 色调映射</label>
          <select v-model="params.hdrToneMap">
            <option value="auto">自动检测</option>
            <option value="on">始终开启</option>
            <option value="off">关闭</option>
          </select>
        </div>

        <div class="fb-param-item">
          <label>输出宽度</label>
          <select v-model.number="params.width">
            <option :value="0">原始分辨率</option>
            <option :value="1920">1920 px</option>
            <option :value="1280">1280 px</option>
          </select>
        </div>

        <div class="fb-param-item fb-param-item--range">
          <label>采样区间 (%)</label>
          <div class="fb-range-inputs">
            <input type="number" v-model.number="params.startPct" min="0" max="90" />
            <span>–</span>
            <input type="number" v-model.number="params.endPct" min="10" max="100" />
          </div>
        </div>

        <div class="fb-param-item fb-param-item--wide">
          <label>自定义时间点（可选，逗号分隔，留空自动均匀采样）</label>
          <input
            type="text"
            v-model="customTimestamps"
            placeholder="例如：00:05:12, 00:24:30, 01:02:15"
          />
        </div>
      </div>

      <div v-if="errorMsg" class="fb-media-error">
        <i class="material-icons">error_outline</i>
        <div>
          <p class="fb-media-error-title">操作失败</p>
          <pre>{{ errorMsg }}</pre>
        </div>
      </div>

      <div v-if="capturing" class="fb-media-loading">
        <div class="fb-spinner"></div>
        <span>正在调用 FFmpeg 抽取关键帧并渲染预览…</span>
      </div>

      <template v-else-if="captureRes">
        <div class="fb-media-badges">
          <span class="fb-pill fb-mono">{{ captureRes.source }}</span>
          <span class="fb-pill fb-mono">共 {{ captureRes.items.length }} 张</span>
          <span v-if="captureRes.isHdr" class="fb-pill fb-pill--accent">
            {{ captureRes.toneMapApplied ? "HDR → SDR 已映射" : "HDR 原色" }}
          </span>
          <span class="fb-pill fb-mono">已选中 {{ selectedIds.length }} 张</span>
        </div>

        <div class="fb-shot-grid">
          <div
            v-for="item in captureRes.items"
            :key="item.id"
            class="fb-shot-card"
            :class="{ selected: selectedIds.includes(item.id) }"
            @click="toggleShot(item.id)"
          >
            <div class="fb-shot-thumb">
              <img :src="item.previewUrl" :alt="item.filename" loading="lazy" />
              <span class="fb-shot-check">
                <i class="material-icons">{{
                  selectedIds.includes(item.id) ? "check_circle" : "radio_button_unchecked"
                }}</i>
              </span>
              <span class="fb-shot-ts fb-mono">{{ item.timestamp }}</span>
            </div>
            <div class="fb-shot-meta">
              <span class="fb-mono">{{ item.width }}×{{ item.height }} · {{ humanSize(item.size) }}</span>
              <a
                :href="`${item.previewUrl}?download=1`"
                class="fb-shot-dl"
                title="下载单张原图"
                @click.stop
              >
                <i class="material-icons">download</i>
              </a>
            </div>
          </div>
        </div>

        <!-- 图床上传区 -->
        <div class="fb-upload-bar">
          <div class="fb-upload-host">
            <label>目标图床</label>
            <select v-model="selectedHostId" @change="onHostChange">
              <option v-for="h in hosts" :key="h.id" :value="h.id">
                {{ h.name }}
              </option>
            </select>
            <router-link
              to="/settings/profile"
              class="fb-host-cfg-link"
              @click="layoutStore.closeHovers()"
            >
              配置图床 API
            </router-link>
          </div>

          <button
            type="button"
            class="button"
            :disabled="uploading || selectedIds.length === 0"
            @click="runUpload"
          >
            <i class="material-icons">{{ uploading ? "hourglass_top" : "cloud_upload" }}</i>
            <span>{{
              uploading ? "上传中…" : `上传选中 (${selectedIds.length} 张)`
            }}</span>
          </button>
        </div>

        <!-- 上传结果 -->
        <div v-if="uploadRes" class="fb-upload-result">
          <div class="fb-tabs">
            <button
              type="button"
              class="fb-tab"
              :class="{ active: outTab === 'bbcode' }"
              @click="outTab = 'bbcode'"
            >
              BBCode (PT 论坛)
            </button>
            <button
              type="button"
              class="fb-tab"
              :class="{ active: outTab === 'markdown' }"
              @click="outTab = 'markdown'"
            >
              Markdown
            </button>
            <button
              type="button"
              class="fb-tab"
              :class="{ active: outTab === 'direct' }"
              @click="outTab = 'direct'"
            >
              直链列表
            </button>
            <button
              type="button"
              class="button fb-copy-btn"
              style="margin-left: auto"
              @click="copyOutput"
            >
              <i class="material-icons">{{ copied ? "check" : "content_copy" }}</i>
              <span>{{ copied ? "已复制" : "一键复制" }}</span>
            </button>
          </div>

          <div v-if="uploadErrors.length > 0" class="fb-upload-warn">
            部分图片上传失败：{{ uploadErrors.join("；") }}
          </div>

          <pre class="fb-report-pre fb-report-pre--sm">{{ currentOutput }}</pre>
        </div>
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
import { computed, reactive, ref } from "vue";
import { useRoute } from "vue-router";
import { useFileStore } from "@/stores/file";
import { useLayoutStore } from "@/stores/layout";
import { media as mediaApi } from "@/api";
import type {
  ScreenshotCaptureParams,
  ScreenshotCaptureResponse,
  ScreenshotUploadResponse,
} from "@/api/media";
import { getLocalPrefs, saveLocalPrefs } from "@/utils/prefs";
import { copy } from "@/utils/clipboard";
import { filesize } from "@/utils";

const route = useRoute();
const fileStore = useFileStore();
const layoutStore = useLayoutStore();

const prefs = getLocalPrefs();
const hosts = ref(prefs.imageHosts);
const selectedHostId = ref(prefs.activeImageHostId);

const params = reactive<ScreenshotCaptureParams>({
  count: prefs.screenshotCount,
  format: prefs.screenshotFormat,
  quality: prefs.screenshotQuality,
  startPct: prefs.screenshotStartPct,
  endPct: prefs.screenshotEndPct,
  hdrToneMap: prefs.screenshotHdrToneMap,
  width: prefs.screenshotWidth,
});
const customTimestamps = ref("");

const capturing = ref(false);
const uploading = ref(false);
const errorMsg = ref("");
const captureRes = ref<ScreenshotCaptureResponse | null>(null);
const selectedIds = ref<string[]>([]);
const uploadRes = ref<ScreenshotUploadResponse | null>(null);
const outTab = ref<"bbcode" | "markdown" | "direct">("bbcode");
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
  return { url: route.path, name: fileStore.req?.name || route.path };
});

const targetName = computed(() => targetItem.value.name);
const humanSize = (bytes: number) => filesize(bytes);

const toggleShot = (id: string) => {
  const idx = selectedIds.value.indexOf(id);
  if (idx >= 0) selectedIds.value.splice(idx, 1);
  else selectedIds.value.push(id);
};

const onHostChange = () => {
  saveLocalPrefs({ activeImageHostId: selectedHostId.value });
};

const runCapture = async () => {
  errorMsg.value = "";
  uploadRes.value = null;
  capturing.value = true;
  try {
    const tsList = customTimestamps.value
      .split(/[,，\s]+/)
      .map((s) => s.trim())
      .filter(Boolean);
    const res = await mediaApi.captureScreenshots(targetItem.value.url, {
      ...params,
      timestamps: tsList.length > 0 ? tsList : undefined,
    });
    captureRes.value = res;
    selectedIds.value = res.items.map((i) => i.id);
  } catch (e: any) {
    errorMsg.value = e?.message || String(e);
  } finally {
    capturing.value = false;
  }
};

const runUpload = async () => {
  const host = hosts.value.find((h) => h.id === selectedHostId.value);
  if (!host) return;
  if (
    ["picx", "chevereto", "lsky"].includes(host.provider) &&
    !host.apiKey.trim()
  ) {
    errorMsg.value = `请先在「个人设置 → 截图与图床配置」中填写 ${host.name} 的 API Key / Token`;
    return;
  }
  errorMsg.value = "";
  uploading.value = true;
  try {
    uploadRes.value = await mediaApi.uploadScreenshots(selectedIds.value, host);
  } catch (e: any) {
    errorMsg.value = e?.message || String(e);
  } finally {
    uploading.value = false;
  }
};

const uploadErrors = computed(() => {
  if (!uploadRes.value) return [];
  return uploadRes.value.results
    .filter((r) => r.error)
    .map((r) => `${r.filename}: ${r.error}`);
});

const currentOutput = computed(() => {
  if (!uploadRes.value) return "";
  if (outTab.value === "bbcode") return uploadRes.value.bbcode;
  if (outTab.value === "markdown") return uploadRes.value.markdown;
  return uploadRes.value.direct;
});

const copyOutput = async () => {
  if (!currentOutput.value) return;
  await copy({ text: currentOutput.value });
  copied.value = true;
  setTimeout(() => (copied.value = false), 1800);
};
</script>
