<template>
  <div class="fb-settings-stack">
    <!-- 外观与浏览习惯 -->
    <form class="fb-settings-section" @submit="updateSettings">
      <div class="fb-section-head">
        <div>
          <h2>外观与浏览</h2>
          <p>自定义界面主题、语言与文件列表交互行为</p>
        </div>
        <button class="button" type="submit" name="submitProfile">
          <i class="material-icons">check</i>
          <span>保存偏好</span>
        </button>
      </div>

      <div class="fb-setting-list">
        <div class="fb-setting-row">
          <div class="fb-setting-info">
            <span class="fb-setting-title">外观主题</span>
            <span class="fb-setting-desc">切换浅色或暗黑模式，立即生效</span>
          </div>
          <div class="fb-segmented">
            <button
              type="button"
              class="fb-seg-text"
              :class="{ active: themeChoice === 'light' }"
              @click="applyTheme('light')"
            >
              浅色
            </button>
            <button
              type="button"
              class="fb-seg-text"
              :class="{ active: themeChoice === 'dark' }"
              @click="applyTheme('dark')"
            >
              暗黑
            </button>
          </div>
        </div>

        <div class="fb-setting-row">
          <div class="fb-setting-info">
            <span class="fb-setting-title">{{ t("settings.language") }}</span>
            <span class="fb-setting-desc">界面显示语言</span>
          </div>
          <div class="fb-setting-control">
            <languages class="input" v-model:locale="locale" />
          </div>
        </div>

        <div class="fb-setting-row">
          <div class="fb-setting-info">
            <span class="fb-setting-title">文件页默认打开路径</span>
            <span class="fb-setting-desc"
              >登录或点击「我的文件」时自动进入的目录（留空则打开根目录）</span
            >
          </div>
          <div class="fb-setting-control fb-setting-control--wide">
            <input
              class="input fb-mono"
              type="text"
              v-model="localPrefs.defaultPath"
              placeholder="/torrents/qbittorrent"
            />
          </div>
        </div>

        <label class="fb-setting-row fb-setting-row--click">
          <div class="fb-setting-info">
            <span class="fb-setting-title">隐藏点文件（`.` 开头）</span>
            <span class="fb-setting-desc"
              >隐藏 .cache、.config、.bashrc 等系统隐藏文件与目录</span
            >
          </div>
          <input
            type="checkbox"
            class="fb-switch"
            name="hideDotfiles"
            v-model="hideDotfiles"
          />
        </label>

        <label class="fb-setting-row fb-setting-row--click">
          <div class="fb-setting-info">
            <span class="fb-setting-title">单击打开文件与目录</span>
            <span class="fb-setting-desc"
              >开启后单击直接进入目录或预览，关闭则需双击</span
            >
          </div>
          <input
            type="checkbox"
            class="fb-switch"
            name="singleClick"
            v-model="singleClick"
          />
        </label>

        <label class="fb-setting-row fb-setting-row--click">
          <div class="fb-setting-info">
            <span class="fb-setting-title">显示精确修改时间</span>
            <span class="fb-setting-desc"
              >使用完整日期时间代替“1 小时前”等相对时间</span
            >
          </div>
          <input
            type="checkbox"
            class="fb-switch"
            name="dateFormat"
            v-model="dateFormat"
          />
        </label>
      </div>
    </form>

    <!-- 媒体分析与海外高速下载偏好 -->
    <section class="fb-settings-section">
      <div class="fb-section-head">
        <div>
          <h2>媒体与高速下载</h2>
          <p>控制高速签名链接、Aria2 并发线程数及 BDInfo 默认报告格式</p>
        </div>
      </div>

      <div class="fb-setting-list">
        <div class="fb-setting-row">
          <div class="fb-setting-info">
            <span class="fb-setting-title">签名直链默认有效期</span>
            <span class="fb-setting-desc"
              >打开高速下载弹窗时默认生成的链接有效时长</span
            >
          </div>
          <div class="fb-segmented">
            <button
              v-for="h in [2, 6, 24, 48]"
              :key="h"
              type="button"
              class="fb-seg-text fb-mono"
              :class="{ active: localPrefs.fastdlTtlHours === h }"
              @click="updatePref('fastdlTtlHours', h)"
            >
              {{ h }}h
            </button>
          </div>
        </div>

        <div class="fb-setting-row">
          <div class="fb-setting-info">
            <span class="fb-setting-title">Aria2 默认并发线程数</span>
            <span class="fb-setting-desc"
              >用于生成 aria2c 命令的 -x / -s 参数，海外 VPS 推荐 16 线程</span
            >
          </div>
          <div class="fb-segmented">
            <button
              v-for="n in [8, 16, 32]"
              :key="n"
              type="button"
              class="fb-seg-text fb-mono"
              :class="{ active: localPrefs.aria2Threads === n }"
              @click="updatePref('aria2Threads', n)"
            >
              {{ n }} 线程
            </button>
          </div>
        </div>

        <div class="fb-setting-row">
          <div class="fb-setting-info">
            <span class="fb-setting-title">BDInfo 默认展示视图</span>
            <span class="fb-setting-desc"
              >蓝光原盘扫描完成后优先显示的报告标签页</span
            >
          </div>
          <div class="fb-segmented">
            <button
              type="button"
              class="fb-seg-text"
              :class="{ active: localPrefs.bdinfoDefaultTab === 'quick' }"
              @click="updatePref('bdinfoDefaultTab', 'quick')"
            >
              Quick Summary
            </button>
            <button
              type="button"
              class="fb-seg-text"
              :class="{ active: localPrefs.bdinfoDefaultTab === 'bbcode' }"
              @click="updatePref('bdinfoDefaultTab', 'bbcode')"
            >
              BBCode
            </button>
            <button
              type="button"
              class="fb-seg-text"
              :class="{ active: localPrefs.bdinfoDefaultTab === 'full' }"
              @click="updatePref('bdinfoDefaultTab', 'full')"
            >
              Full Report
            </button>
          </div>
        </div>
      </div>
    </section>

    <!-- 截图参数与多图床配置 -->
    <section class="fb-settings-section">
      <div class="fb-section-head">
        <div>
          <h2>截图参数与图床配置</h2>
          <p>默认截图参数及多图床 API 配置（内置 PICX / Pixhost / Chevereto / 兰空 / 自定义）</p>
        </div>
        <button type="button" class="button" @click="saveShotAndHostPrefs">
          <i class="material-icons">check</i>
          <span>保存截图与图床</span>
        </button>
      </div>

      <div class="fb-setting-list">
        <div class="fb-setting-row">
          <div class="fb-setting-info">
            <span class="fb-setting-title">默认截图张数与格式</span>
            <span class="fb-setting-desc">打开截图弹窗时自动带入的张数、图片格式与 HDR 映射策略</span>
          </div>
          <div class="fb-shot-inline-controls">
            <select class="input" v-model.number="localPrefs.screenshotCount">
              <option v-for="n in [2, 3, 4, 6, 8, 10, 12]" :key="n" :value="n">
                {{ n }} 张
              </option>
            </select>
            <select class="input" v-model="localPrefs.screenshotFormat">
              <option value="png">PNG</option>
              <option value="jpg">JPG</option>
              <option value="webp">WebP</option>
            </select>
            <select class="input" v-model="localPrefs.screenshotHdrToneMap">
              <option value="auto">HDR 自动映射</option>
              <option value="on">HDR 强制映射</option>
              <option value="off">HDR 关闭</option>
            </select>
            <select class="input" v-model.number="localPrefs.screenshotWidth">
              <option :value="0">原分辨率</option>
              <option :value="1920">1920px</option>
              <option :value="1280">1280px</option>
            </select>
          </div>
        </div>

        <div class="fb-setting-row">
          <div class="fb-setting-info">
            <span class="fb-setting-title">默认采样区间 (%)</span>
            <span class="fb-setting-desc">自动跳过片头与片尾黑屏区间，默认 10% – 90%</span>
          </div>
          <div class="fb-range-inputs">
            <input
              class="input fb-mono"
              type="number"
              v-model.number="localPrefs.screenshotStartPct"
              min="0"
              max="90"
            />
            <span>–</span>
            <input
              class="input fb-mono"
              type="number"
              v-model.number="localPrefs.screenshotEndPct"
              min="10"
              max="100"
            />
          </div>
        </div>

        <!-- 图床预设选择与参数 -->
        <div class="fb-host-editor">
          <div class="fb-host-tabs">
            <button
              v-for="h in localPrefs.imageHosts"
              :key="h.id"
              type="button"
              class="fb-tab"
              :class="{ active: localPrefs.activeImageHostId === h.id }"
              @click="localPrefs.activeImageHostId = h.id"
            >
              {{ h.name }}
            </button>
          </div>

          <div v-if="activeHost" class="fb-host-form">
            <div class="fb-field">
              <label>上传 API 地址</label>
              <input
                class="input input--block fb-mono"
                type="text"
                v-model="activeHost.endpoint"
                :placeholder="
                  activeHost.provider === 'picx'
                    ? 'https://picx.cx/api/1/upload'
                    : 'https://...'
                "
              />
            </div>

            <div class="fb-field" v-if="activeHost.provider !== 'pixhost'">
              <label>
                {{
                  activeHost.provider === "picx" ||
                  activeHost.provider === "chevereto"
                    ? "X-API-Key（Chevereto v1.1 API Key）"
                    : activeHost.provider === "lsky"
                      ? "Bearer Token"
                      : "API Key / Token"
                }}
              </label>
              <input
                class="input input--block fb-mono"
                type="password"
                v-model="activeHost.apiKey"
                placeholder="输入对应图床的 API Key / Token"
              />
            </div>

            <template v-if="activeHost.provider === 'custom'">
              <div class="fb-field">
                <label>文件表单字段名</label>
                <input
                  class="input input--block fb-mono"
                  type="text"
                  v-model="activeHost.fieldName"
                  placeholder="file / source / image"
                />
              </div>
              <div class="fb-field">
                <label>鉴权 Header 名称</label>
                <input
                  class="input input--block fb-mono"
                  type="text"
                  v-model="activeHost.authHeader"
                  placeholder="Authorization 或 X-API-Key"
                />
              </div>
              <div class="fb-field">
                <label>鉴权值前缀（可选）</label>
                <input
                  class="input input--block fb-mono"
                  type="text"
                  v-model="activeHost.authPrefix"
                  placeholder="例如 Bearer （含末尾空格）或留空"
                />
              </div>
              <div class="fb-field">
                <label>返回图片 URL 的 JSON 路径</label>
                <input
                  class="input input--block fb-mono"
                  type="text"
                  v-model="activeHost.urlJsonPath"
                  placeholder="例如 image.url 或 data.links.url"
                />
              </div>
            </template>
          </div>
        </div>
      </div>
    </section>

    <!-- 账号安全 -->
    <form
      v-if="!noAuth && !authStore.user?.lockPassword"
      class="fb-settings-section"
      @submit="updatePassword"
    >
      <div class="fb-section-head">
        <div>
          <h2>{{ t("settings.changePassword") }}</h2>
          <p>定期更新登录密码以保护服务器文件安全</p>
        </div>
        <button class="button button--flat" type="submit" name="submitPassword">
          更新密码
        </button>
      </div>

      <div class="fb-password-grid">
        <div class="fb-field" v-if="isCurrentPasswordRequired">
          <label>{{ t("settings.currentPassword") }}</label>
          <input
            class="input input--block"
            type="password"
            :placeholder="t('settings.currentPassword')"
            v-model="currentPassword"
            name="current_password"
            autocomplete="current-password"
          />
        </div>
        <div class="fb-field">
          <label>{{ t("settings.newPassword") }}</label>
          <input
            :class="passwordClass"
            type="password"
            :placeholder="t('settings.newPassword')"
            v-model="password"
            name="password"
            autocomplete="new-password"
          />
        </div>
        <div class="fb-field">
          <label>{{ t("settings.newPasswordConfirm") }}</label>
          <input
            :class="passwordClass"
            type="password"
            :placeholder="t('settings.newPasswordConfirm')"
            v-model="passwordConf"
            name="passwordConf"
            autocomplete="new-password"
          />
        </div>
      </div>
    </form>
  </div>
