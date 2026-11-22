<template>
  <div id="login" :class="{ recaptcha: recaptcha }">
    <button
      type="button"
      class="fb-login-theme"
      :title="isDark ? '切换浅色模式' : '切换暗黑模式'"
      @click="toggleTheme"
    >
      <i class="material-icons">{{ isDark ? "light_mode" : "dark_mode" }}</i>
    </button>

    <form class="fb-login-card" @submit="submit">
      <div class="fb-login-brand">
        <div class="fb-login-logo">
          <img :src="logoURL" alt="File Browser" />
        </div>
        <h1>{{ name }}</h1>
        <p class="fb-login-sub">
          {{ createMode ? t("login.createAnAccount") : "高速媒体与文件工作台" }}
        </p>
      </div>

      <p v-if="reason != null" class="logout-message">
        <i class="material-icons">info_outline</i>
        <span>{{ t(`login.logout_reasons.${reason}`) }}</span>
      </p>

      <div v-if="error !== ''" class="wrong">
        <i class="material-icons">error_outline</i>
        <span>{{ error }}</span>
      </div>

      <div class="fb-login-field">
        <label for="login-username">{{ t("login.username") }}</label>
        <input
          id="login-username"
          autofocus
          class="input input--block"
          type="text"
          autocomplete="username"
          autocapitalize="off"
          v-model="username"
          :placeholder="t('login.username')"
        />
      </div>

      <div class="fb-login-field">
        <label for="login-password">{{ t("login.password") }}</label>
        <div class="fb-password-wrap">
          <input
            id="login-password"
            class="input input--block"
            :type="showPassword ? 'text' : 'password'"
            autocomplete="current-password"
            v-model="password"
            :placeholder="t('login.password')"
          />
          <button
            type="button"
            class="fb-password-toggle"
            :aria-label="showPassword ? '隐藏密码' : '显示密码'"
            @click="showPassword = !showPassword"
          >
            <i class="material-icons">{{
              showPassword ? "visibility_off" : "visibility"
            }}</i>
          </button>
        </div>
      </div>

      <div class="fb-login-field" v-if="createMode">
        <label for="login-password-confirm">{{
          t("login.passwordConfirm")
        }}</label>
        <input
          id="login-password-confirm"
          class="input input--block"
          :type="showPassword ? 'text' : 'password'"
          autocomplete="new-password"
          v-model="passwordConfirm"
          :placeholder="t('login.passwordConfirm')"
        />
      </div>

      <div v-if="recaptcha" id="recaptcha"></div>

      <button
        class="button button--block fb-login-submit"
        type="submit"
        :disabled="submitting"
      >
        <span v-if="submitting" class="fb-login-spinner"></span>
        <span>{{
          submitting
            ? "正在登录…"
            : createMode
              ? t("login.signup")
              : t("login.submit")
        }}</span>
      </button>

      <p class="fb-login-switch" @click="toggleMode" v-if="signup">
        {{ createMode ? t("login.loginInstead") : t("login.createAnAccount") }}
      </p>
    </form>
  </div>
</template>

<script setup lang="ts">
import { StatusError } from "@/api/utils";
import * as auth from "@/utils/auth";
import {
  name,
  logoURL,
  recaptcha,
  recaptchaKey,
  signup,
} from "@/utils/constants";
import { getMediaPreference, getTheme, setTheme } from "@/utils/theme";
import { inject, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { useRoute, useRouter } from "vue-router";

const createMode = ref<boolean>(false);
const error = ref<string>("");
const username = ref<string>("");
const password = ref<string>("");
const passwordConfirm = ref<string>("");
const showPassword = ref<boolean>(false);
const submitting = ref<boolean>(false);

const isDark = ref<boolean>((getTheme() || getMediaPreference()) === "dark");
const toggleTheme = () => {
  const next = isDark.value ? "light" : "dark";
  setTheme(next);
  isDark.value = next === "dark";
};

const route = useRoute();
const router = useRouter();
const { t } = useI18n({});
const toggleMode = () => (createMode.value = !createMode.value);

const $showError = inject<IToastError>("$showError")!;

const reason = route.query["logout-reason"] ?? null;

const submit = async (event: Event) => {
  event.preventDefault();
  event.stopPropagation();
  if (submitting.value) return;

  error.value = "";
  const redirect = (route.query.redirect || "/files/") as string;

  let captcha = "";
  if (recaptcha) {
    captcha = window.grecaptcha.getResponse();

    if (captcha === "") {
      error.value = t("login.wrongCredentials");
      return;
    }
  }

  if (createMode.value) {
    if (password.value !== passwordConfirm.value) {
      error.value = t("login.passwordsDontMatch");
      return;
    }
  }

  submitting.value = true;
  try {
    if (createMode.value) {
      await auth.signup(username.value, password.value);
    }

    await auth.login(username.value, password.value, captcha);
    router.push({ path: redirect });
  } catch (e: any) {
    if (e instanceof StatusError) {
      if (e.status === 409) {
        error.value = t("login.usernameTaken");
      } else if (e.status === 403) {
        error.value = t("login.wrongCredentials");
      } else if (e.status === 400) {
        const match = e.message.match(/minimum length is (\d+)/);
        if (match) {
          error.value = t("login.passwordTooShort", { min: match[1] });
        } else {
          error.value = e.message;
        }
      } else {
        $showError(e);
      }
    }
  } finally {
    submitting.value = false;
  }
};

onMounted(() => {
  if (!recaptcha) return;

  window.grecaptcha.ready(function () {
    window.grecaptcha.render("recaptcha", {
      sitekey: recaptchaKey,
    });
  });
});
</script>
