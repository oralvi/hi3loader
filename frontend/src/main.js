import "./style.css";
import { applyLocale, getLocale, initI18n, listLocales, t, translateMessage } from "./i18n";

import {
  BackgroundDataURL,
  BrowseBackgroundImage,
  Bootstrap,
  CheckGameUpdate,
  BrowseGamePath,
  BrowseLauncherPath,
  ClearCurrentAccount,
  LaunchUpdater,
  LaunchGame,
  LogSnapshot,
  Login,
  CancelCaptchaLogin,
  ReloadCaptchaLogin,
  PauseMonitor,
  ResetQuitFlag,
  ResetBackground,
  RecordClientMessage,
  ResumeMonitor,
  SaveCredentialSettings,
  SaveFeatureSettings,
  SaveLauncherPath,
  ScanWindow,
  SelectSavedAccount,
  UpdateBackground,
} from "../wailsjs/go/main/App";
import { EventsOn, Quit } from "../wailsjs/runtime/runtime";

const shellBaseWidth = 1440;
const shellBaseHeight = 920;
const sensitiveKeys = new Set([
  "password",
  "access_key",
]);
const largeBlobKeys = new Set();
const skipUpdatePromptStorageKey = "hi3loader.skip-update-prompt-once";

function buildInfoText(key) {
  const zh = {
    title: "构建信息",
    version: "版本号",
    buildDate: "构建时间",
    developer: "开发者",
    license: "License",
  };
  const en = {
    title: "Build Info",
    version: "Version",
    buildDate: "Build Date",
    developer: "Developer",
    license: "License",
  };
  const table = getLocale() === "en-US" ? en : zh;
  return table[key] || key;
}

function updateAckText() {
  return getLocale() === "en-US" ? "OK" : "好的";
}

function markSkipUpdatePromptOnce() {
  try {
    window.sessionStorage.setItem(skipUpdatePromptStorageKey, "1");
  } catch (_) {
  }
}

function consumeSkipUpdatePromptOnce() {
  try {
    if (window.sessionStorage.getItem(skipUpdatePromptStorageKey) !== "1") {
      return false;
    }
    window.sessionStorage.removeItem(skipUpdatePromptStorageKey);
    return true;
  } catch (_) {
    return false;
  }
}

