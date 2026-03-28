import "./style.css";
import { applyLocale, getLocale, initI18n, listLocales, t, translateMessage } from "./i18n";

import {
  BackgroundDataURL,
  BrowseBackgroundImage,
  Bootstrap,
  BrowseGamePath,
  LaunchGame,
  LogSnapshot,
  Login,
  ResetQuitFlag,
  ResetBackground,
  ScanClipboard,
  ScanURL,
  ScanWindow,
  RecordClientMessage,
  SaveSetting,
  UpdateBackground,
  UpdateConfig,
  ManualRefreshDispatch,
  ManualFetchBiliHitoken,
} from "../wailsjs/go/main/App";
import { EventsOn, Quit } from "../wailsjs/runtime/runtime";

const shellBaseWidth = 1440;
const shellBaseHeight = 920;
const sensitiveKeys = new Set([
  "password",
  "hi3uid",
  "bilihitoken",
  "bili_hitoken",
  "access_key",
  "combo_token",
  "accounttoken",
  "account_token",
]);
const largeBlobKeys = new Set(["dispatch_data"]);

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
            <span class="status-name">${t("topbar.monitor")}</span>
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
            <span class="status-name">${t("topbar.dispatchLabel")}</span>
          </div>
          <strong class="status-value" id="dispatchValue">${t("topbar.dispatchPending")}</strong>
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
          <div class="action-pill" id="versionPill">${t("action.versionPending")}</div>
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
            <button class="button action-button primary-button" id="scanClipboardBtn" type="button">${t("action.scanClipboard")}</button>
            <button class="button action-button primary-button" id="scanWindowBtn" type="button">${t("action.scanWindow")}</button>
          </div>
          <div class="utility-actions">
            <button class="button action-button utility-button" id="manualDispatchBtn" type="button">${t("action.refreshDispatch")}</button>
            <button class="button action-button utility-button" id="manualFetchTokenBtn" type="button">${t("action.fetchBiliHitoken")}</button>
          </div>
        </div>

        <details class="manual-details">
          <summary>${t("action.manualLink")}</summary>
          <label class="field manual-field">
            <span>${t("action.qrLink")}</span>
            <textarea id="urlInput" rows="3" placeholder="${t("action.qrLinkPlaceholder")}"></textarea>
          </label>
          <button class="button button-ghost manual-button" id="scanUrlBtn" type="button">${t("action.submitLink")}</button>
        </details>

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
            <p class="eyebrow">${t("settings.control")}</p>
            <h2>${t("settings.title")}</h2>
          </div>
          <div class="settings-top-actions">
            <button class="button button-small" id="saveBtn" type="button">${t("common.save")}</button>
            <button class="button button-accent button-small" id="loginBtn" type="button">${t("common.login")}</button>
            <button class="button button-solid button-small settings-close-button" id="settingsCloseBtn" type="button">${t("common.close")}</button>
          </div>
        </header>

        <p class="settings-note" id="pathHintValue" hidden></p>

        <div class="settings-grid">
          <label class="field settings-card">
            <span>${t("settings.account")}</span>
            <input id="accountInput" autocomplete="off" placeholder="${t("settings.accountPlaceholder")}" />
          </label>
          <label class="field settings-card">
            <span>${t("settings.password")}</span>
            <input id="passwordInput" type="password" autocomplete="new-password" placeholder="${t("settings.passwordPlaceholder")}" />
          </label>
          <label class="field settings-card">
            <span>${t("settings.hi3uid")}</span>
            <input id="hi3uidInput" autocomplete="off" placeholder="${t("settings.hi3uidPlaceholder")}" />
          </label>
          <label class="field settings-card">
            <span>${t("settings.biliHitoken")}</span>
            <input id="biliHitokenInput" autocomplete="off" placeholder="${t("settings.biliHitokenPlaceholder")}" />
          </label>
          <label class="field settings-card settings-card-wide">
            <span>${t("settings.locale")}</span>
            <select id="localeSelect" aria-label="${t("settings.locale")}"></select>
          </label>
        </div>

        <section class="settings-section">
          <div class="section-labels">
            <span class="section-title">${t("settings.gameDirectory")}</span>
            <small class="section-hint">${t("settings.gameDirectoryHint")}</small>
          </div>
          <div class="path-row">
            <input id="gamePathInput" readonly placeholder="${t("settings.gameDirectoryPlaceholder")}" />
            <button class="button button-solid path-button" id="browseGamePathBtn" type="button">${t("common.browse")}</button>
          </div>
        </section>

        <div class="settings-split">
          <section class="settings-section">
            <div class="section-labels">
              <span class="section-title">${t("settings.background")}</span>
              <small class="section-hint" id="backgroundStatusValue">${t("settings.backgroundUnset")}</small>
            </div>
            <div class="background-actions">
              <button class="button button-solid path-button" id="browseBackgroundBtn" type="button">${t("common.browse")}</button>
              <button class="button button-ghost path-button" id="resetBackgroundBtn" type="button">${t("common.reset")}</button>
            </div>
          </section>

          <section class="settings-section">
            <div class="section-labels">
              <span class="section-title">${t("settings.uiOpacity")}</span>
              <small class="section-hint"><strong id="backgroundOpacityValue">35%</strong></small>
            </div>
            <input id="backgroundOpacityInput" type="range" min="0" max="100" step="1" value="35" />
          </section>
        </div>

        <div class="toggle-grid">
          <label class="toggle">
            <input id="panelBlurInput" type="checkbox" />
            <span class="toggle-copy">
              <strong>${t("settings.blurTitle")}</strong>
            </span>
          </label>
          <label class="toggle">
            <input id="clipCheckInput" type="checkbox" />
            <span class="toggle-copy">
              <strong>${t("settings.clipboardTitle")}</strong>
            </span>
          </label>
          <label class="toggle">
            <input id="autoClipInput" type="checkbox" />
            <span class="toggle-copy">
              <strong>${t("settings.windowTitle")}</strong>
            </span>
          </label>
          <label class="toggle">
            <input id="autoCloseInput" type="checkbox" />
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
  versionPill: document.getElementById("versionPill"),
  actionValue: document.getElementById("actionValue"),
  errorValue: document.getElementById("errorValue"),
  accountInput: document.getElementById("accountInput"),
  passwordInput: document.getElementById("passwordInput"),
  hi3uidInput: document.getElementById("hi3uidInput"),
  biliHitokenInput: document.getElementById("biliHitokenInput"),
  localeSelect: document.getElementById("localeSelect"),
  gamePathInput: document.getElementById("gamePathInput"),
  backgroundOpacityInput: document.getElementById("backgroundOpacityInput"),
  backgroundOpacityValue: document.getElementById("backgroundOpacityValue"),
  backgroundStatusValue: document.getElementById("backgroundStatusValue"),
  pathHintValue: document.getElementById("pathHintValue"),
  panelBlurInput: document.getElementById("panelBlurInput"),
  clipCheckInput: document.getElementById("clipCheckInput"),
  autoClipInput: document.getElementById("autoClipInput"),
  autoCloseInput: document.getElementById("autoCloseInput"),
  urlInput: document.getElementById("urlInput"),
  responseBox: document.getElementById("responseBox"),
  logList: document.getElementById("logList"),
  launchGameBtn: document.getElementById("launchGameBtn"),
  settingsBtn: document.getElementById("settingsBtn"),
  settingsBackdrop: document.getElementById("settingsBackdrop"),
  settingsSheet: document.getElementById("settingsSheet"),
  settingsCloseBtn: document.getElementById("settingsCloseBtn"),
  browseGamePathBtn: document.getElementById("browseGamePathBtn"),
  browseBackgroundBtn: document.getElementById("browseBackgroundBtn"),
  resetBackgroundBtn: document.getElementById("resetBackgroundBtn"),
  saveBtn: document.getElementById("saveBtn"),
  loginBtn: document.getElementById("loginBtn"),
  scanUrlBtn: document.getElementById("scanUrlBtn"),
  scanClipboardBtn: document.getElementById("scanClipboardBtn"),
  scanWindowBtn: document.getElementById("scanWindowBtn"),
  manualDispatchBtn: document.getElementById("manualDispatchBtn"),
  manualFetchTokenBtn: document.getElementById("manualFetchTokenBtn"),
  captchaOverlay: document.getElementById("captchaOverlay"),
  captchaFrame: document.getElementById("captchaFrame"),
  captchaCopy: document.getElementById("captchaCopy"),
  captchaCloseBtn: document.getElementById("captchaCloseBtn"),
};

