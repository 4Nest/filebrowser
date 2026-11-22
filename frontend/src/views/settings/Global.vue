<template>
  <errors v-if="error" :errorCode="error.status" />
  <form
    v-else-if="!layoutStore.loading && settings !== null"
    class="fb-settings-stack"
    @submit.prevent="save"
  >
    <!-- 实例与访问安全 -->
    <section class="fb-settings-section">
      <div class="fb-section-head">
        <div>
          <h2>实例与安全</h2>
          <p>控制站点名称、注册开关与密码安全策略</p>
        </div>
        <button class="button" type="submit">
          <i class="material-icons">check</i>
          <span>保存全局设置</span>
        </button>
      </div>

      <div class="fb-setting-list">
        <div class="fb-setting-row">
          <div class="fb-setting-info">
            <span class="fb-setting-title">实例名称</span>
            <span class="fb-setting-desc">显示在登录页与浏览器标签页的标题</span>
          </div>
          <div class="fb-setting-control">
            <input
              class="input"
              type="text"
              v-model="settings.branding.name"
              placeholder="File Browser"
            />
          </div>
        </div>

        <label class="fb-setting-row fb-setting-row--click">
          <div class="fb-setting-info">
            <span class="fb-setting-title">允许公开注册</span>
            <span class="fb-setting-desc">私人 VPS 强烈建议保持关闭</span>
          </div>
          <input type="checkbox" class="fb-switch" v-model="settings.signup" />
        </label>

        <label class="fb-setting-row fb-setting-row--click">
          <div class="fb-setting-info">
            <span class="fb-setting-title">在公开分享页隐藏登录按钮</span>
            <span class="fb-setting-desc">访客打开分享链接时不显示侧栏登录入口</span>
          </div>
          <input
            type="checkbox"
            class="fb-switch"
            v-model="settings.hideLoginButton"
          />
        </label>

        <label class="fb-setting-row fb-setting-row--click">
          <div class="fb-setting-info">
            <span class="fb-setting-title">隐藏侧栏磁盘容量卡片</span>
            <span class="fb-setting-desc">关闭后左侧栏底部不再展示存储占用统计</span>
          </div>
          <input
            type="checkbox"
            class="fb-switch"
            v-model="settings.branding.disableUsedPercentage"
          />
        </label>

        <div class="fb-setting-row">
          <div class="fb-setting-info">
            <span class="fb-setting-title">最小密码长度</span>
            <span class="fb-setting-desc">新建用户或修改密码时的最低字符要求</span>
          </div>
          <div class="fb-setting-control fb-setting-control--sm">
            <vue-number-input
              controls
              v-model.number="settings.minimumPasswordLength"
              :min="4"
            />
          </div>
        </div>
      </div>
    </section>

    <!-- 上传与断点续传 -->
    <section class="fb-settings-section">
      <div class="fb-section-head">
        <div>
          <h2>上传与分片传输 (TUS)</h2>
          <p>优化海外 VPS 大文件上传的稳定性与分片吞吐</p>
        </div>
      </div>

      <div class="fb-setting-list">
        <div class="fb-setting-row">
          <div class="fb-setting-info">
            <span class="fb-setting-title">分片大小</span>
            <span class="fb-setting-desc"
              >支持 10MB、20MB 等格式，跨国网络建议 10MB–20MB</span
            >
          </div>
          <div class="fb-setting-control fb-setting-control--sm">
            <input
              class="input fb-mono"
              type="text"
              v-model="formattedChunkSize"
              id="tus-chunkSize"
            />
          </div>
        </div>

        <div class="fb-setting-row">
          <div class="fb-setting-info">
            <span class="fb-setting-title">分片失败重试次数</span>
            <span class="fb-setting-desc">网络抖动时单个分片的最大自动重试次数</span>
          </div>
          <div class="fb-setting-control fb-setting-control--sm">
            <vue-number-input
              controls
              v-model.number="settings.tus.retryCount"
              :min="0"
            />
          </div>
        </div>
      </div>
    </section>

    <!-- 高级设置（折叠收起） -->
    <details class="fb-settings-details">
      <summary>
        <span>高级配置：路径过滤规则与新用户默认模板</span>
        <i class="material-icons">expand_more</i>
      </summary>

      <div class="fb-settings-details-body">
        <div class="fb-sub-block">
          <h3>全局路径访问规则</h3>
          <p class="fb-setting-desc">通过正则表达式隐藏或限制特定路径的访问</p>
          <rules v-model:rules="settings.rules" />
        </div>

        <div class="fb-sub-block">
          <h3>新用户默认权限模板</h3>
          <p class="fb-setting-desc">创建新成员账号时自动套用的初始权限</p>
          <user-form
            :isNew="false"
            :isDefault="true"
            v-model:user="settings.defaults"
          />
        </div>

        <div class="fb-sub-block" v-if="enableExec">
          <h3>Shell 执行前缀</h3>
          <p class="fb-setting-desc">例如 bash -c</p>
          <input
            class="input input--block"
            type="text"
            placeholder="bash -c"
            v-model="shellValue"
          />
        </div>
      </div>
    </details>
  </form>