</template>

<script setup lang="ts">
import { useAuthStore } from "@/stores/auth";
import { useLayoutStore } from "@/stores/layout";
import { users as api } from "@/api";
import Languages from "@/components/settings/Languages.vue";
import { getMediaPreference, getTheme, setTheme } from "@/utils/theme";
import { getLocalPrefs, saveLocalPrefs, type LocalPrefs } from "@/utils/prefs";
import { computed, inject, onMounted, reactive, ref } from "vue";
import { useI18n } from "vue-i18n";
import { authMethod, noAuth } from "@/utils/constants";

const layoutStore = useLayoutStore();
const authStore = useAuthStore();
const { t } = useI18n();

const $showSuccess = inject<IToastSuccess>("$showSuccess")!;
const $showError = inject<IToastError>("$showError")!;

const password = ref<string>("");
const passwordConf = ref<string>("");
const currentPassword = ref<string>("");
const isCurrentPasswordRequired = ref<boolean>(false);
const hideDotfiles = ref<boolean>(false);
const singleClick = ref<boolean>(false);
const dateFormat = ref<boolean>(false);
const locale = ref<string>("");

const themeChoice = ref<UserTheme>(getTheme() || getMediaPreference());
const applyTheme = (next: UserTheme) => {
  setTheme(next);
  themeChoice.value = next;
};

