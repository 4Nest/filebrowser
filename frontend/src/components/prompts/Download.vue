<template>
  <div class="card floating fb-fastdl-modal" id="download">
    <div class="card-title fb-media-header">
      <div class="fb-media-title-group">
        <h2>高速下载</h2>
        <span class="fb-media-subtitle" v-if="signed" :title="signed.filename">
          {{ signed.filename }} · {{ signed.fileCount }} 个文件 · {{ humanSize(signed.totalSize) }}
        </span>
      </div>
      <div class="fb-ttl-select">
        <label>有效期</label>
        <select v-model.number="ttlHours" @change="sign">
          <option :value="2">2 小时</option>
          <option :value="6">6 小时</option>
          <option :value="24">24 小时</option>
          <option :value="48">48 小时</option>
        </select>
      </div>
    </div>

    <div class="card-content fb-fastdl-body">
      <div v-if="loading" class="fb-media-loading">
        <div class="fb-spinner"></div>
        <span>正在索引文件并生成多线程签名链接…</span>
      </div>

      <div v-else-if="errorMsg" class="fb-media-error">
        <i class="material-icons">error_outline</i>
        <div>
          <p class="fb-media-error-title">生成高速下载链接失败</p>
          <pre>{{ errorMsg }}</pre>
        </div>
      </div>

      <template v-else-if="signed">
        <div class="fb-fastdl-section">
          <div class="fb-fastdl-section-head">
            <div>
              <strong>
                {{ signed.mode === "file" ? "单文件多线程直链" : "虚拟无压缩 TAR（推荐拉取 BDMV / 多碎文件）" }}
              </strong>
              <p class="fb-fastdl-desc">
                {{
                  signed.mode === "file"
                    ? "支持 HTTP Range 断点续传，丢进 IDM / Aria2 多线程即可跑满海外 VPS 带宽。"
                    : "零服务器磁盘拷贝、零 CPU 压缩开销，支持 HTTP Range 与 IDM / Aria2 16 线程并发拉取，彻底消除碎文件 RTT 瓶颈。"
                }}
              </p>
            </div>
            <button class="button" @click="directDownload">
              <i class="material-icons">bolt</i>
              <span>立即下载</span>
            </button>
          </div>

          <div class="fb-cmd-list">
            <div class="fb-cmd-row" @click="copyText(signed.url, 'url')">
              <span class="fb-cmd-label">IDM / 浏览器直链</span>
              <span class="fb-cmd-preview" :title="signed.url">{{ signed.url }}</span>
              <button class="button button--flat fb-copy-btn" @click.stop="copyText(signed.url, 'url')">
                <i class="material-icons">{{ copiedKey === "url" ? "check" : "content_copy" }}</i>
                <span>{{ copiedKey === "url" ? "已复制" : "复制" }}</span>
              </button>
            </div>

            <div class="fb-cmd-row" @click="copyText(effectiveAria2Cmd, 'aria2')">
              <span class="fb-cmd-label">Aria2 ({{ prefs.aria2Threads }} 线程)</span>
              <span class="fb-cmd-preview" :title="effectiveAria2Cmd">{{ effectiveAria2Cmd }}</span>
              <button class="button button--flat fb-copy-btn" @click.stop="copyText(effectiveAria2Cmd, 'aria2')">
                <i class="material-icons">{{ copiedKey === "aria2" ? "check" : "content_copy" }}</i>
                <span>{{ copiedKey === "aria2" ? "已复制" : "复制" }}</span>
              </button>
            </div>

            <div class="fb-cmd-row" @click="copyText(signed.curlCmd, 'curl')">
              <span class="fb-cmd-label">curl 断点续传</span>
              <span class="fb-cmd-preview" :title="signed.curlCmd">{{ signed.curlCmd }}</span>
              <button class="button button--flat fb-copy-btn" @click.stop="copyText(signed.curlCmd, 'curl')">
                <i class="material-icons">{{ copiedKey === "curl" ? "check" : "content_copy" }}</i>
                <span>{{ copiedKey === "curl" ? "已复制" : "复制" }}</span>
              </button>
            </div>
          </div>
        </div>

        <div class="fb-fastdl-section" v-if="signed.aria2ManifestCmd">
          <div class="fb-fastdl-section-head">
            <div>
              <strong>Aria2 逐文件并发清单（免解包，直接还原目录）</strong>
              <p class="fb-fastdl-desc">
                同时拉取 6 个文件 × 16 线程，直接按原始目录结构落地本地，适合不想解压 .tar 的场景。
              </p>
            </div>
          </div>

          <div class="fb-cmd-list">
            <div class="fb-cmd-row" @click="copyText(effectiveManifestCmd, 'manifest')">
              <span class="fb-cmd-label">一键并发拉取</span>
              <span class="fb-cmd-preview" :title="effectiveManifestCmd">{{ effectiveManifestCmd }}</span>
              <button class="button button--flat fb-copy-btn" @click.stop="copyText(effectiveManifestCmd, 'manifest')">
                <i class="material-icons">{{ copiedKey === "manifest" ? "check" : "content_copy" }}</i>
                <span>{{ copiedKey === "manifest" ? "已复制" : "复制" }}</span>
              </button>
            </div>
          </div>
        </div>

        <details class="fb-legacy-formats" v-if="signed.mode !== 'file'">
          <summary>传统归档格式（zip / tar.gz / tar.zst 等）</summary>
          <div class="fb-legacy-grid">
            <button
              v-for="(ext, format) in formats"
              :key="format"
              class="button button--flat"
              @click="layoutStore.currentPrompt?.confirm(format)"
            >
              {{ ext }}
            </button>
          </div>
        </details>
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
import { useRoute } from "vue-router";
import { useFileStore } from "@/stores/file";
import { useLayoutStore } from "@/stores/layout";
import { media as mediaApi } from "@/api";
import type { FastDLSignResponse } from "@/api/media";
import { copy } from "@/utils/clipboard";
import { filesize } from "@/utils";
import { getLocalPrefs } from "@/utils/prefs";

