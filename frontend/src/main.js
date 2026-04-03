import "./style.css";
import { applyLocale, getLocale, initI18n, listLocales, t, translateMessage } from "./i18n";

import {
  BackgroundDataURL,
  BrowseBackgroundImage,
  Bootstrap,
  BrowseGamePath,
  BrowseLauncherPath,
  ClearCurrentAccount,
  LaunchGame,
  LogSnapshot,
  Login,
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

        <p class="settings-note" id="pathHintValue" hidden></p>

        <div class="settings-grid">
          <label class="field settings-card settings-card-span">
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
            <span>${t("settings.locale")}</span>
            <div class="locale-picker" id="localePicker">
              <button class="locale-trigger" id="localeTrigger" type="button" aria-haspopup="listbox" aria-expanded="false" aria-label="${t("settings.locale")}">
                <span class="locale-value" id="localeValue"></span>
                <span class="locale-chevron" aria-hidden="true"></span>
              </button>
              <div class="locale-menu" id="localeMenu" role="listbox" hidden></div>
            </div>
          </label>
          <label class="field settings-card settings-card-span">
            <span>${t("settings.apiAddress")}</span>
            <input id="loaderApiInput" autocomplete="off" placeholder="${t("settings.apiAddressPlaceholder")}" />
          </label>
        </div>

        <section class="settings-section settings-section-paths">
          <div class="section-labels">
            <span class="section-title">${t("settings.runtimePaths")}</span>
            <small class="section-hint">${t("settings.runtimePathsHint")}</small>
          </div>
          <label class="field path-field">
            <span>${t("settings.gameDirectory")}</span>
            <div class="path-row">
              <input id="gamePathInput" readonly placeholder="${t("settings.gameDirectoryPlaceholder")}" />
              <div class="inline-actions">
                <button class="button button-solid path-button" id="browseGamePathBtn" type="button">${t("common.browse")}</button>
                <button class="button button-ghost path-button" id="clearGamePathBtn" type="button">${t("common.clear")}</button>
              </div>
            </div>
          </label>
          <label class="field path-field">
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
          <section class="settings-section">
            <div class="section-labels">
              <span class="section-title">${t("settings.background")}</span>
              <small class="section-hint section-status-pill" id="backgroundStatusValue" data-tone="warn">${t("settings.backgroundUnset")}</small>
            </div>
            <div class="background-actions">
              <button class="button button-solid path-button" id="browseBackgroundBtn" type="button">${t("common.browse")}</button>
              <button class="button button-ghost path-button" id="resetBackgroundBtn" type="button">${t("common.reset")}</button>
            </div>
          </section>

          <section class="settings-section">
            <div class="section-labels">
              <span class="section-title">${t("settings.uiOpacity")}</span>
              <small class="section-hint section-value-pill"><strong id="backgroundOpacityValue">35%</strong></small>
            </div>
            <div class="opacity-control">
              <input id="backgroundOpacityInput" type="range" min="0" max="100" step="1" value="19" />
            </div>
          </section>
        </div>

        <div class="toggle-grid">
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
        </div>
      </div>
    </aside>

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
        <div class="settings-grid auth-grid">
          <label class="field settings-card">
            <span>${t("settings.account")}</span>
            <input id="loginAccountInput" autocomplete="off" placeholder="${t("settings.accountPlaceholder")}" />
          </label>
          <label class="field settings-card">
            <span>${t("settings.password")}</span>
            <input id="loginPasswordInput" type="password" autocomplete="new-password" placeholder="${t("settings.passwordPlaceholder")}" />
          </label>
        </div>
        <div class="auth-actions">
          <button class="button button-solid" id="authCancelBtn" type="button">${t("common.close")}</button>
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
  pathHintValue: document.getElementById("pathHintValue"),
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
  authSubmitBtn: document.getElementById("authSubmitBtn"),
  loginAccountInput: document.getElementById("loginAccountInput"),
  loginPasswordInput: document.getElementById("loginPasswordInput"),
};

