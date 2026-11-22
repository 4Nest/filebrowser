<template>
  <div v-show="active" @click="closeHovers" class="overlay"></div>
  <aside class="fb-sidebar" :class="{ active }">
    <div class="fb-sidebar-brand">
      <div class="fb-brand-left" @click="toRoot">
        <div class="fb-brand-icon">
          <img :src="logoURL" alt="File Browser" />
        </div>
        <div class="fb-brand-meta">
          <span class="fb-brand-name">FileBrowser</span>
          <span class="fb-brand-tag">Nest</span>
        </div>
      </div>
      <button
        type="button"
        class="fb-icon-btn"
        :title="isDark ? '切换浅色模式' : '切换暗黑模式'"
        @click="toggleTheme"
      >
        <i class="material-icons">{{ isDark ? "light_mode" : "dark_mode" }}</i>
      </button>
    </div>

    <template v-if="isLoggedIn">
      <div class="fb-sidebar-quick" v-if="user.perm.create">
        <button
          type="button"
          class="fb-create-btn fb-create-btn--primary"
          @click="showHover('newDir')"
        >
          <i class="material-icons">create_new_folder</i>
          <span>新建文件夹</span>
        </button>
        <button
          type="button"
          class="fb-create-btn"
          @click="showHover('newFile')"
          title="新建文件"
        >
          <i class="material-icons">note_add</i>
        </button>
      </div>

      <div class="fb-sidebar-scroll">
        <div class="fb-nav-group">
          <div class="fb-nav-label">工作区</div>
          <button
            type="button"
            class="fb-nav-item"
            :class="{ active: isFiles }"
            @click="toRoot"
          >
            <i class="material-icons">folder_open</i>
            <span>{{ $t("sidebar.myFiles") }}</span>
          </button>
        </div>

        <div class="fb-nav-group">
          <div class="fb-nav-label">系统与管理</div>
          <button
            type="button"
            class="fb-nav-item"
            :class="{ active: $route.path === '/settings/profile' }"
            @click="go('/settings/profile')"
          >
            <i class="material-icons">tune</i>
            <span>{{ $t("settings.profileSettings") }}</span>
          </button>
          <button
            v-if="user.perm.share"
            type="button"
            class="fb-nav-item"
            :class="{ active: $route.path === '/settings/shares' }"
            @click="go('/settings/shares')"
          >
            <i class="material-icons">share</i>
            <span>{{ $t("settings.shareManagement") }}</span>
          </button>
          <button
            v-if="user.perm.admin"
            type="button"
            class="fb-nav-item"
            :class="{ active: $route.path === '/settings/global' }"
            @click="go('/settings/global')"
          >
            <i class="material-icons">settings</i>
            <span>{{ $t("settings.globalSettings") }}</span>
          </button>
          <button
            v-if="user.perm.admin"
            type="button"
            class="fb-nav-item"
            :class="{
              active:
                $route.path === '/settings/users' || $route.name === 'User',
            }"
            @click="go('/settings/users')"
          >
            <i class="material-icons">group</i>
            <span>{{ $t("settings.userManagement") }}</span>
          </button>
          <button type="button" class="fb-nav-item" @click="help">
            <i class="material-icons">help_outline</i>
            <span>{{ $t("sidebar.help") }}</span>
          </button>
        </div>
      </div>

      <div class="fb-sidebar-footer">
        <div class="fb-storage-card" v-if="isFiles && !disableUsedPercentage">
          <div class="fb-storage-head">
            <span>存储空间</span>
            <span class="fb-mono">{{ Math.round(usage.usedPercentage) }}%</span>
          </div>
          <div class="fb-storage-bar">
            <div
              class="fb-storage-fill"
              :style="{ width: `${Math.min(100, usage.usedPercentage)}%` }"
            ></div>
          </div>
          <div class="fb-storage-meta fb-mono">
            {{ usage.used }} / {{ usage.total }}
          </div>
        </div>

        <div class="fb-user-card">
          <div class="fb-user-info" @click="go('/settings/profile')">
            <div class="fb-user-avatar">
              {{ (user.username || "U").slice(0, 1).toUpperCase() }}
            </div>
            <div class="fb-user-text">
              <span class="fb-user-name">{{ user.username }}</span>
              <span class="fb-user-role">{{
                user.perm.admin ? "管理员" : "用户"
              }}</span>
            </div>
          </div>
          <button
            v-if="canLogout"
            type="button"
            class="fb-icon-btn"
            id="logout"
            :title="$t('sidebar.logout')"
            @click="logout"
          >
            <i class="material-icons">logout</i>
          </button>
        </div>
      </div>
    </template>

    <template v-else>
      <div class="fb-sidebar-scroll">
        <router-link
          v-if="!hideLoginButton"
          class="fb-nav-item"
          to="/login"
        >
          <i class="material-icons">login</i>
          <span>{{ $t("sidebar.login") }}</span>
        </router-link>
      </div>
    </template>
  </aside>