</template>

<script setup lang="ts">
import { settings as api } from "@/api";
import { StatusError } from "@/api/utils";
import Rules from "@/components/settings/Rules.vue";
import UserForm from "@/components/settings/UserForm.vue";
import { useLayoutStore } from "@/stores/layout";
import { enableExec } from "@/utils/constants";
import Errors from "@/views/Errors.vue";
import { computed, inject, onBeforeUnmount, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";

const error = ref<StatusError | null>(null);
const settings = ref<ISettings | null>(null);
const debounceTimeout = ref<number | null>(null);
const pendingChunkSize = ref<string | null>(null);
const shellValue = ref<string>("");

const $showError = inject<IToastError>("$showError")!;
const $showSuccess = inject<IToastSuccess>("$showSuccess")!;
const { t } = useI18n();
const layoutStore = useLayoutStore();

const formattedChunkSize = computed({
  get() {
    return settings?.value?.tus?.chunkSize
      ? formatBytes(settings?.value?.tus?.chunkSize)
      : "";
  },
  set(value: string) {
    if (debounceTimeout.value) clearTimeout(debounceTimeout.value);
    pendingChunkSize.value = value;
    debounceTimeout.value = window.setTimeout(applyChunkSize, 1200);
  },
});

const applyChunkSize = () => {
  if (debounceTimeout.value) {
    clearTimeout(debounceTimeout.value);
    debounceTimeout.value = null;
  }
  if (pendingChunkSize.value === null) return;
  if (settings.value) {
    settings.value.tus.chunkSize = parseBytes(pendingChunkSize.value);
  }
  pendingChunkSize.value = null;
};

const save = async () => {
  if (settings.value === null) return false;
  applyChunkSize();
  const newSettings: ISettings = {
    ...settings.value,
    shell: shellValue.value
      .trim()
      .split(" ")
      .filter((s) => s !== ""),
  };
  try {
    await api.update(newSettings);
    $showSuccess(t("settings.settingsUpdated"));
  } catch (e: any) {
    $showError(e);
  }
  return true;
};

const parseBytes = (input: string) => {
  const regex = /^(\d+)(\.\d+)?(B|K|KB|M|MB|G|GB|T|TB)?$/i;
  const matches = input.match(regex);
  if (matches) {
    const size = parseFloat(matches[1].concat(matches[2] || ""));
    let unit: keyof SettingsUnit = (
      matches[3] ?? "B"
    ).toUpperCase() as keyof SettingsUnit;
    if (!unit.endsWith("B")) unit += "B";
    const units: SettingsUnit = {
      KB: 1024,
      MB: 1024 ** 2,
      GB: 1024 ** 3,
      TB: 1024 ** 4,
    };
    return size * (units[unit as keyof SettingsUnit] || 1);
  }
  return 1024 ** 2;
};

const formatBytes = (bytes: number) => {
  const units = ["B", "KB", "MB", "GB", "TB"];
  let size = bytes;
  let unitIndex = 0;
  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024;
    unitIndex++;
  }
  return `${size}${units[unitIndex]}`;
};

onMounted(async () => {
  try {
    layoutStore.loading = true;
    const original: ISettings = await api.get();
    settings.value = { ...original };
    shellValue.value = (original.shell || []).join(" ");
  } catch (err) {
    if (err instanceof Error) error.value = err;
  } finally {
    layoutStore.loading = false;
  }
});

onBeforeUnmount(() => {
  if (debounceTimeout.value) clearTimeout(debounceTimeout.value);
});
</script>