const maxRenderedLogs = 300;
let activeCaptchaURL = "";
let captchaDismissed = false;
let appBootstrapped = false;
let latestConfigView = {};
let latestState = null;
let lastScanHintToastKey = "";
let monitorPauseRequested = false;
let authSheetMode = "existing";
let authSheetContext = {
  baselineAccount: "",
  currentLoggedIn: false,
  hasStoredPassword: false,
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

function refreshAuthSubmitState(cfg = latestConfigView) {
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
    currentLoggedIn: mode !== "new" && Boolean(cfg.account_login ?? cfg.accountLogin),
    hasStoredPassword: mode !== "new" && Boolean(cfg.has_password),
  };
  elements.authTitle.textContent = mode === "new" ? t("auth.titleAdd") : t("auth.title");
  elements.authCopy.textContent = mode === "new" ? t("auth.copyAdd") : t("auth.copy");
  elements.loginAccountInput.value = mode === "new" ? "" : currentAccount;
  elements.loginPasswordInput.value = "";
  markSecretFieldClean("password");
  syncAuthPasswordPlaceholder(cfg);
  refreshAuthSubmitState(cfg);
  elements.authOverlay.hidden = false;
  window.setTimeout(() => {
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
  };
  elements.loginPasswordInput.value = "";
  markSecretFieldClean("password");
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

function syncCaptcha(state) {
  const pending = Boolean(state.captchaPending && state.captchaURL);
  const url = pending ? String(state.captchaURL) : "";

  if (pending) {
    elements.captchaCopy.textContent = t("captcha.copy");
    if (activeCaptchaURL !== url) {
      activeCaptchaURL = url;
      captchaDismissed = false;
      elements.captchaFrame.src = url;
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
  const pathHintText = translateMessage(state.gamePathMessage, state.gamePathPrompt || "");
  elements.errorValue.textContent = formatErrorValue(state.lastError, state.lastErrorMessage);
  elements.pathHintValue.hidden = !pathHintText;
  elements.pathHintValue.textContent = pathHintText;

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
  const nextOpacity = sliderToOpacityPercent(elements.backgroundOpacityInput.value) / 100;
  const currentOpacity = Number(currentCfg.background_opacity ?? currentCfg.backgroundOpacity ?? 0.35);
  const needsFeatureSave =
    nextGamePath !== currentGamePath ||
    nextAutoClose !== currentAutoClose ||
    nextAutoWindowCapture !== currentAutoWindowCapture ||
    nextPanelBlur !== currentPanelBlur ||
    Math.abs(nextOpacity - currentOpacity) > 0.0001;
  const needsLauncherSave = nextLauncherPath !== currentLauncherPath;

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
  const mode = elements.accountStatusBtn.dataset.mode === "new" ? "new" : "existing";
  openAuthSheet(mode);
});
elements.addAccountBtn.addEventListener("click", () => {
  openAuthSheet("new");
});
elements.editAccountBtn.addEventListener("click", () => {
  openAuthSheet("existing");
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
  closeAuthSheet();
});
elements.authOverlay.addEventListener("click", (event) => {
  if (event.target === elements.authOverlay) {
    closeAuthSheet();
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
  const result = await runTask(
    () => Login(elements.loginAccountInput.value, elements.loginPasswordInput.value),
    (value) => value,
    "neutral",
  );
  if (result?.ok) {
    markSecretFieldClean("password");
    closeAuthSheet();
  }
  if (result?.needsCaptcha) {
    openSettings();
    closeAuthSheet();
    const url = result.captchaURL || result.CaptchaURL || "";
    if (url) {
      activeCaptchaURL = String(url);
      captchaDismissed = false;
      elements.captchaFrame.src = activeCaptchaURL;
      elements.captchaOverlay.hidden = false;
      void syncMonitorPauseState();
    }
  }
});

elements.launchGameBtn.addEventListener("click", async () => {
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
  void syncMonitorPauseState();
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

    appBootstrapped = true;
    renderState(state);
    refreshBackground().catch((error) => {
      showPayload(formatError(error), "error");
    });
    if (!state.gamePathValid) {
      showPayload(translateMessage(state.gamePathMessage, state.gamePathPrompt || t("status.selectPathFirst")), "warn");
      return;
    }
    showPayload(
        {
          session: formatSessionStatus(state, state.config || {}).text,
          api: formatAPIStatus(state, state.config || {}).text,
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