</template>

<script>
import { reactive, ref } from "vue";
import { mapActions, mapState } from "pinia";
import { useAuthStore } from "@/stores/auth";
import { useFileStore } from "@/stores/file";
import { useLayoutStore } from "@/stores/layout";

import * as auth from "@/utils/auth";
import {
  version,
  logoURL,
  signup,
  hideLoginButton,
  disableExternal,
  disableUsedPercentage,
  noAuth,
  logoutPage,
  loginPage,
} from "@/utils/constants";
import { getTheme, getMediaPreference, setTheme } from "@/utils/theme";
import { files as api } from "@/api";
import prettyBytes from "pretty-bytes";

const USAGE_DEFAULT = { used: "0 B", total: "0 B", usedPercentage: 0 };

export default {
  name: "sidebar",
  setup() {
    const usage = reactive({ ...USAGE_DEFAULT });
    const isDark = ref((getTheme() || getMediaPreference()) === "dark");
    const toggleTheme = () => {
      const next = isDark.value ? "light" : "dark";
      setTheme(next);
      isDark.value = next === "dark";
    };
    return {
      usage,
      isDark,
      toggleTheme,
      usageAbortController: new AbortController(),
    };
  },
  inject: ["$showError"],
  computed: {
    ...mapState(useAuthStore, ["user", "isLoggedIn"]),
    ...mapState(useFileStore, ["isFiles", "reload"]),
    ...mapState(useLayoutStore, ["currentPromptName"]),
    active() {
      return this.currentPromptName === "sidebar";
    },
    logoURL: () => logoURL,
    signup: () => signup,
    hideLoginButton: () => hideLoginButton,
    version: () => version,
    disableExternal: () => disableExternal,
    disableUsedPercentage: () => disableUsedPercentage,
    canLogout: () => !noAuth && (loginPage || logoutPage !== "/login"),
  },
  methods: {
    ...mapActions(useLayoutStore, ["closeHovers", "showHover"]),
    abortOngoingFetchUsage() {
      this.usageAbortController.abort();
    },
    async fetchUsage() {
      const path = this.$route.path.endsWith("/")
        ? this.$route.path
        : this.$route.path + "/";
      let usageStats = USAGE_DEFAULT;
      if (this.disableUsedPercentage) {
        return Object.assign(this.usage, usageStats);
      }
      try {
        this.abortOngoingFetchUsage();
        this.usageAbortController = new AbortController();
        const usage = await api.usage(path, this.usageAbortController.signal);
        usageStats = {
          used: prettyBytes(usage.used, { binary: true }),
          total: prettyBytes(usage.total, { binary: true }),
          usedPercentage: Math.round((usage.used / usage.total) * 100),
        };
      } catch (error) {
        this.$showError(error);
      }
      return Object.assign(this.usage, usageStats);
    },
    toRoot() {
      this.$router.push({ path: "/files" });
      this.closeHovers();
    },
    go(path) {
      this.$router.push({ path });
      this.closeHovers();
    },
    help() {
      this.showHover("help");
    },
    logout: auth.logout,
  },
  watch: {
    $route: {
      handler(to) {
        if (to.path.includes("/files")) {
          this.fetchUsage();
        }
      },
      immediate: true,
    },
  },
  unmounted() {
    this.abortOngoingFetchUsage();
  },
};
</script>