const route = useRoute();
const fileStore = useFileStore();
const layoutStore = useLayoutStore();

const prefs = getLocalPrefs();
const loading = ref(true);
const errorMsg = ref("");
const signed = ref<FastDLSignResponse | null>(null);
const ttlHours = ref(prefs.fastdlTtlHours);
const copiedKey = ref("");

const effectiveAria2Cmd = computed(() => {
  if (!signed.value) return "";
  const t = prefs.aria2Threads;
  return signed.value.aria2Cmd.replace(/-x \d+ -s \d+/, `-x ${t} -s ${t}`);
});

const effectiveManifestCmd = computed(() => {
  if (!signed.value?.aria2ManifestCmd) return "";
  const t = prefs.aria2Threads;
  return signed.value.aria2ManifestCmd.replace(
    /-x \d+ -s \d+/,
    `-x ${t} -s ${t}`
  );
});

const formats = {
  vtar: "tar (高速可续传)",
  zip: "zip",
  targz: "tar.gz",
  tarzst: "tar.zst",
  tarbz2: "tar.bz2",
  tarxz: "tar.xz",
};

const targetUrls = computed<string[]>(() => {
  const propUrls = layoutStore.currentPrompt?.props?.urls;
  if (Array.isArray(propUrls) && propUrls.length > 0) {
    return propUrls as string[];
  }
  if (fileStore.selectedCount > 0 && fileStore.req) {
    return fileStore.selected.map((i) => fileStore.req!.items[i].url);
  }
  return [route.path];
});

const humanSize = (bytes: number) => filesize(bytes);

const sign = async () => {
  loading.value = true;
  errorMsg.value = "";
  try {
    signed.value = await mediaApi.signFastDownload(targetUrls.value, "", ttlHours.value);
  } catch (e: any) {
    errorMsg.value = e?.message || String(e);
  } finally {
    loading.value = false;
  }
};

const directDownload = () => {
  if (!signed.value) return;
  window.open(signed.value.url, "_blank");
};

const copyText = async (text: string, key: string) => {
  await copy({ text });
  copiedKey.value = key;
  setTimeout(() => {
    if (copiedKey.value === key) copiedKey.value = "";
  }, 1800);
};

onMounted(sign);
</script>