const localPrefs = reactive<LocalPrefs>(getLocalPrefs());
const activeHost = computed(() =>
  localPrefs.imageHosts.find((h) => h.id === localPrefs.activeImageHostId)
);
const updatePref = <K extends keyof LocalPrefs>(key: K, val: LocalPrefs[K]) => {
  localPrefs[key] = val;
  saveLocalPrefs({ [key]: val });
  $showSuccess("偏好已保存");
};
const saveShotAndHostPrefs = () => {
  saveLocalPrefs({
    screenshotCount: localPrefs.screenshotCount,
    screenshotFormat: localPrefs.screenshotFormat,
    screenshotQuality: localPrefs.screenshotQuality,
    screenshotStartPct: localPrefs.screenshotStartPct,
    screenshotEndPct: localPrefs.screenshotEndPct,
    screenshotHdrToneMap: localPrefs.screenshotHdrToneMap,
    screenshotWidth: localPrefs.screenshotWidth,
    activeImageHostId: localPrefs.activeImageHostId,
    imageHosts: localPrefs.imageHosts.map((h) => ({ ...h })),
  });
  $showSuccess("截图参数与图床配置已保存");
};

const passwordClass = computed(() => {
  const baseClass = "input input--block";
  if (password.value === "" && passwordConf.value === "") return baseClass;
  if (password.value === passwordConf.value) return `${baseClass} input--green`;
  return `${baseClass} input--red`;
});