const maxRenderedLogs = 300;
let activeCaptchaURL = "";
let captchaDismissed = false;
let appBootstrapped = false;
let latestConfigView = {};
let latestState = null;
let lastScanHintToastKey = "";
const seenLogKeys = new Set();
const autofillGuardNames = Object.freeze({
  account: "ctl_contact_ref",
  password: "ctl_access_phrase",
  hi3uid: "ctl_release_ref",
  biliHitoken: "ctl_dispatch_ref",
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

installAutofillGuard(elements.accountInput, autofillGuardNames.account);
installAutofillGuard(elements.passwordInput, autofillGuardNames.password);
installAutofillGuard(elements.hi3uidInput, autofillGuardNames.hi3uid, { inputMode: "numeric" });
installAutofillGuard(elements.biliHitokenInput, autofillGuardNames.biliHitoken);

const secretFieldMeta = {
  password: {
    input: elements.passwordInput,
    hasKey: "has_password",
    maskKey: "masked_password",
    defaultPlaceholder: elements.passwordInput.getAttribute("placeholder") || "",
  },
  hi3uid: {
    input: elements.hi3uidInput,
    hasKey: "has_hi3uid",
    maskKey: "masked_hi3uid",
    defaultPlaceholder: elements.hi3uidInput.getAttribute("placeholder") || "",
  },
  biliHitoken: {
    input: elements.biliHitokenInput,
    hasKey: "has_bilihitoken",
    maskKey: "masked_bilihitoken",
    defaultPlaceholder: elements.biliHitokenInput.getAttribute("placeholder") || "",
  },
};

const secretFieldDirty = {
  password: false,
  hi3uid: false,
  biliHitoken: false,
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
    [/(\"hi3uid\"\s*:\s*\")([^\"]*)(\")/gi, (_, prefix, value, suffix) => `${prefix}${maskSecret(value)}${suffix}`],
    [/(\"biliHitoken\"\s*:\s*\")([^\"]*)(\")/gi, (_, prefix, value, suffix) => `${prefix}${maskSecret(value)}${suffix}`],
    [/(\"BILIHITOKEN\"\s*:\s*\")([^\"]*)(\")/gi, (_, prefix, value, suffix) => `${prefix}${maskSecret(value)}${suffix}`],
    [/(\"access_key\"\s*:\s*\")([^\"]*)(\")/gi, "$1***$3"],
    [/(\"combo_token\"\s*:\s*\")([^\"]*)(\")/gi, "$1***$3"],
    [/(\"accountToken\"\s*:\s*\")([^\"]*)(\")/gi, "$1***$3"],
    [/(\bpassword=)([^&\s]+)/gi, "$1***"],
    [/(\baccess_key=)([^&\s]+)/gi, "$1***"],
    [/(\bcombo_token=)([^&\s]+)/gi, "$1***"],
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

function syncRangeValue(input, value) {
  if (document.activeElement !== input) {
    input.value = String(value);
  }
}

function hasSecretValue(key, cfg = latestConfigView) {
  const meta = secretFieldMeta[key];
  if (!meta) {
    return false;
  }
  return Boolean(String(meta.input.value ?? "").trim()) || Boolean(cfg?.[meta.hasKey]);
}

function refreshDraftActionState(cfg = latestConfigView) {
  elements.manualDispatchBtn.disabled = !hasSecretValue("hi3uid", cfg) || !hasSecretValue("biliHitoken", cfg);
}

function hasCachedSessionState(cfg) {
  return Boolean((cfg.account_login ?? cfg.accountLogin) || (cfg.last_login_succ && cfg.has_access_key));
}

function formatLocaleLabel(locale) {
  const key = `locale.name.${locale}`;
  const translated = t(key);
  if (translated !== key) {
    return translated;
  }
  return locale;
}

function populateLocaleOptions() {
  const locales = listLocales();
  const currentLocale = getLocale();
  elements.localeSelect.innerHTML = "";

  locales.forEach((locale) => {
    const option = document.createElement("option");
    option.value = locale;
    option.textContent = formatLocaleLabel(locale);
    option.selected = locale === currentLocale;
    elements.localeSelect.append(option);
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
  const clampedPercent = Math.max(0, Math.min(100, Number(percent || 0)));
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

function previewOpacity(percent) {
  const normalized = Math.max(0, Math.min(100, Number(percent || 0)));
  elements.backgroundOpacityInput.value = String(normalized);
  elements.backgroundOpacityValue.textContent = `${normalized}%`;
  applySurfaceOpacity(normalized);
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

function formatDispatchStatus(state, cfg) {
  const hasDispatch = Boolean(String(state.dispatchSource ?? "").trim()) || Boolean(cfg.has_dispatch_data);
  return hasDispatch
    ? { tone: "ok", text: t("status.cached") }
    : { tone: "error", text: t("status.requestRequired") };
}

function formatSessionStatus(state, cfg) {
  const account = String(cfg.account ?? "").trim();
  const hasPassword = Boolean(cfg.has_password) || Boolean(String(elements.passwordInput?.value ?? "").trim());
  const hasCredentials = Boolean(account && hasPassword);
  const hasCachedSession = hasCachedSessionState(cfg);

  if (state.captchaPending) {
    return { tone: "warn", text: t("status.verificationRequired") };
  }
  if (!hasCredentials) {
    return { tone: "error", text: t("status.notLoggedIn") };
  }
  if (hasCachedSession) {
    return { tone: "ok", text: t("status.cached") };
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
    return;
  }

  captchaDismissed = false;
  elements.captchaOverlay.hidden = true;
  if (activeCaptchaURL) {
    activeCaptchaURL = "";
    elements.captchaFrame.src = "about:blank";
  }
}

function renderState(state) {
  latestState = state || null;
  const cfg = state.config || {};
  latestConfigView = cfg;
  const clipCheck = Boolean(cfg.clip_check ?? cfg.clipCheck);
  const autoClip = Boolean(cfg.auto_clip ?? cfg.autoClip);
  const autoClose = Boolean(cfg.auto_close ?? cfg.autoClose);
  const gamePath = cfg.game_path ?? cfg.gamePath ?? "";
  const hasBackgroundImage = Boolean(cfg.has_background_image ?? cfg.hasBackgroundImage);
  const backgroundOpacity = Math.round(
    Math.max(0, Math.min(1, Number(cfg.background_opacity ?? cfg.backgroundOpacity ?? 0.35))) * 100,
  );
  const panelBlur = cfg.panel_blur ?? cfg.panelBlur ?? true;
  const session = formatSessionStatus(state, cfg);
  const dispatch = formatDispatchStatus(state, cfg);
  const versionText = String(cfg.bh_ver ?? cfg.bhVer ?? "").trim();
  const hasCachedAccessKey = hasCachedSessionState(cfg);
  const gameTone = state.gamePathValid ? "ok" : "warn";
  const gameText = state.gamePathValid
    ? t("status.gameConfigured", { version: versionText }).trim()
    : gamePath
      ? t("status.gameNeedsFix")
      : t("status.gameOptional");

  setStatus(
    elements.appDot,
    elements.appValue,
    appBootstrapped ? "ok" : "error",
    state.running ? t("status.monitoring") : t("status.idle"),
  );
  setStatus(elements.sessionDot, elements.sessionValue, session.tone, session.text);
  setStatus(elements.dispatchDot, elements.dispatchValue, dispatch.tone, dispatch.text);
  setStatus(elements.gameDot, elements.gameValue, gameTone, gameText);
  elements.versionPill.textContent = versionText ? `BHVer ${versionText}` : t("action.versionPending");

  elements.actionValue.textContent = formatActionValue(state.lastAction);
  const pathHintText = translateMessage(state.gamePathMessage, state.gamePathPrompt || "");
  elements.errorValue.textContent = formatErrorValue(state.lastError, state.lastErrorMessage);
  elements.pathHintValue.hidden = !pathHintText;
  elements.pathHintValue.textContent = pathHintText;

  const hintCode = String(state?.lastErrorMessage?.code ?? "").trim();
  if (windowScanHintCodes.has(hintCode)) {
    const hintText = translateMessage(state.lastErrorMessage, state.lastError || "");
    const toastKey = `${hintCode}|${hintText}`;
    if (hintText && toastKey !== lastScanHintToastKey) {
      lastScanHintToastKey = toastKey;
      showPayload(hintText, "warn");
    }
  } else {
    lastScanHintToastKey = "";
  }

  syncInputValue(elements.accountInput, cfg.account);
  syncSecretInput("password", cfg);
  syncSecretInput("hi3uid", cfg);
  syncSecretInput("biliHitoken", cfg);
  syncInputValue(elements.gamePathInput, gamePath);
  syncRangeValue(elements.backgroundOpacityInput, backgroundOpacity);
  previewOpacity(backgroundOpacity);
  elements.backgroundStatusValue.textContent = hasBackgroundImage ? t("settings.backgroundSet") : t("settings.backgroundUnset");
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
    elements.clipCheckInput.checked = clipCheck;
    elements.autoClipInput.checked = autoClip;
    elements.autoCloseInput.checked = autoClose;
  }
  elements.launchGameBtn.disabled = !state.gamePathValid;
  refreshDraftActionState(cfg);
  elements.loginBtn.disabled = hasCachedAccessKey;
  elements.loginBtn.textContent = hasCachedAccessKey ? t("status.loggedIn") : t("common.login");
  elements.loginBtn.classList.toggle("button-success", hasCachedAccessKey);
  elements.loginBtn.classList.toggle("button-accent", !hasCachedAccessKey);
  if (elements.localeSelect.value !== getLocale()) {
    elements.localeSelect.value = getLocale();
  }

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
elements.settingsCloseBtn.addEventListener("click", () => closeSettings());
elements.settingsBackdrop.addEventListener("click", () => closeSettings());
elements.localeSelect.addEventListener("change", () => {
  const selectedLocale = elements.localeSelect.value;
  if (!selectedLocale || selectedLocale === getLocale()) {
    return;
  }
  applyLocale(selectedLocale);
});
elements.passwordInput.addEventListener("input", () => markSecretFieldDirty("password"));
elements.hi3uidInput.addEventListener("input", () => {
  markSecretFieldDirty("hi3uid");
  refreshDraftActionState();
});
elements.biliHitokenInput.addEventListener("input", () => {
  markSecretFieldDirty("biliHitoken");
  refreshDraftActionState();
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
      UpdateConfig(
        selected,
        elements.clipCheckInput.checked,
        elements.autoCloseInput.checked,
        elements.autoClipInput.checked,
        elements.panelBlurInput.checked,
      ),
    () => ({ saved: true, gamePath: selected }),
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
        Number(elements.backgroundOpacityInput.value || "35") / 100,
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
  const percent = Number(elements.backgroundOpacityInput.value || "35");
  const state = await runTask(
    () => UpdateBackground("", percent / 100),
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
  const settingsToSave = [["account", elements.accountInput.value || ""]];
  const secretKeysToReset = [];

  if (secretFieldDirty.password) {
    settingsToSave.push(["password", elements.passwordInput.value || ""]);
    secretKeysToReset.push("password");
  }
  if (secretFieldDirty.hi3uid) {
    settingsToSave.push(["HI3UID", elements.hi3uidInput.value || ""]);
    secretKeysToReset.push("hi3uid");
  }
  if (secretFieldDirty.biliHitoken) {
    settingsToSave.push(["BILIHITOKEN", elements.biliHitokenInput.value || ""]);
    secretKeysToReset.push("biliHitoken");
  }

  for (const [key, value] of settingsToSave) {
    const savedState = await runTask(() => SaveSetting(key, value), null, "soft");
    if (!savedState) {
      return;
    }
  }

  const state = await runTask(
    () =>
      UpdateConfig(
        elements.gamePathInput.value,
        elements.clipCheckInput.checked,
        elements.autoCloseInput.checked,
        elements.autoClipInput.checked,
        elements.panelBlurInput.checked,
      ),
    () => ({ saved: true, gamePath: elements.gamePathInput.value || null }),
  );
  if (!state) {
    return;
  }
  secretKeysToReset.forEach((key) => markSecretFieldClean(key));
  elements.passwordInput.value = "";
  elements.hi3uidInput.value = "";
  elements.biliHitokenInput.value = "";
  renderState(state);
  const cfg = state.config || {};
  showPayload(
    {
      saved: true,
      account: String(cfg.account ?? "").trim() ? "configured" : "cleared",
      password: cfg.has_password ? "configured" : "cleared",
      hi3uid: cfg.masked_hi3uid || "cleared",
      biliHitoken: cfg.masked_bilihitoken || "cleared",
      gamePath: cfg.game_path ?? cfg.gamePath ?? null,
    },
    "neutral",
  );
});

elements.loginBtn.addEventListener("click", async () => {
  const result = await runTask(
    () => Login(elements.accountInput.value, elements.passwordInput.value),
    (value) => value,
    "neutral",
  );
  if (result?.needsCaptcha) {
    openSettings();
    // Ensure captcha overlay shows immediately if backend returned a URL
    const url = result.captchaURL || result.CaptchaURL || "";
    if (url) {
      activeCaptchaURL = String(url);
      captchaDismissed = false;
      elements.captchaFrame.src = activeCaptchaURL;
      elements.captchaOverlay.hidden = false;
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

elements.scanUrlBtn.addEventListener("click", async () => {
  await runTask(() => ScanURL(elements.urlInput.value), (value) => value);
});

elements.manualDispatchBtn.addEventListener("click", async () => {
  const hi3uid = elements.hi3uidInput.value || "";
  const biliHitoken = elements.biliHitokenInput.value || "";
  const state = await runTask(
    () => ManualRefreshDispatch(hi3uid, biliHitoken),
    (value) => value,
  );
  if (state) {
    renderState(state);
  }
});

elements.manualFetchTokenBtn.addEventListener("click", async () => {
  const state = await runTask(() => ManualFetchBiliHitoken(), (value) => value);
  if (state) {
    renderState(state);
  }
});

elements.scanClipboardBtn.addEventListener("click", async () => {
  await runTask(() => ScanClipboard(), (matched) => ({ matched }), "soft");
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
        dispatch: formatDispatchStatus(state, state.config || {}).text,
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