void async function main() {
await initI18n();

document.querySelector("#app").innerHTML = `
  <div class="custom-background" id="customBackground" hidden></div>
  <main class="shell">
    <section class="panel topbar">
      <div class="status-grid">
        <article class="status-card">
          <div class="status-head">
            <span class="status-dot" id="appDot"></span>
            <span class="status-name">${t("topbar.window")}</span>
          </div>
          <strong class="status-value" id="appValue">${t("topbar.starting")}</strong>
        </article>
        <article class="status-card">
          <div class="status-head">
            <span class="status-dot" id="sessionDot"></span>
            <span class="status-name">${t("topbar.session")}</span>
          </div>
          <strong class="status-value" id="sessionValue">${t("common.unknown")}</strong>
        </article>
        <article class="status-card">
          <div class="status-head">
            <span class="status-dot" id="dispatchDot"></span>
            <span class="status-name">${t("topbar.api")}</span>
          </div>
          <strong class="status-value" id="dispatchValue">${t("topbar.apiPending")}</strong>
        </article>
        <article class="status-card">
          <div class="status-head">
            <span class="status-dot" id="gameDot"></span>
            <span class="status-name">${t("topbar.gamePath")}</span>
          </div>
          <strong class="status-value" id="gameValue">${t("topbar.notSet")}</strong>
        </article>
      </div>
      <div class="topbar-actions">
        <button class="button button-launch-top" id="launchGameBtn" type="button">${t("topbar.launchGame")}</button>
        <button class="icon-button" id="settingsBtn" type="button" aria-label="${t("topbar.openSettings")}">
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <path d="M19.2 13.5a7.7 7.7 0 0 0 .1-1.5 7.7 7.7 0 0 0-.1-1.5l2-1.6a.7.7 0 0 0 .2-.9l-1.9-3.2a.7.7 0 0 0-.9-.3l-2.4 1a7.1 7.1 0 0 0-2.6-1.5l-.4-2.6a.7.7 0 0 0-.7-.6H9.5a.7.7 0 0 0-.7.6l-.4 2.6a7.1 7.1 0 0 0-2.6 1.5l-2.4-1a.7.7 0 0 0-.9.3L.6 8a.7.7 0 0 0 .2.9l2 1.6a7.7 7.7 0 0 0-.1 1.5 7.7 7.7 0 0 0 .1 1.5l-2 1.6a.7.7 0 0 0-.2.9l1.9 3.2a.7.7 0 0 0 .9.3l2.4-1a7.1 7.1 0 0 0 2.6 1.5l.4 2.6a.7.7 0 0 0 .7.6h3.8a.7.7 0 0 0 .7-.6l.4-2.6a7.1 7.1 0 0 0 2.6-1.5l2.4 1a.7.7 0 0 0 .9-.3l1.9-3.2a.7.7 0 0 0-.2-.9l-2-1.6ZM11.4 15.6A3.6 3.6 0 1 1 15 12a3.6 3.6 0 0 1-3.6 3.6Z"/>
          </svg>
        </button>
      </div>
    </section>

    <section class="workspace">
      <article class="panel action-panel">
        <div class="panel-head action-head">
        <div>
            <h2>${t("action.brand")}</h2>
          </div>
        </div>

        <div class="info-grid info-grid-compact">
          <div class="info-card">
            <span>${t("action.recentAction")}</span>
            <strong id="actionValue">${t("action.idle")}</strong>
          </div>
          <div class="info-card">
            <span>${t("action.recentError")}</span>
            <strong id="errorValue">${t("common.none")}</strong>
          </div>
        </div>

        <div class="action-stack">
          <div class="primary-actions">
            <button class="button action-button primary-button" id="scanWindowBtn" type="button">${t("action.scanWindow")}</button>
          </div>
        </div>

        <div class="response-box" id="responseBox">${t("action.responsePlaceholder")}</div>
      </article>

      <article class="panel log-panel">
        <div class="panel-head panel-head-tight">
          <div>
            <h2>${t("action.logs")}</h2>
          </div>
        </div>
        <div class="log-list" id="logList"></div>
      </article>
    </section>

    <section class="settings-backdrop" id="settingsBackdrop" hidden></section>
    <aside class="settings-sheet" id="settingsSheet" hidden>
      <div class="panel settings-panel">
        <header class="settings-head">
          <div>
            <h2>${t("settings.title")}</h2>
          </div>
          <div class="settings-top-actions">
            <button class="button button-small" id="saveBtn" type="button">${t("common.save")}</button>
            <button class="button button-solid button-small settings-close-button" id="settingsCloseBtn" type="button">${t("common.close")}</button>
          </div>
        </header>

        <div class="settings-grid">
          <label class="field settings-card settings-card-span account-card">
            <span>${t("settings.accounts")}</span>
            <div class="account-picker-row">
              <div class="locale-picker account-picker" id="accountPicker">
                <button class="locale-trigger" id="accountTrigger" type="button" aria-haspopup="listbox" aria-expanded="false" aria-label="${t("settings.accounts")}">
                  <span class="locale-value" id="accountValue">${t("settings.noSavedAccounts")}</span>
                  <span class="locale-chevron" aria-hidden="true"></span>
                </button>
                <div class="locale-menu" id="accountMenu" role="listbox" hidden></div>
              </div>
              <button class="button button-solid account-icon-button account-edit-button" id="editAccountBtn" type="button" aria-label="${t("settings.editAccount")}" title="${t("settings.editAccount")}">
                <svg viewBox="0 0 24 24" aria-hidden="true">
                  <path d="M3 17.25V21h3.75l11-11.03-3.75-3.72L3 17.25Zm17.71-10.04a1 1 0 0 0 0-1.41l-2.5-2.5a1 1 0 0 0-1.41 0l-1.96 1.94 3.75 3.72 2.12-2.1Z"/>
                </svg>
              </button>
              <button class="button button-solid account-icon-button account-add-button" id="addAccountBtn" type="button" aria-label="${t("settings.addAccount")}" title="${t("settings.addAccount")}">
                <span aria-hidden="true">+</span>
              </button>
              <button class="button button-ghost account-icon-button account-clear-button" id="clearAccountBtn" type="button" aria-label="${t("common.clear")}" title="${t("common.clear")}">
                <svg viewBox="0 0 24 24" aria-hidden="true">
                  <path d="M9 3h6l1 2h4v2H4V5h4l1-2Zm1 6h2v8h-2V9Zm4 0h2v8h-2V9ZM7 9h2v8H7V9Zm-1 11h12a1 1 0 0 0 1-1V8H5v11a1 1 0 0 0 1 1Z"/>
                </svg>
              </button>
            </div>
            <button class="account-status-button" id="accountStatusBtn" type="button">${t("auth.status.none")}</button>
          </label>
          <label class="field settings-card">
            <span>${t("settings.nickname")}</span>
            <input id="asteriskNameInput" autocomplete="off" placeholder="${t("settings.nicknamePlaceholder")}" />
          </label>
          <label class="field settings-card">
            <span>界面语言 / Language</span>
            <div class="locale-picker" id="localePicker">
              <button class="locale-trigger" id="localeTrigger" type="button" aria-haspopup="listbox" aria-expanded="false" aria-label="界面语言 / Language">
                <span class="locale-value" id="localeValue"></span>
                <span class="locale-chevron" aria-hidden="true"></span>
              </button>
              <div class="locale-menu" id="localeMenu" role="listbox" hidden></div>
            </div>
          </label>
          <label class="field settings-card">
            <span>${t("settings.apiAddress")}</span>
            <input id="loaderApiInput" autocomplete="off" placeholder="${t("settings.apiAddressPlaceholder")}" />
          </label>
        </div>

        <section class="settings-section settings-section-toggles">
          <label class="toggle">
            <input id="panelBlurInput" type="checkbox" />
            <span class="toggle-control" aria-hidden="true"></span>
            <span class="toggle-copy">
              <strong>${t("settings.blurTitle")}</strong>
            </span>
          </label>
          <label class="toggle">
            <input id="autoWindowCaptureInput" type="checkbox" />
            <span class="toggle-control" aria-hidden="true"></span>
            <span class="toggle-copy">
              <strong>${t("settings.windowTitle")}</strong>
            </span>
          </label>
          <label class="toggle">
            <input id="autoCloseInput" type="checkbox" />
            <span class="toggle-control" aria-hidden="true"></span>
            <span class="toggle-copy">
              <strong>${t("settings.exitTitle")}</strong>
            </span>
          </label>
        </section>

        <section class="settings-section settings-section-paths">
          <label class="field path-field path-field-required">
            <span>${t("settings.gameDirectory")}</span>
            <div class="path-row">
              <input id="gamePathInput" readonly placeholder="${t("settings.gameDirectoryPlaceholder")}" />
              <div class="inline-actions">
                <button class="button button-solid path-button" id="browseGamePathBtn" type="button">${t("common.browse")}</button>
                <button class="button button-ghost path-button" id="clearGamePathBtn" type="button">${t("common.clear")}</button>
              </div>
            </div>
          </label>
          <label class="field path-field path-field-optional">
            <span>${t("settings.launcherPath")}</span>
            <div class="path-row">
              <input id="launcherPathInput" autocomplete="off" placeholder="${t("settings.launcherPathPlaceholder")}" />
              <div class="inline-actions">
                <button class="button button-solid path-button" id="browseLauncherPathBtn" type="button">${t("common.browse")}</button>
                <button class="button button-ghost path-button" id="clearLauncherPathBtn" type="button">${t("common.clear")}</button>
              </div>
            </div>
          </label>
        </section>

        <div class="settings-split">
          <section class="settings-section background-section">
            <div class="section-labels">
              <span class="section-title">${t("settings.background")}</span>
              <small class="section-hint section-status-pill" id="backgroundStatusValue" data-tone="warn">${t("settings.backgroundUnset")}</small>
            </div>
            <div class="background-actions">
              <button class="button button-solid path-button" id="browseBackgroundBtn" type="button">${t("common.browse")}</button>
              <button class="button button-ghost path-button" id="resetBackgroundBtn" type="button">${t("common.reset")}</button>
            </div>
          </section>

          <section class="settings-section opacity-section">
            <div class="section-labels">
              <span class="section-title">${t("settings.uiOpacity")}</span>
              <small class="section-hint section-value-pill"><strong id="backgroundOpacityValue">35%</strong></small>
            </div>
            <div class="opacity-control">
              <input id="backgroundOpacityInput" type="range" min="0" max="100" step="1" value="19" />
            </div>
          </section>

          <section class="settings-section buildinfo-section">
            <div class="section-labels">
              <span class="section-title">${buildInfoText("title")}</span>
            </div>
            <div class="buildinfo-list">
              <div class="buildinfo-item">
                <span class="buildinfo-label">${buildInfoText("version")}</span>
                <strong class="buildinfo-value" id="buildVersionValue">-</strong>
              </div>
              <div class="buildinfo-item">
                <span class="buildinfo-label">${buildInfoText("buildDate")}</span>
                <strong class="buildinfo-value" id="buildDateValue">-</strong>
              </div>
              <div class="buildinfo-item">
                <span class="buildinfo-label">${buildInfoText("developer")}</span>
                <strong class="buildinfo-value" id="buildDeveloperValue">-</strong>
              </div>
            </div>
          </section>
        </div>

      </div>
    </aside>

    <section class="update-overlay" id="updateOverlay" hidden>
      <article class="update-modal panel">
        <header class="update-head">
          <div class="update-title-wrap">
            <div class="update-icon" aria-hidden="true">
              <svg viewBox="0 0 24 24">
                <path d="M12 2 1.8 20.5a1 1 0 0 0 .87 1.5h18.66a1 1 0 0 0 .87-1.5L12 2Zm0 6.2a1 1 0 0 1 1 1v4.8a1 1 0 1 1-2 0V9.2a1 1 0 0 1 1-1Zm0 10.1a1.25 1.25 0 1 1 0-2.5 1.25 1.25 0 0 1 0 2.5Z"/>
              </svg>
            </div>
            <div>
              <h2 id="updateTitle">${t("update.title")}</h2>
            </div>
          </div>
          <button class="button button-solid captcha-close" id="updateCloseBtn" type="button">${t("common.close")}</button>
        </header>
        <p class="update-copy" id="updateCopy"></p>
        <div class="update-version-grid">
          <div class="update-version-card">
            <span>${t("update.localLabel")}</span>
            <strong id="updateLocalValue">-</strong>
          </div>
          <div class="update-version-card">
            <span>${t("update.remoteLabel")}</span>
            <strong id="updateRemoteValue">-</strong>
          </div>
        </div>
        <p class="update-note" id="updateNote"></p>
        <div class="update-actions" id="updateActions"></div>
      </article>
    </section>

    <section class="captcha-overlay" id="captchaOverlay" hidden>
      <article class="captcha-modal">
        <header class="captcha-head">
          <div>
            <p class="eyebrow">${t("captcha.eyebrow")}</p>
            <h2>${t("captcha.title")}</h2>
          </div>
          <button class="button button-solid captcha-close" id="captchaCloseBtn" type="button">${t("common.hide")}</button>
        </header>
        <p class="captcha-copy" id="captchaCopy">${t("captcha.copy")}</p>
        <iframe class="captcha-frame" id="captchaFrame" title="${t("captcha.frameTitle")}"></iframe>
      </article>
    </section>
    <section class="auth-overlay" id="authOverlay" hidden>
      <article class="auth-modal panel">
        <header class="captcha-head auth-head">
          <div>
            <h2 id="authTitle">${t("auth.title")}</h2>
          </div>
          <button class="button button-solid captcha-close" id="authCloseBtn" type="button">${t("common.close")}</button>
        </header>
        <p class="auth-copy" id="authCopy">${t("auth.copy")}</p>
        <div class="settings-grid auth-grid" id="authFields">
          <label class="field settings-card">
            <span>${t("settings.account")}</span>
            <input id="loginAccountInput" autocomplete="off" placeholder="${t("settings.accountPlaceholder")}" />
          </label>
          <label class="field settings-card">
            <span>${t("settings.password")}</span>
            <input id="loginPasswordInput" type="password" autocomplete="new-password" placeholder="${t("settings.passwordPlaceholder")}" />
          </label>
        </div>
        <label class="toggle toggle-compact auth-remember-toggle" id="authRememberToggle">
          <input id="rememberPasswordInput" type="checkbox" />
          <span class="toggle-control" aria-hidden="true"></span>
          <span class="toggle-copy">
            <strong>${t("auth.rememberPassword")}</strong>
          </span>
        </label>
        <div class="auth-actions">
          <button class="button button-solid" id="authCancelBtn" type="button">${t("common.close")}</button>
          <button class="button button-solid" id="authReloadBtn" type="button" hidden>${t("auth.reloadCaptcha")}</button>
          <button class="button button-accent" id="authSubmitBtn" type="button">${t("common.login")}</button>
        </div>
      </article>
    </section>
  </main>
`;

function syncShellScale() {
  const safeWidth = Math.max(window.innerWidth - 24, 320);
  const safeHeight = Math.max(window.innerHeight - 24, 240);
  const scale = Math.min(safeWidth / shellBaseWidth, safeHeight / shellBaseHeight);
  document.documentElement.style.setProperty("--app-scale", String(Math.max(scale, 0.1)));
}

window.addEventListener("resize", syncShellScale);
syncShellScale();

const elements = {
  customBackground: document.getElementById("customBackground"),
  appDot: document.getElementById("appDot"),
  appValue: document.getElementById("appValue"),
  sessionDot: document.getElementById("sessionDot"),
  sessionValue: document.getElementById("sessionValue"),
  dispatchDot: document.getElementById("dispatchDot"),
  dispatchValue: document.getElementById("dispatchValue"),
  gameDot: document.getElementById("gameDot"),
  gameValue: document.getElementById("gameValue"),
  actionValue: document.getElementById("actionValue"),
  errorValue: document.getElementById("errorValue"),
  accountPicker: document.getElementById("accountPicker"),
  accountTrigger: document.getElementById("accountTrigger"),
  accountValue: document.getElementById("accountValue"),
  accountMenu: document.getElementById("accountMenu"),
  editAccountBtn: document.getElementById("editAccountBtn"),
  addAccountBtn: document.getElementById("addAccountBtn"),
  clearAccountBtn: document.getElementById("clearAccountBtn"),
  accountStatusBtn: document.getElementById("accountStatusBtn"),
  asteriskNameInput: document.getElementById("asteriskNameInput"),
  loaderApiInput: document.getElementById("loaderApiInput"),
  localePicker: document.getElementById("localePicker"),
  localeTrigger: document.getElementById("localeTrigger"),
  localeValue: document.getElementById("localeValue"),
  localeMenu: document.getElementById("localeMenu"),
  gamePathInput: document.getElementById("gamePathInput"),
  launcherPathInput: document.getElementById("launcherPathInput"),
  backgroundOpacityInput: document.getElementById("backgroundOpacityInput"),
  backgroundOpacityValue: document.getElementById("backgroundOpacityValue"),
  backgroundStatusValue: document.getElementById("backgroundStatusValue"),
  buildVersionValue: document.getElementById("buildVersionValue"),
  buildDateValue: document.getElementById("buildDateValue"),
  buildDeveloperValue: document.getElementById("buildDeveloperValue"),
  panelBlurInput: document.getElementById("panelBlurInput"),
  autoWindowCaptureInput: document.getElementById("autoWindowCaptureInput"),
  autoCloseInput: document.getElementById("autoCloseInput"),
  responseBox: document.getElementById("responseBox"),
  logList: document.getElementById("logList"),
  launchGameBtn: document.getElementById("launchGameBtn"),
  settingsBtn: document.getElementById("settingsBtn"),
  settingsBackdrop: document.getElementById("settingsBackdrop"),
  settingsSheet: document.getElementById("settingsSheet"),
  settingsCloseBtn: document.getElementById("settingsCloseBtn"),
  updateOverlay: document.getElementById("updateOverlay"),
  updateTitle: document.getElementById("updateTitle"),
  updateCopy: document.getElementById("updateCopy"),
  updateLocalValue: document.getElementById("updateLocalValue"),
  updateRemoteValue: document.getElementById("updateRemoteValue"),
  updateNote: document.getElementById("updateNote"),
  updateActions: document.getElementById("updateActions"),
  updateCloseBtn: document.getElementById("updateCloseBtn"),
  browseGamePathBtn: document.getElementById("browseGamePathBtn"),
  clearGamePathBtn: document.getElementById("clearGamePathBtn"),
  browseLauncherPathBtn: document.getElementById("browseLauncherPathBtn"),
  clearLauncherPathBtn: document.getElementById("clearLauncherPathBtn"),
  browseBackgroundBtn: document.getElementById("browseBackgroundBtn"),
  resetBackgroundBtn: document.getElementById("resetBackgroundBtn"),
  saveBtn: document.getElementById("saveBtn"),
  scanWindowBtn: document.getElementById("scanWindowBtn"),
  captchaOverlay: document.getElementById("captchaOverlay"),
  captchaFrame: document.getElementById("captchaFrame"),
  captchaCopy: document.getElementById("captchaCopy"),
  captchaCloseBtn: document.getElementById("captchaCloseBtn"),
  authOverlay: document.getElementById("authOverlay"),
  authTitle: document.getElementById("authTitle"),
  authCopy: document.getElementById("authCopy"),
  authCloseBtn: document.getElementById("authCloseBtn"),
  authCancelBtn: document.getElementById("authCancelBtn"),
  authReloadBtn: document.getElementById("authReloadBtn"),
  authSubmitBtn: document.getElementById("authSubmitBtn"),
  authFields: document.getElementById("authFields"),
  authRememberToggle: document.getElementById("authRememberToggle"),
  loginAccountInput: document.getElementById("loginAccountInput"),
  loginPasswordInput: document.getElementById("loginPasswordInput"),
  rememberPasswordInput: document.getElementById("rememberPasswordInput"),
};

const maxRenderedLogs = 300;
let activeCaptchaURL = "";
let captchaDismissed = false;
let appBootstrapped = false;
let latestConfigView = {};
let latestState = null;
let lastScanHintToastKey = "";
let monitorPauseRequested = false;
let pendingGameUpdateReason = "";
let updateDialogState = null;
let authSheetMode = "existing";
let authSheetContext = {
  baselineAccount: "",
  currentLoggedIn: false,
  hasStoredPassword: false,
  rememberPassword: false,
};
const seenLogKeys = new Set();
const autofillGuardNames = Object.freeze({
  account: "ctl_contact_ref",
  password: "ctl_access_phrase",
});
const windowScanHintCodes = new Set([
  "backend.hint.qr_expand_manual",
  "backend.hint.qr_refresh_manual",
  "backend.hint.qr_panel_unrecognized",
  "backend.hint.qr_visible_but_unreadable",
]);

function resolveEmbeddedCaptchaURL(rawURL) {
  const normalized = String(rawURL || "").trim();
  if (!normalized) {
    return "about:blank";
  }
  try {
    const parsed = new URL(normalized);
    if (parsed.pathname === "/") {
      parsed.pathname = "/geetest";
    }
    return parsed.toString();
  } catch (_) {
    return normalized.replace("/?", "/geetest?");
  }
}

function installAutofillGuard(input, nameHint, { inputMode = "" } = {}) {
  if (!input) {
    return;
  }

  input.setAttribute("autocomplete", input.type === "password" ? "new-password" : "off");
  input.setAttribute("autocapitalize", "off");
  input.setAttribute("autocorrect", "off");
  input.setAttribute("spellcheck", "false");
  input.setAttribute("aria-autocomplete", "none");
  input.setAttribute("data-autofill-guard", "true");
  input.setAttribute("data-form-type", "other");
  input.setAttribute("data-lpignore", "true");
  input.setAttribute("data-1p-ignore", "true");
  if (inputMode) {
    input.setAttribute("inputmode", inputMode);
  }

  input.removeAttribute("name");
  input.readOnly = true;

  let nameTimer = 0;

  const unlock = () => {
    input.readOnly = false;
    if (nameTimer) {
      window.clearTimeout(nameTimer);
    }
    nameTimer = window.setTimeout(() => {
      input.setAttribute("name", nameHint);
    }, 120);
  };

  const relock = () => {
    if (document.activeElement === input) {
      return;
    }
    if (nameTimer) {
      window.clearTimeout(nameTimer);
      nameTimer = 0;
    }
    input.removeAttribute("name");
    input.readOnly = true;
  };

  input.addEventListener("pointerdown", unlock);
  input.addEventListener("focus", unlock);
  input.addEventListener("keydown", unlock);
  input.addEventListener("blur", () => {
    window.setTimeout(relock, 0);
  });
}

installAutofillGuard(elements.loginAccountInput, autofillGuardNames.account);
installAutofillGuard(elements.loginPasswordInput, autofillGuardNames.password);

const secretFieldMeta = {
  password: {
    input: elements.loginPasswordInput,
    hasKey: "has_password",
    maskKey: "masked_password",
    defaultPlaceholder: elements.loginPasswordInput.getAttribute("placeholder") || "",
  },
};

const secretFieldDirty = {
  password: false,
};

function asText(value, fallback = "none") {
  if (value === undefined || value === null || value === "") {
    return fallback;
  }
  return String(value);
}

function maskSecret(value) {
  const text = String(value ?? "");
  if (!text) {
    return "";
  }
  if (text.length <= 4) {
    return "*".repeat(text.length);
  }
  if (text.length <= 8) {
    return `${text.slice(0, 1)}${"*".repeat(text.length - 2)}${text.slice(-1)}`;
  }
  return `${text.slice(0, 2)}${"*".repeat(text.length - 4)}${text.slice(-2)}`;
}

function sanitizeText(text) {
  let output = String(text ?? "");
  const replacements = [
    [/(\"password\"\s*:\s*\")([^\"]*)(\")/gi, "$1***$3"],
    [/(\"access_key\"\s*:\s*\")([^\"]*)(\")/gi, "$1***$3"],
    [/(\bpassword=)([^&\s]+)/gi, "$1***"],
    [/(\baccess_key=)([^&\s]+)/gi, "$1***"],
  ];

  replacements.forEach(([pattern, replacement]) => {
    output = output.replace(pattern, replacement);
  });
  return output;
}

function compactLogText(text) {
  return sanitizeText(text).replace(/\r?\n+/g, "\\n");
}

function sanitizeDisplayValue(value, key = "") {
  const normalizedKey = String(key).toLowerCase();

  if (largeBlobKeys.has(normalizedKey) && value && typeof value === "object") {
    return `[redacted ${normalizedKey}]`;
  }
  if (Array.isArray(value)) {
    return value.map((entry) => sanitizeDisplayValue(entry));
  }
  if (value && typeof value === "object") {
    return Object.fromEntries(
      Object.entries(value).map(([entryKey, entryValue]) => [
        entryKey,
        sanitizeDisplayValue(entryValue, entryKey),
      ]),
    );
  }
  if (sensitiveKeys.has(normalizedKey)) {
    return maskSecret(value);
  }
  if (
    typeof value === "string" &&
    (largeBlobKeys.has(normalizedKey) || (normalizedKey === "data" && value.length > 512))
  ) {
    return `[redacted ${value.length} chars]`;
  }
  if (typeof value === "string") {
    return sanitizeText(value);
  }
  return value;
}

function serializePayload(payload, compact = false) {
  if (typeof payload === "string") {
    return compact ? compactLogText(payload) : sanitizeText(payload);
  }

  try {
    const encoded = JSON.stringify(sanitizeDisplayValue(payload), null, compact ? 0 : 2);
    if (typeof encoded === "string") {
      return encoded;
    }
  } catch (_) {
  }

  const fallback = String(payload ?? "");
  return compact ? compactLogText(fallback) : sanitizeText(fallback);
}

function showPayload(payload, tone = "neutral") {
  const rendered = serializePayload(payload, false);
  elements.responseBox.dataset.tone = tone;
  elements.responseBox.textContent = rendered;

  if (tone === "error" || tone === "warn" || tone === "neutral") {
    const maxLogLength = 1200;
    const compactRendered = serializePayload(payload, true);
    const compact =
      compactRendered.length > maxLogLength
        ? `${compactRendered.slice(0, maxLogLength)} ...[truncated ${compactRendered.length - maxLogLength} chars]`
        : compactRendered;
    void RecordClientMessage(`[ui/${tone}] ${compact}`);
  }
}

function logEntryKey(entry) {
  const at = String(entry?.at ?? "").trim();
  const message = String(entry?.message ?? "").trim();
  return `${at}|${message}`;
}

function appendLog(entry) {
  const key = logEntryKey(entry);
  if (!key || seenLogKeys.has(key)) {
    return;
  }
  seenLogKeys.add(key);

  const row = document.createElement("article");
  row.className = "log-entry";
  row.innerHTML = `
    <time>${new Date(entry.at).toLocaleTimeString()}</time>
    <p>${sanitizeText(entry.message)}</p>
  `;
  elements.logList.prepend(row);
  while (elements.logList.childElementCount > maxRenderedLogs) {
    elements.logList.lastElementChild.remove();
  }
}

function renderLogSnapshot(entries = []) {
  elements.logList.innerHTML = "";
  seenLogKeys.clear();
  entries.forEach((entry) => appendLog(entry));
}

function syncInputValue(input, value) {
  if (document.activeElement !== input) {
    input.value = value ?? "";
  }
}

function markSecretFieldDirty(key) {
  if (!secretFieldDirty[key]) {
    secretFieldDirty[key] = true;
  }
}

function markSecretFieldClean(key) {
  secretFieldDirty[key] = false;
}

function syncSecretInput(key, cfg) {
  const meta = secretFieldMeta[key];
  if (!meta) {
    return;
  }
  if (key === "password") {
    syncAuthPasswordPlaceholder(cfg);
    return;
  }
  const { input, hasKey, maskKey, defaultPlaceholder } = meta;
  const hasValue = Boolean(cfg?.[hasKey]);
  const maskedValue = String(cfg?.[maskKey] ?? "").trim();

  input.placeholder = maskedValue || defaultPlaceholder;
  input.dataset.configured = hasValue ? "true" : "false";

  if (secretFieldDirty[key] || document.activeElement === input) {
    return;
  }
  input.value = "";
}

function syncAuthPasswordPlaceholder(cfg = latestConfigView) {
  const meta = secretFieldMeta.password;
  const currentAccount = String(cfg?.account ?? "").trim();
  const draftAccount = String(elements.loginAccountInput.value ?? "").trim();
  const sameAccount =
    currentAccount !== "" &&
    draftAccount !== "" &&
    draftAccount.toLowerCase() === currentAccount.toLowerCase();
  const canShowStoredPassword = authSheetMode !== "new" && sameAccount && Boolean(cfg?.[meta.hasKey]);

  elements.loginPasswordInput.placeholder = canShowStoredPassword
    ? String(cfg?.[meta.maskKey] ?? "").trim() || meta.defaultPlaceholder
    : meta.defaultPlaceholder;
  elements.loginPasswordInput.dataset.configured = canShowStoredPassword ? "true" : "false";

  if (secretFieldDirty.password || document.activeElement === elements.loginPasswordInput) {
    return;
  }
  elements.loginPasswordInput.value = "";
}

function syncRememberPasswordToggle(checked = false) {
  if (!elements.rememberPasswordInput) {
    return;
  }
  elements.rememberPasswordInput.checked = Boolean(checked);
}

function refreshAuthSubmitState(cfg = latestConfigView) {
  if (authSheetMode === "captcha") {
    elements.authCancelBtn.parentElement.dataset.mode = "captcha";
    elements.authSubmitBtn.disabled = !(latestState?.captchaPending && activeCaptchaURL);
    elements.authSubmitBtn.textContent = t("auth.continueCaptcha");
    elements.authReloadBtn.hidden = false;
    elements.authReloadBtn.disabled = false;
    elements.authReloadBtn.textContent = t("auth.reloadCaptcha");
    elements.authCancelBtn.textContent = t("auth.cancelCaptcha");
    return;
  }

  elements.authReloadBtn.hidden = true;
  elements.authReloadBtn.disabled = true;
  elements.authCancelBtn.parentElement.dataset.mode = "default";

  const draftAccount = String(elements.loginAccountInput.value ?? "").trim();
  const draftPassword = String(elements.loginPasswordInput.value ?? "").trim();
  const baselineAccount = String(authSheetContext.baselineAccount ?? "").trim();
  const sameAccount =
    baselineAccount !== "" &&
    draftAccount !== "" &&
    draftAccount.toLowerCase() === baselineAccount.toLowerCase();
  const accountChanged = draftAccount !== baselineAccount;
  const passwordChanged = draftPassword !== "";
  const canReuseStoredPassword = sameAccount && authSheetContext.hasStoredPassword;

  if (authSheetMode === "existing" && authSheetContext.currentLoggedIn && !accountChanged && !passwordChanged) {
    elements.authSubmitBtn.disabled = true;
    elements.authSubmitBtn.textContent = t("auth.status.loggedIn");
    return;
  }

  const canSubmit = draftAccount !== "" && (passwordChanged || canReuseStoredPassword);
  elements.authSubmitBtn.disabled = !canSubmit;
  elements.authSubmitBtn.textContent = t("common.login");
  elements.authCancelBtn.textContent = t("common.close");
}

function syncRangeValue(input, value) {
  if (document.activeElement !== input) {
    input.value = String(value);
  }
}

function sliderToOpacityPercent(value) {
  const normalized = Math.max(0, Math.min(100, Number(value || 0)));
  return Math.round(20 + normalized * 0.8);
}

function opacityPercentToSlider(percent) {
  const normalized = Math.max(20, Math.min(100, Number(percent || 20)));
  return Math.round((normalized - 20) / 0.8);
}

function refreshDraftActionState(cfg = latestConfigView) {
  void cfg;
}

function hasCachedSessionState(cfg) {
  return Boolean((cfg.account_login ?? cfg.accountLogin) || (cfg.last_login_succ && cfg.has_access_key));
}

function describeAccountStatus(state, cfg) {
  const account = String(cfg?.account ?? "").trim();
  if (!account) {
    return {
      text: t("auth.status.none"),
      tone: "neutral",
      mode: "new",
    };
  }

  if (state?.captchaPending) {
    return {
      text: t("auth.status.current", { status: t("auth.status.captcha") }),
      tone: "warn",
      mode: "existing",
    };
  }

  if (Boolean(cfg?.account_login ?? cfg?.accountLogin)) {
    return {
      text: t("auth.status.current", { status: t("auth.status.loggedIn") }),
      tone: "ok",
      mode: "existing",
    };
  }

  if (Boolean(cfg?.has_access_key)) {
    return {
      text: t("auth.status.current", { status: t("auth.status.cached") }),
      tone: "warn",
      mode: "existing",
    };
  }

  if (Boolean(cfg?.has_password)) {
    return {
      text: t("auth.status.current", { status: t("auth.status.relogin") }),
      tone: "error",
      mode: "existing",
    };
  }

  return {
    text: t("auth.status.current", { status: t("auth.status.passwordRequired") }),
    tone: "error",
    mode: "existing",
  };
}

function applyAccountStatus(state, cfg) {
  const status = describeAccountStatus(state, cfg);
  elements.accountStatusBtn.textContent = status.text;
  elements.accountStatusBtn.dataset.tone = status.tone;
  elements.accountStatusBtn.dataset.mode = status.mode;
}

async function syncMonitorPauseState() {
  const shouldPause = !elements.authOverlay.hidden || !elements.captchaOverlay.hidden;
  if (shouldPause === monitorPauseRequested) {
    return;
  }
  monitorPauseRequested = shouldPause;
  try {
    if (shouldPause) {
      await PauseMonitor();
    } else {
      await ResumeMonitor();
    }
  } catch (_) {
  }
}

function formatLocaleLabel(locale) {
  const key = `locale.name.${locale}`;
  const translated = t(key);
  if (translated !== key) {
    return translated;
  }
  return locale;
}

function normalizeLoaderAPIAddress(value) {
  const raw = String(value ?? "").trim();
  if (!raw) {
    return "";
  }
  if (/^https:\/\//i.test(raw)) {
    return raw;
  }
  if (/^http:\/\//i.test(raw)) {
    return `https://${raw.replace(/^http:\/\//i, "")}`;
  }
  if (/^[a-z][a-z\d+\-.]*:\/\//i.test(raw)) {
    return raw;
  }
  if (raw.startsWith("//")) {
    return `https:${raw}`;
  }
  return `https://${raw}`;
}

function setLocaleTriggerLabel(locale) {
  elements.localeValue.textContent = formatLocaleLabel(locale);
  elements.localeTrigger.dataset.locale = locale;
}

function setAccountTriggerLabel(cfg = latestConfigView) {
  const accounts = Array.isArray(cfg?.saved_accounts) ? cfg.saved_accounts : [];
  const currentAccount = String(cfg?.account ?? "").trim();
  const selected = accounts.find((entry) => String(entry?.account ?? "").trim() === currentAccount);
  elements.accountValue.textContent = selected?.display_name || currentAccount || t("settings.noSavedAccounts");
}

function closeLocaleMenu() {
  elements.localeMenu.hidden = true;
  elements.localeTrigger.setAttribute("aria-expanded", "false");
  elements.localePicker.dataset.open = "false";
}

function openLocaleMenu() {
  elements.localeMenu.hidden = false;
  elements.localeTrigger.setAttribute("aria-expanded", "true");
  elements.localePicker.dataset.open = "true";
}

function closeAccountMenu() {
  elements.accountMenu.hidden = true;
  elements.accountTrigger.setAttribute("aria-expanded", "false");
  elements.accountPicker.dataset.open = "false";
}

function openAccountMenu() {
  elements.accountMenu.hidden = false;
  elements.accountTrigger.setAttribute("aria-expanded", "true");
  elements.accountPicker.dataset.open = "true";
}

function openAuthSheet(mode = "existing") {
  const cfg = latestConfigView || {};
  const currentAccount = String(cfg.account ?? "").trim();
  authSheetMode = mode;
  authSheetContext = {
    baselineAccount: mode === "new" ? "" : currentAccount,
    currentLoggedIn: mode === "existing" && Boolean(cfg.account_login ?? cfg.accountLogin),
    hasStoredPassword: mode === "existing" && Boolean(cfg.has_password),
    rememberPassword: mode === "existing" && Boolean(cfg.remember_password ?? cfg.rememberPassword),
  };
  const captchaMode = mode === "captcha";
  elements.authTitle.textContent = captchaMode ? t("auth.titleCaptcha") : mode === "new" ? t("auth.titleAdd") : t("auth.title");
  elements.authCopy.textContent = captchaMode
    ? (latestState?.lastAction === "captcha_expired" ? t("auth.copyCaptchaExpired") : t("auth.copyCaptcha"))
    : mode === "new"
      ? t("auth.copyAdd")
      : t("auth.copy");
  elements.loginAccountInput.value = mode === "new" ? "" : currentAccount;
  elements.loginPasswordInput.value = "";
  syncRememberPasswordToggle(authSheetContext.rememberPassword);
  markSecretFieldClean("password");
  syncAuthPasswordPlaceholder(cfg);
  elements.authFields.hidden = captchaMode;
  elements.authRememberToggle.hidden = captchaMode;
  refreshAuthSubmitState(cfg);
  elements.authOverlay.hidden = false;
  window.setTimeout(() => {
    if (captchaMode) {
      elements.authSubmitBtn.focus();
      return;
    }
    if (mode === "new" || !currentAccount) {
      elements.loginAccountInput.focus();
      return;
    }
    elements.loginPasswordInput.focus();
  }, 0);
  void syncMonitorPauseState();
}

function closeAuthSheet() {
  elements.authOverlay.hidden = true;
  authSheetMode = "existing";
  authSheetContext = {
    baselineAccount: "",
    currentLoggedIn: false,
    hasStoredPassword: false,
    rememberPassword: false,
  };
  elements.loginPasswordInput.value = "";
  syncRememberPasswordToggle(false);
  markSecretFieldClean("password");
  elements.authFields.hidden = false;
  elements.authRememberToggle.hidden = false;
  elements.authReloadBtn.hidden = true;
  elements.authReloadBtn.disabled = true;
  syncAuthPasswordPlaceholder(latestConfigView);
  refreshAuthSubmitState(latestConfigView);
  void syncMonitorPauseState();
}

function populateLocaleOptions() {
  const locales = listLocales();
  const currentLocale = getLocale();
  elements.localeMenu.innerHTML = "";
  setLocaleTriggerLabel(currentLocale);

  locales.forEach((locale) => {
    const option = document.createElement("button");
    option.type = "button";
    option.className = "locale-option";
    option.dataset.locale = locale;
    option.setAttribute("role", "option");
    option.setAttribute("aria-selected", locale === currentLocale ? "true" : "false");
    option.textContent = formatLocaleLabel(locale);
    option.addEventListener("click", async () => {
      closeLocaleMenu();
      if (!locale || locale === getLocale()) {
        return;
      }
      const persisted = await persistSettings({ showToast: false, closeSheet: false });
      if (!persisted) {
        return;
      }
      markSkipUpdatePromptOnce();
      applyLocale(locale);
    });
    elements.localeMenu.append(option);
  });
}

function populateAccountOptions(cfg = latestConfigView) {
  const accounts = Array.isArray(cfg?.saved_accounts) ? cfg.saved_accounts : [];
  const currentAccount = String(cfg?.account ?? "").trim();
  elements.accountMenu.innerHTML = "";
  setAccountTriggerLabel(cfg);
  elements.accountTrigger.disabled = accounts.length === 0;
  elements.editAccountBtn.disabled = !currentAccount;

  if (!accounts.length) {
    closeAccountMenu();
    return;
  }

  accounts.forEach((entry) => {
    const account = String(entry?.account ?? "").trim();
    if (!account) {
      return;
    }
    const option = document.createElement("button");
    option.type = "button";
    option.className = "locale-option";
    option.dataset.account = account;
    option.setAttribute("role", "option");
    option.setAttribute("aria-selected", account === currentAccount ? "true" : "false");
    option.textContent = String(entry?.display_name ?? account);
    option.addEventListener("click", async () => {
      closeAccountMenu();
      if (account === currentAccount) {
        return;
      }
      await PauseMonitor().catch(() => {});
      const state = await runTask(() => SelectSavedAccount(account), null, "soft");
      await ResumeMonitor().catch(() => {});
      if (!state) {
        return;
      }
      renderState(state);
      showPayload({ account: entry?.display_name ?? account }, "neutral");
    });
    elements.accountMenu.append(option);
  });
}

async function refreshBackground(forceURL = "") {
  const dataURL = forceURL || (await BackgroundDataURL());
  if (!dataURL) {
    elements.customBackground.hidden = true;
    elements.customBackground.style.backgroundImage = "";
    return;
  }

  elements.customBackground.hidden = false;
  elements.customBackground.style.opacity = "1";
  elements.customBackground.style.backgroundImage = `url("${dataURL}")`;
}

function applySurfaceOpacity(percent) {
  const clampedPercent = Math.max(20, Math.min(100, Number(percent || 20)));
  const alpha = clampedPercent / 100;
  const root = document.documentElement;
  root.style.setProperty("--panel-alpha", alpha.toFixed(3));
  root.style.setProperty("--panel-strong-alpha", Math.min(1, alpha).toFixed(3));
  root.style.setProperty("--card-alpha", Math.max(0, alpha * 0.78).toFixed(3));
  root.style.setProperty("--input-alpha", Math.max(0, alpha * 0.9).toFixed(3));
  root.style.setProperty("--readonly-alpha", Math.max(0, alpha * 0.95).toFixed(3));
  root.style.setProperty("--soft-alpha", Math.max(0, alpha * 0.08).toFixed(3));
  root.style.setProperty("--response-alpha", Math.max(0, alpha * 0.92).toFixed(3));
}

function previewOpacity(sliderValue) {
  const normalized = Math.max(0, Math.min(100, Number(sliderValue || 0)));
  const opacityPercent = sliderToOpacityPercent(normalized);
  elements.backgroundOpacityInput.value = String(normalized);
  elements.backgroundOpacityValue.textContent = `${opacityPercent}%`;
  applySurfaceOpacity(opacityPercent);
}

function applyBlurEnabled(enabled) {
  const root = document.documentElement;
  root.style.setProperty("--panel-blur", enabled ? "16px" : "0px");
  root.style.setProperty("--overlay-blur", enabled ? "8px" : "0px");
  root.style.setProperty("--overlay-strong-blur", enabled ? "10px" : "0px");
}

function setStatus(dot, valueNode, tone, text) {
  dot.dataset.tone = tone;
  valueNode.textContent = text;
}

const actionTextMap = {
  monitoring: "actionState.monitoring",
  runtime_starting: "topbar.starting",
  stopped: "actionState.stopped",
  scan_complete: "actionState.scan_complete",
  launch_game: "actionState.launch_game",
  scan: "actionState.scan",
  quit_requested: "actionState.quit_requested",
  waiting_login: "actionState.waiting_login",
  waiting_window: "actionState.waiting_window",
  ticket_detected: "actionState.ticket_detected",
  login: "actionState.login",
  captcha_required: "actionState.captcha_required",
  captcha_expired: "actionState.captcha_expired",
};

function formatActionValue(value) {
  const key = String(value ?? "").trim().toLowerCase();
  if (!key || key === "none") {
    return t("action.idle");
  }
  if (actionTextMap[key]) {
    return t(actionTextMap[key]);
  }
  return sanitizeText(key.replace(/_/g, " "));
}

function formatErrorValue(value, messageRef = null) {
  const translated = translateMessage(messageRef, "");
  if (translated) {
    return sanitizeText(translated);
  }
  const text = String(value ?? "").trim();
  if (!text || text.toLowerCase() === "none") {
    return t("common.none");
  }
  return sanitizeText(text);
}

function formatAPIStatus(state, cfg) {
  if (state.runtimePreparing) {
    return { tone: "warn", text: t("topbar.apiPending") };
  }
  const apiAddress = String(cfg.loader_api_base_url ?? cfg.loaderApiBaseURL ?? "").trim();
  if (!apiAddress) {
    return { tone: "error", text: t("topbar.notSet") };
  }
  return state.apiReady
    ? { tone: "ok", text: t("status.connected") }
    : { tone: "error", text: t("status.unavailable") };
}

function formatWindowStatus(state) {
  if (state.runtimePreparing) {
    return { tone: "warn", text: t("topbar.starting") };
  }
  return state.running
    ? { tone: "ok", text: t("status.monitoring") }
    : { tone: "error", text: t("status.idle") };
}

function formatSessionStatus(state, cfg) {
  const hasCachedSession = hasCachedSessionState(cfg);
  const account = String(cfg.account ?? "").trim();
  const hasPassword = Boolean(cfg.has_password);
  const hasCredentials = Boolean(account && hasPassword);

  if (state.captchaPending) {
    return { tone: "warn", text: t("status.verificationRequired") };
  }
  if (hasCachedSession) {
    return { tone: "ok", text: t("status.cached") };
  }
  if (!hasCredentials) {
    return { tone: "error", text: t("status.notLoggedIn") };
  }
  return { tone: "error", text: t("status.waitingLogin") };
}

function openSettings() {
  elements.settingsBackdrop.hidden = false;
  elements.settingsSheet.hidden = false;
}

function closeSettings() {
  elements.settingsBackdrop.hidden = true;
  elements.settingsSheet.hidden = true;
  closeLocaleMenu();
  closeAccountMenu();
}

function queueGameUpdatePrompt(reason) {
  const normalized = String(reason || "").trim();
  if (!normalized) {
    return;
  }
  if (normalized === "startup" && consumeSkipUpdatePromptOnce()) {
    return;
  }
  pendingGameUpdateReason = normalized;
  void flushQueuedGameUpdatePrompt();
}

async function fetchGameUpdatePrompt({ showError = false } = {}) {
  try {
    return await CheckGameUpdate();
  } catch (error) {
    if (showError) {
      showPayload(formatError(error), "warn");
    }
    return null;
  }
}

async function flushQueuedGameUpdatePrompt() {
  if (!pendingGameUpdateReason || updateDialogState || !latestState) {
    return;
  }
  if (!latestState.gamePathValid || latestState.runtimePreparing || !latestState.apiReady) {
    return;
  }

  const reason = pendingGameUpdateReason;
  pendingGameUpdateReason = "";

  const prompt = await fetchGameUpdatePrompt();
  if (!prompt?.outdated) {
    return;
  }
  openUpdateDialog(prompt, { reason, manualOnly: !prompt.launcherAvailable });
}

function openUpdateDialog(prompt, { reason = "", manualOnly = false } = {}) {
  updateDialogState = {
    prompt,
    reason: String(reason || "").trim(),
    manualOnly: Boolean(manualOnly),
  };

  elements.updateTitle.textContent = t("update.title");
  elements.updateCopy.textContent = t("update.summary", {
    local: prompt?.localVersion || t("common.unknown"),
    remote: prompt?.remoteVersion || t("common.unknown"),
  });
  elements.updateLocalValue.textContent = prompt?.localVersion || t("common.unknown");
  elements.updateRemoteValue.textContent = prompt?.remoteVersion || t("common.unknown");
  elements.updateNote.textContent = manualOnly
    ? t("update.manualMessage")
    : t("update.promptMessage");
  elements.updateActions.innerHTML = "";

  if (manualOnly) {
    elements.updateActions.dataset.layout = "single";
    const okButton = document.createElement("button");
    okButton.type = "button";
    okButton.className = "button button-accent";
    okButton.textContent = updateAckText();
    okButton.addEventListener("click", () => {
      closeUpdateDialog();
    });
    elements.updateActions.append(okButton);
  } else {
    elements.updateActions.dataset.layout = "double";
    const laterButton = document.createElement("button");
    laterButton.type = "button";
    laterButton.className = "button button-solid";
    laterButton.textContent = t("update.later");
    laterButton.addEventListener("click", () => {
      transitionUpdateDialogToManual();
    });

    const launchButton = document.createElement("button");
    launchButton.type = "button";
    launchButton.className = "button button-accent";
    launchButton.textContent = t("update.openLauncher");
    launchButton.addEventListener("click", async () => {
      try {
        await LaunchUpdater();
        showPayload(t("update.launcherOpened"), "soft");
      } catch (error) {
        showPayload(formatError(error), "error");
        transitionUpdateDialogToManual();
        return;
      }
      closeUpdateDialog();
    });

    elements.updateActions.append(laterButton, launchButton);
  }

  elements.updateOverlay.hidden = false;
}

function closeUpdateDialog() {
  updateDialogState = null;
  elements.updateOverlay.hidden = true;
  void flushQueuedGameUpdatePrompt();
}

function transitionUpdateDialogToManual() {
  if (!updateDialogState) {
    return;
  }
  openUpdateDialog(updateDialogState.prompt, {
    reason: updateDialogState.reason,
    manualOnly: true,
  });
}

function syncCaptcha(state) {
  const pending = Boolean(state.captchaPending && state.captchaURL);
  const url = pending ? String(state.captchaURL) : "";

  if (pending) {
    elements.captchaCopy.textContent = t("captcha.copy");
    if (activeCaptchaURL !== url) {
      activeCaptchaURL = url;
      captchaDismissed = false;
      elements.captchaFrame.src = resolveEmbeddedCaptchaURL(url);
    }
    elements.captchaOverlay.hidden = captchaDismissed;
    void syncMonitorPauseState();
    return;
  }

  captchaDismissed = false;
  elements.captchaOverlay.hidden = true;
  if (activeCaptchaURL) {
    activeCaptchaURL = "";
    elements.captchaFrame.src = "about:blank";
  }
  void syncMonitorPauseState();
}

function renderState(state) {
  latestState = state || null;
  const cfg = state.config || {};
  latestConfigView = cfg;
  const autoWindowCapture = Boolean(cfg.auto_window_capture ?? cfg.autoWindowCapture);
  const autoClose = Boolean(cfg.auto_close ?? cfg.autoClose);
  const gamePath = cfg.game_path ?? cfg.gamePath ?? "";
  const launcherPath = cfg.launcher_path ?? cfg.launcherPath ?? "";
  const hasBackgroundImage = Boolean(cfg.has_background_image ?? cfg.hasBackgroundImage);
  const backgroundOpacity = Math.round(
    Math.max(20, Math.min(100, Number(cfg.background_opacity ?? cfg.backgroundOpacity ?? 0.35) * 100)),
  );
  const panelBlur = cfg.panel_blur ?? cfg.panelBlur ?? true;
  const build = state.buildInfo || state.build_info || {};
  const windowStatus = formatWindowStatus(state);
  const session = formatSessionStatus(state, cfg);
  const api = formatAPIStatus(state, cfg);
  const gameTone = state.gamePathValid ? "ok" : "warn";
  const gameText = state.gamePathValid
    ? t("common.configured")
    : gamePath
      ? t("status.gameNeedsFix")
      : t("status.gameOptional");

  setStatus(elements.appDot, elements.appValue, appBootstrapped ? windowStatus.tone : "error", windowStatus.text);
  setStatus(elements.sessionDot, elements.sessionValue, session.tone, session.text);
  setStatus(elements.dispatchDot, elements.dispatchValue, api.tone, api.text);
  setStatus(elements.gameDot, elements.gameValue, gameTone, gameText);

  elements.actionValue.textContent = formatActionValue(state.lastAction);
  elements.errorValue.textContent = formatErrorValue(state.lastError, state.lastErrorMessage);

  const hintCode = String(state?.lastErrorMessage?.code ?? "").trim();
  if (windowScanHintCodes.has(hintCode) && elements.authOverlay.hidden && elements.captchaOverlay.hidden) {
    const hintText = translateMessage(state.lastErrorMessage, state.lastError || "");
    const toastKey = `${hintCode}|${hintText}`;
    if (hintText && toastKey !== lastScanHintToastKey) {
      lastScanHintToastKey = toastKey;
      showPayload(hintText, "warn");
    }
  } else {
    lastScanHintToastKey = "";
  }

  syncSecretInput("password", cfg);
  syncInputValue(elements.asteriskNameInput, cfg.asterisk_name ?? cfg.asteriskName ?? "");
  syncInputValue(elements.loaderApiInput, normalizeLoaderAPIAddress(cfg.loader_api_base_url ?? cfg.loaderApiBaseURL ?? ""));
  syncInputValue(elements.gamePathInput, gamePath);
  syncInputValue(elements.launcherPathInput, launcherPath);
  elements.buildVersionValue.textContent = build.version || "-";
  elements.buildDateValue.textContent = build.buildDate || build.build_date || "-";
  elements.buildDeveloperValue.textContent = build.developer || "-";
  syncRangeValue(elements.backgroundOpacityInput, opacityPercentToSlider(backgroundOpacity));
  previewOpacity(opacityPercentToSlider(backgroundOpacity));
  elements.backgroundStatusValue.textContent = hasBackgroundImage ? t("settings.backgroundSet") : t("settings.backgroundUnset");
  elements.backgroundStatusValue.dataset.tone = hasBackgroundImage ? "ok" : "warn";
  elements.resetBackgroundBtn.disabled = !hasBackgroundImage;
  const blurEnabled = Boolean(panelBlur);
  if (elements.settingsSheet.hidden) {
    elements.panelBlurInput.checked = blurEnabled;
  }
  applyBlurEnabled(elements.panelBlurInput.checked);
  if (!hasBackgroundImage) {
    elements.customBackground.hidden = true;
    elements.customBackground.style.backgroundImage = "";
  }
  if (elements.settingsSheet.hidden) {
    elements.autoWindowCaptureInput.checked = autoWindowCapture;
    elements.autoCloseInput.checked = autoClose;
  }
  elements.launchGameBtn.disabled = !state.gamePathValid;
  elements.clearGamePathBtn.disabled = !gamePath;
  elements.clearLauncherPathBtn.disabled = !launcherPath;
  elements.clearAccountBtn.disabled = !(cfg.saved_accounts ?? cfg.savedAccounts ?? []).length;
  refreshDraftActionState(cfg);
  populateAccountOptions(cfg);
  applyAccountStatus(state, cfg);
  setLocaleTriggerLabel(getLocale());

  syncCaptcha(state);
  void flushQueuedGameUpdatePrompt();
}

function formatError(error) {
  if (typeof error === "string") {
    return sanitizeText(error);
  }
  if (error?.message) {
    return sanitizeText(error.message);
  }
  return sanitizeText(JSON.stringify(error, null, 2));
}

async function persistSettings({ showToast = true, closeSheet = false } = {}) {
  const currentCfg = latestConfigView || {};
  const currentNickname = String(currentCfg.asterisk_name ?? currentCfg.asteriskName ?? "").trim();
  const nextNickname = String(elements.asteriskNameInput.value ?? "").trim();
  const currentLoaderAPI = normalizeLoaderAPIAddress(currentCfg.loader_api_base_url ?? currentCfg.loaderApiBaseURL ?? "");
  const nextLoaderAPI = normalizeLoaderAPIAddress(elements.loaderApiInput.value ?? "");
  const needsCredentialSave = nextNickname !== currentNickname || nextLoaderAPI !== currentLoaderAPI;

  if (String(elements.loaderApiInput.value ?? "").trim() !== nextLoaderAPI) {
    elements.loaderApiInput.value = nextLoaderAPI;
  }

  const nextAutoClose = elements.autoCloseInput.checked;
  const nextAutoWindowCapture = elements.autoWindowCaptureInput.checked;
  const nextPanelBlur = elements.panelBlurInput.checked;
  const nextGamePath = elements.gamePathInput.value || "";
  const currentAutoClose = Boolean(currentCfg.auto_close ?? currentCfg.autoClose);
  const currentAutoWindowCapture = Boolean(currentCfg.auto_window_capture ?? currentCfg.autoWindowCapture);
  const currentPanelBlur = Boolean(currentCfg.panel_blur ?? currentCfg.panelBlur ?? true);
  const currentGamePath = String(currentCfg.game_path ?? currentCfg.gamePath ?? "");
  const currentLauncherPath = String(currentCfg.launcher_path ?? currentCfg.launcherPath ?? "");
  const nextLauncherPath = String(elements.launcherPathInput.value ?? "").trim();
  const gamePathChanged = nextGamePath !== currentGamePath;
  const launcherPathChanged = nextLauncherPath !== currentLauncherPath;
  const nextOpacity = sliderToOpacityPercent(elements.backgroundOpacityInput.value) / 100;
  const currentOpacity = Number(currentCfg.background_opacity ?? currentCfg.backgroundOpacity ?? 0.35);
  const needsFeatureSave =
    nextGamePath !== currentGamePath ||
    nextAutoClose !== currentAutoClose ||
    nextAutoWindowCapture !== currentAutoWindowCapture ||
    nextPanelBlur !== currentPanelBlur ||
    Math.abs(nextOpacity - currentOpacity) > 0.0001;
  const needsLauncherSave = launcherPathChanged;

  if (!needsCredentialSave && !needsFeatureSave && !needsLauncherSave) {
    if (closeSheet) {
      closeSettings();
    }
    return true;
  }

  let state = null;
  if (needsCredentialSave) {
    const savedState = await runTask(
      () => SaveCredentialSettings(nextNickname, nextLoaderAPI),
      null,
      "soft",
    );
    if (!savedState) {
      return false;
    }
    state = savedState;
  }

  if (needsFeatureSave) {
    state = await runTask(
      () => SaveFeatureSettings(nextGamePath, nextAutoClose, nextAutoWindowCapture, nextPanelBlur, nextOpacity),
      showToast ? () => ({ saved: true, gamePath: nextGamePath || null }) : null,
    );
    if (!state) {
      return false;
    }
  }

  if (needsLauncherSave) {
    state = await runTask(
      () => SaveLauncherPath(nextLauncherPath),
      showToast ? () => ({ launcherPath: nextLauncherPath || null }) : null,
      "soft",
    );
    if (!state) {
      return false;
    }
  }

  if (state) {
    appBootstrapped = true;
    renderState(state);
    if (needsFeatureSave) {
      await refreshBackground();
    }
    if ((gamePathChanged || launcherPathChanged) && state.gamePathValid) {
      queueGameUpdatePrompt("path_saved");
    }
  }

  if (showToast) {
    const cfg = (state?.config) || latestConfigView || {};
    showPayload(
      {
        saved: true,
        nickname: cfg.asterisk_name ?? cfg.asteriskName ?? null,
        gamePath: cfg.game_path ?? cfg.gamePath ?? null,
      },
      "neutral",
    );
  }

  if (closeSheet) {
    closeSettings();
  }
  return true;
}

async function runTask(task, successPayload, tone = "ok") {
  try {
    const result = await task();
    if (successPayload) {
      showPayload(successPayload(result), tone);
    }
    return result;
  } catch (error) {
    showPayload(formatError(error), "error");
    return null;
  }
}

elements.settingsBtn.addEventListener("click", () => openSettings());
elements.settingsCloseBtn.addEventListener("click", async () => {
  await persistSettings({ showToast: false, closeSheet: true });
});
elements.settingsBackdrop.addEventListener("click", async () => {
  await persistSettings({ showToast: false, closeSheet: true });
});
elements.updateCloseBtn.addEventListener("click", () => {
  if (updateDialogState?.manualOnly) {
    closeUpdateDialog();
    return;
  }
  transitionUpdateDialogToManual();
});
elements.updateOverlay.addEventListener("click", (event) => {
  if (event.target === elements.updateOverlay) {
    event.preventDefault();
  }
});
elements.loaderApiInput.addEventListener("blur", () => {
  const normalized = normalizeLoaderAPIAddress(elements.loaderApiInput.value ?? "");
  if (normalized !== String(elements.loaderApiInput.value ?? "").trim()) {
    elements.loaderApiInput.value = normalized;
  }
});
elements.localeTrigger.addEventListener("click", () => {
  if (elements.localeMenu.hidden) {
    openLocaleMenu();
  } else {
    closeLocaleMenu();
  }
});
elements.accountTrigger.addEventListener("click", () => {
  if (elements.accountMenu.hidden) {
    openAccountMenu();
  } else {
    closeAccountMenu();
  }
});
document.addEventListener("pointerdown", (event) => {
  if (!elements.localeMenu.hidden && !elements.localePicker.contains(event.target)) {
    closeLocaleMenu();
  }
  if (!elements.accountMenu.hidden && !elements.accountPicker.contains(event.target)) {
    closeAccountMenu();
  }
});
elements.browseGamePathBtn.addEventListener("click", async () => {
  const selected = await runTask(
    () => BrowseGamePath(),
    (value) => ({ selected: value || null }),
    "soft",
  );
  if (selected === null || !selected) {
    return;
  }
  const state = await runTask(
    () =>
      SaveFeatureSettings(
        selected,
        elements.autoCloseInput.checked,
        elements.autoWindowCaptureInput.checked,
        elements.panelBlurInput.checked,
        sliderToOpacityPercent(elements.backgroundOpacityInput.value) / 100,
      ),
    () => ({ saved: true, gamePath: selected }),
    "soft",
  );
  if (state) {
    renderState(state);
    if (state.gamePathValid) {
      queueGameUpdatePrompt("path_saved");
    }
  }
});

elements.clearGamePathBtn.addEventListener("click", async () => {
  elements.gamePathInput.value = "";
  const state = await runTask(
    () =>
      SaveFeatureSettings(
        "",
        elements.autoCloseInput.checked,
        elements.autoWindowCaptureInput.checked,
        elements.panelBlurInput.checked,
        sliderToOpacityPercent(elements.backgroundOpacityInput.value) / 100,
      ),
    () => ({ gamePath: null }),
    "soft",
  );
  if (state) {
    renderState(state);
  }
});

elements.browseLauncherPathBtn.addEventListener("click", async () => {
  const selected = await runTask(
    () => BrowseLauncherPath(),
    (value) => ({ selected: value || null }),
    "soft",
  );
  if (selected === null || !selected) {
    return;
  }
  elements.launcherPathInput.value = selected;
  const state = await runTask(
    () => SaveLauncherPath(selected),
    () => ({ launcherPath: selected }),
    "soft",
  );
  if (state) {
    renderState(state);
    if (state.gamePathValid) {
      queueGameUpdatePrompt("path_saved");
    }
  }
});

elements.clearLauncherPathBtn.addEventListener("click", async () => {
  elements.launcherPathInput.value = "";
  const state = await runTask(
    () => SaveLauncherPath(""),
    () => ({ launcherPath: null }),
    "soft",
  );
  if (state) {
    renderState(state);
  }
});

elements.browseBackgroundBtn.addEventListener("click", async () => {
  const selected = await runTask(
    () => BrowseBackgroundImage(),
    (value) => ({ selected: value || null }),
    "soft",
  );
  if (selected === null || !selected) {
    return;
  }
  const state = await runTask(
    () =>
      UpdateBackground(
        selected,
        sliderToOpacityPercent(elements.backgroundOpacityInput.value) / 100,
      ),
    () => ({ background: "updated" }),
    "soft",
  );
  if (!state) {
    return;
  }
  renderState(state);
  await refreshBackground();
});

elements.backgroundOpacityInput.addEventListener("input", () => {
  previewOpacity(elements.backgroundOpacityInput.value);
});

elements.backgroundOpacityInput.addEventListener("change", async () => {
  const percent = sliderToOpacityPercent(elements.backgroundOpacityInput.value);
  const cfg = latestConfigView || {};
  const state = await runTask(
    () =>
      SaveFeatureSettings(
        String(cfg.game_path ?? cfg.gamePath ?? ""),
        Boolean(cfg.auto_close ?? cfg.autoClose),
        Boolean(cfg.auto_window_capture ?? cfg.autoWindowCapture),
        Boolean(cfg.panel_blur ?? cfg.panelBlur ?? true),
        percent / 100,
      ),
    () => ({ opacity: `${percent}%` }),
    "soft",
  );
  if (!state) {
    return;
  }
  renderState(state);
  await refreshBackground();
});

elements.panelBlurInput.addEventListener("change", () => {
  applyBlurEnabled(elements.panelBlurInput.checked);
});

elements.resetBackgroundBtn.addEventListener("click", async () => {
  const state = await runTask(() => ResetBackground(), () => ({ background: null }), "soft");
  if (!state) {
    return;
  }
  renderState(state);
  await refreshBackground("");
});

elements.saveBtn.addEventListener("click", async () => {
  await persistSettings({ showToast: true, closeSheet: false });
});

elements.accountStatusBtn.addEventListener("click", () => {
  const mode = latestState?.captchaPending ? "captcha" : elements.accountStatusBtn.dataset.mode === "new" ? "new" : "existing";
  openAuthSheet(mode);
});
elements.addAccountBtn.addEventListener("click", () => {
  openAuthSheet(latestState?.captchaPending ? "captcha" : "new");
});
elements.editAccountBtn.addEventListener("click", () => {
  openAuthSheet(latestState?.captchaPending ? "captcha" : "existing");
});
elements.clearAccountBtn.addEventListener("click", async () => {
  const state = await runTask(() => ClearCurrentAccount(), () => ({ account: null }), "soft");
  if (state) {
    renderState(state);
  }
});
elements.authCloseBtn.addEventListener("click", () => {
  closeAuthSheet();
});
elements.authCancelBtn.addEventListener("click", () => {
  if (authSheetMode === "captcha") {
    void runTask(() => CancelCaptchaLogin(), null, "soft").then((state) => {
      closeAuthSheet();
      if (state) {
        renderState(state);
      }
    });
    return;
  }
  closeAuthSheet();
});
elements.authOverlay.addEventListener("click", (event) => {
  if (event.target === elements.authOverlay) {
    closeAuthSheet();
  }
});
elements.authReloadBtn.addEventListener("click", async () => {
  const result = await runTask(
    () => ReloadCaptchaLogin(),
    (value) => value,
    "neutral",
  );
  if (!result) {
    return;
  }
  if (result?.ok) {
    markSecretFieldClean("password");
    closeAuthSheet();
    return;
  }
  if (result?.needsCaptcha) {
    const url = result.captchaURL || result.CaptchaURL || "";
    if (url) {
      activeCaptchaURL = String(url);
      captchaDismissed = false;
      elements.captchaFrame.src = resolveEmbeddedCaptchaURL(activeCaptchaURL);
      closeAuthSheet();
      elements.captchaOverlay.hidden = false;
      void syncMonitorPauseState();
    }
  }
});
elements.loginAccountInput.addEventListener("keydown", (event) => {
  if (event.key === "Enter") {
    event.preventDefault();
    elements.authSubmitBtn.click();
  }
});
elements.loginAccountInput.addEventListener("input", () => {
  const currentDraftAccount = String(elements.loginAccountInput.value ?? "").trim();
  const baselineAccount = String(authSheetContext.baselineAccount ?? "").trim();
  const accountChanged =
    baselineAccount !== "" &&
    currentDraftAccount.toLowerCase() !== baselineAccount.toLowerCase();
  if (accountChanged) {
    elements.loginPasswordInput.value = "";
    syncRememberPasswordToggle(false);
    markSecretFieldClean("password");
  }
  syncAuthPasswordPlaceholder(latestConfigView);
  refreshAuthSubmitState(latestConfigView);
});
elements.loginPasswordInput.addEventListener("keydown", (event) => {
  if (event.key === "Enter") {
    event.preventDefault();
    elements.authSubmitBtn.click();
  }
});
elements.loginPasswordInput.addEventListener("input", () => {
  markSecretFieldDirty("password");
  refreshAuthSubmitState(latestConfigView);
});
elements.authSubmitBtn.addEventListener("click", async () => {
  if (authSheetMode === "captcha") {
    closeAuthSheet();
    captchaDismissed = false;
    elements.captchaOverlay.hidden = false;
    void syncMonitorPauseState();
    return;
  }
  const result = await runTask(
    () => Login(elements.loginAccountInput.value, elements.loginPasswordInput.value, elements.rememberPasswordInput.checked),
    (value) => value,
    "neutral",
  );
  if (result?.ok) {
    markSecretFieldClean("password");
    closeAuthSheet();
  }
  if (result?.needsCaptcha) {
    closeAuthSheet();
    const url = result.captchaURL || result.CaptchaURL || "";
    if (url) {
      activeCaptchaURL = String(url);
      captchaDismissed = false;
      elements.captchaFrame.src = resolveEmbeddedCaptchaURL(activeCaptchaURL);
      elements.captchaOverlay.hidden = false;
      void syncMonitorPauseState();
    }
  }
});

elements.launchGameBtn.addEventListener("click", async () => {
  if (latestState?.runtimePreparing) {
    showPayload(t("update.pending"), "warn");
    return;
  }
  const update = await fetchGameUpdatePrompt();
  if (update?.outdated) {
    openUpdateDialog(update, { reason: "launch", manualOnly: !update.launcherAvailable });
    return;
  }
  const result = await runTask(
    () => LaunchGame(),
    () => ({ launched: true, gamePath: elements.gamePathInput.value || null }),
    "soft",
  );
  if (result === null) {
    return;
  }
});

elements.scanWindowBtn.addEventListener("click", async () => {
  const result = await runTask(() => ScanWindow(), null, "soft");
  if (result === null) {
    return;
  }
  if (result?.matched) {
    showPayload({ matched: true }, "soft");
    return;
  }

  const hint = translateMessage(result?.messageRef, result?.message || "");
  if (hint) {
    showPayload(hint, "warn");
    return;
  }
  const stateHintCode = String(latestState?.lastErrorMessage?.code ?? "").trim();
  if (windowScanHintCodes.has(stateHintCode)) {
    const stateHint = translateMessage(latestState.lastErrorMessage, latestState.lastError || "");
    if (stateHint) {
      showPayload(stateHint, "warn");
      return;
    }
  }
  showPayload(t("action.scanWindowNoMatch"), "warn");
});

elements.captchaCloseBtn.addEventListener("click", () => {
  captchaDismissed = true;
  elements.captchaOverlay.hidden = true;
  openAuthSheet("captcha");
  void syncMonitorPauseState();
});

window.addEventListener("message", (event) => {
  if (event.source !== elements.captchaFrame.contentWindow) {
    return;
  }
  const payload = event.data;
  if (!payload || payload.type !== "hi3loader-captcha-control") {
    return;
  }
  if (payload.action === "close" || payload.action === "back") {
    captchaDismissed = true;
    elements.captchaOverlay.hidden = true;
    openAuthSheet("captcha");
    void syncMonitorPauseState();
  }
});

EventsOn("state", (state) => {
  renderState(state);
});

EventsOn("quit-requested", async () => {
  await new Promise((resolve) => setTimeout(resolve, 1200));
  try {
    await ResetQuitFlag();
  } catch (_) {
  }
  Quit();
});

populateLocaleOptions();

Bootstrap()
  .then(async (state) => {
    try {
      const logs = await LogSnapshot();
      renderLogSnapshot(logs);
    } catch (error) {
      showPayload(formatError(error), "warn");
    }
    EventsOn("log", (entry) => {
      appendLog(entry);
    });

    const bootState = latestState || state;
    appBootstrapped = true;
    renderState(bootState);
    refreshBackground().catch((error) => {
      showPayload(formatError(error), "error");
    });
    if (!bootState.gamePathValid) {
      showPayload(
        translateMessage(bootState.gamePathMessage, bootState.gamePathPrompt || t("status.selectPathFirst")),
        "warn",
      );
      return;
    }
    queueGameUpdatePrompt("startup");
    showPayload(
        {
          session: formatSessionStatus(bootState, bootState.config || {}).text,
          api: formatAPIStatus(bootState, bootState.config || {}).text,
        },
        "soft",
    );
  })
  .catch((error) => {
    appBootstrapped = false;
    setStatus(elements.appDot, elements.appValue, "error", t("status.startFailed"));
    showPayload(formatError(error), "error");
    openSettings();
  });
}();