onMounted(async () => {
  layoutStore.loading = true;
  if (authStore.user === null) return false;
  locale.value = authStore.user.locale;
  hideDotfiles.value = authStore.user.hideDotfiles;
  singleClick.value = authStore.user.singleClick;
  dateFormat.value = authStore.user.dateFormat;
  layoutStore.loading = false;
  isCurrentPasswordRequired.value = authMethod == "json";
  return true;
});

const updatePassword = async (event: Event) => {
  event.preventDefault();
  if (
    password.value !== passwordConf.value ||
    password.value === "" ||
    (isCurrentPasswordRequired.value && currentPassword.value === "") ||
    authStore.user === null
  ) {
    return;
  }
  try {
    const data = {
      ...authStore.user,
      id: authStore.user.id,
      password: password.value,
    };
    await api.update(data, ["password"], currentPassword.value);
    authStore.updateUser(data);
    $showSuccess(t("settings.passwordUpdated"));
  } catch (e: any) {
    $showError(e);
  } finally {
    password.value = passwordConf.value = currentPassword.value = "";
  }
};

const updateSettings = async (event: Event) => {
  event.preventDefault();
  try {
    if (authStore.user === null) throw new Error("User is not set!");
    const data = {
      ...authStore.user,
      id: authStore.user.id,
      locale: locale.value,
      hideDotfiles: hideDotfiles.value,
      singleClick: singleClick.value,
      dateFormat: dateFormat.value,
    };
    await api.update(data, [
      "locale",
      "hideDotfiles",
      "singleClick",
      "dateFormat",
    ]);
    saveLocalPrefs({ defaultPath: localPrefs.defaultPath.trim() });
    authStore.updateUser(data);
    $showSuccess(t("settings.settingsUpdated"));
  } catch (err) {
    if (err instanceof Error) $showError(err);
  }
};
</script>
