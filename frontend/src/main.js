import "./style.css";

import {
  BackgroundDataURL,
  BrowseBackgroundImage,
  Bootstrap,
  BrowseGamePath,
  LaunchGame,
  Login,
  ResetQuitFlag,
  ResetBackground,
  ScanClipboard,
  ScanURL,
  ScanWindow,
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
  "access_key",
  "combo_token",
  "accounttoken",
  "account_token",
]);
const largeBlobKeys = new Set(["dispatch_cache"]);

document.querySelector("#app").innerHTML = `
  <div class="custom-background" id="customBackground" hidden></div>
  <main class="shell">
    <section class="panel topbar">
      <div class="status-grid">
        <article class="status-card">
          <div class="status-head">
            <span class="status-dot" id="appDot"></span>
            <span class="status-name">\u76d1\u542c</span>
          </div>
          <strong class="status-value" id="appValue">\u542f\u52a8\u4e2d</strong>
        </article>
        <article class="status-card">
          <div class="status-head">
            <span class="status-dot" id="sessionDot"></span>
            <span class="status-name">\u4f1a\u8bdd</span>
          </div>
          <strong class="status-value" id="sessionValue">\u672a\u77e5</strong>
        </article>
        <article class="status-card">
          <div class="status-head">
            <span class="status-dot" id="dispatchDot"></span>
            <span class="status-name">Dispatch</span>
          </div>
          <strong class="status-value" id="dispatchValue">\u672a\u5c31\u7eea</strong>
        </article>
        <article class="status-card">
          <div class="status-head">
            <span class="status-dot" id="gameDot"></span>
            <span class="status-name">\u6e38\u620f\u8def\u5f84</span>
          </div>
          <strong class="status-value" id="gameValue">\u672a\u8bbe\u7f6e</strong>
        </article>
      </div>
      <div class="topbar-actions">
        <button class="button button-launch-top" id="launchGameBtn" type="button">\u542f\u52a8\u6e38\u620f</button>
        <button class="icon-button" id="settingsBtn" type="button" aria-label="\u6253\u5f00\u8bbe\u7f6e">
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <path d="M19.2 13.5a7.7 7.7 0 0 0 .1-1.5 7.7 7.7 0 0 0-.1-1.5l2-1.6a.7.7 0 0 0 .2-.9l-1.9-3.2a.7.7 0 0 0-.9-.3l-2.4 1a7.1 7.1 0 0 0-2.6-1.5l-.4-2.6a.7.7 0 0 0-.7-.6H9.5a.7.7 0 0 0-.7.6l-.4 2.6a7.1 7.1 0 0 0-2.6 1.5l-2.4-1a.7.7 0 0 0-.9.3L.6 8a.7.7 0 0 0 .2.9l2 1.6a7.7 7.7 0 0 0-.1 1.5 7.7 7.7 0 0 0 .1 1.5l-2 1.6a.7.7 0 0 0-.2.9l1.9 3.2a.7.7 0 0 0 .9.3l2.4-1a7.1 7.1 0 0 0 2.6 1.5l.4 2.6a.7.7 0 0 0 .7.6h3.8a.7.7 0 0 0 .7-.6l.4-2.6a7.1 7.1 0 0 0 2.6-1.5l2.4 1a.7.7 0 0 0 .9-.3l1.9-3.2a.7.7 0 0 0-.2-.9l-2-1.6ZM11.4 15.6A3.6 3.6 0 1 1 15 12a3.6 3.6 0 0 1-3.6 3.6Z"/>
          </svg>
        </button>
      </div>
    </section>

    <section class="workspace">
      <article class="panel action-panel">
        <div class="panel-head">
          <div>
            <h2>\u8bc6\u522b</h2>
          </div>
        </div>

        <div class="info-grid">
          <div class="info-card">
            <span>\u6700\u8fd1\u64cd\u4f5c</span>
            <strong id="actionValue">\u5f85\u547d</strong>
          </div>
          <div class="info-card">
            <span>\u6700\u8fd1\u9519\u8bef</span>
            <strong id="errorValue">\u65e0</strong>
          </div>
        </div>

        <div class="manual-button-row">
          <button class="button button-solid manual-button" id="manualFetchTokenBtn" type="button">\u624b\u52a8\u83b7\u53d6 BILIHITOKEN</button>
        </div>

        <div class="scan-actions">
          <button class="button button-solid" id="scanClipboardBtn" type="button">\u4ece\u526a\u8d34\u677f\u8bc6\u522b</button>
          <button class="button button-solid" id="scanWindowBtn" type="button">\u4ece\u6e38\u620f\u7a97\u53e3\u8bc6\u522b</button>
          <button class="button button-ghost" id="manualDispatchBtn" type="button">\u624b\u52a8\u66f4\u65b0 Dispatch</button>
        </div>

        <details class="manual-details">
          <summary>\u624b\u52a8\u7c98\u8d34\u94fe\u63a5</summary>
          <label class="field manual-field">
            <span>\u4e8c\u7ef4\u7801\u94fe\u63a5</span>
            <textarea id="urlInput" rows="3" placeholder="\u7c98\u8d34\u4e8c\u7ef4\u7801\u94fe\u63a5\u540e\u63d0\u4ea4"></textarea>
          </label>
          <button class="button button-ghost manual-button" id="scanUrlBtn" type="button">\u63d0\u4ea4\u94fe\u63a5</button>
        </details>

        <div class="response-box" id="responseBox">\u8fd4\u56de\u7ed3\u679c\u4f1a\u663e\u793a\u5728\u8fd9\u91cc\u3002</div>
      </article>

      <article class="panel log-panel">
        <div class="panel-head panel-head-tight">
          <div>
            <h2>\u65e5\u5fd7</h2>
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
            <h2>\u8bbe\u7f6e</h2>
          </div>
          <div class="settings-top-actions">
            <button class="button button-small" id="saveBtn" type="button">\u4fdd\u5b58</button>
            <button class="button button-accent button-small" id="loginBtn" type="button">\u767b\u5f55</button>
            <button class="settings-close-button" id="settingsCloseBtn" type="button">\u5173\u95ed</button>
          </div>
        </header>

        <p class="settings-note" id="pathHintValue" hidden></p>

        <label class="field">
          <span>B站账号</span>
          <input id="accountInput" autocomplete="username" placeholder="account" />
        </label>
        <label class="field">
          <span>\u5bc6\u7801</span>
          <input id="passwordInput" type="password" autocomplete="current-password" placeholder="password" />
        </label>
        <label class="field">
          <span>HI3 UID</span>
          <input id="hi3uidInput" placeholder="HI3UID (用于手动更新 dispatch)" />
        </label>
        <label class="field">
          <span>BILIHITOKEN</span>
          <input id="biliHitokenInput" placeholder="BILIHITOKEN (用于手动更新 dispatch)" />
        </label>
        <label class="field">
          <span>\u6e38\u620f\u76ee\u5f55</span>
          <div class="path-row">
            <input id="gamePathInput" readonly placeholder="\u8bf7\u9009\u62e9\u5d29\u574f3\u5b89\u88c5\u76ee\u5f55" />
            <button class="button button-solid path-button" id="browseGamePathBtn" type="button">\u6d4f\u89c8</button>
          </div>
        </label>

        <label class="field">
          <span>\u80cc\u666f</span>
          <div class="background-actions">
            <button class="button button-solid path-button" id="browseBackgroundBtn" type="button">\u6d4f\u89c8</button>
            <button class="button button-ghost path-button" id="resetBackgroundBtn" type="button">\u91cd\u7f6e</button>
          </div>
          <small class="field-hint" id="backgroundStatusValue">\u672a\u8bbe\u7f6e</small>
        </label>

        <label class="field">
          <span>\u754c\u9762\u900f\u660e\u5ea6 <strong id="backgroundOpacityValue">35%</strong></span>
          <input id="backgroundOpacityInput" type="range" min="0" max="100" step="1" value="35" />
        </label>

        <div class="toggle-group">
          <label class="toggle">
            <input id="panelBlurInput" type="checkbox" />
            <span class="toggle-copy">
              <strong>\u5f00\u542f\u6bdb\u73bb\u7483</strong>
              <small>\u7ed9\u4e3b\u754c\u9762\u548c\u8bbe\u7f6e\u9762\u677f\u4fdd\u7559\u80cc\u666f\u6a21\u7cca\u6548\u679c\u3002</small>
            </span>
          </label>
          <label class="toggle">
            <input id="clipCheckInput" type="checkbox" />
            <span class="toggle-copy">
              <strong>\u8bfb\u53d6\u526a\u8d34\u677f</strong>
              <small>\u6301\u7eed\u68c0\u67e5\u526a\u8d34\u677f\u91cc\u7684\u622a\u56fe\uff0c\u6709\u4e8c\u7ef4\u7801\u5c31\u76f4\u63a5\u8bc6\u522b\u3002</small>
            </span>
          </label>
          <label class="toggle">
            <input id="autoClipInput" type="checkbox" />
            <span class="toggle-copy">
              <strong>\u6293\u6e38\u620f\u7a97\u53e3</strong>
              <small>\u7b49\u5d29\u574f3\u7a97\u53e3\u51fa\u73b0\u540e\u81ea\u52a8\u622a\u56fe\uff0c\u7528\u4e8e\u8bc6\u522b\u767b\u5f55\u4e8c\u7ef4\u7801\u3002</small>
            </span>
          </label>
          <label class="toggle">
            <input id="autoCloseInput" type="checkbox" />
            <span class="toggle-copy">
              <strong>\u6210\u529f\u540e\u9000\u51fa</strong>
              <small>\u626b\u7801\u786e\u8ba4\u6210\u529f\u540e\u7acb\u5373\u9000\u51fa\u542f\u52a8\u5668\uff0c\u4e0d\u518d\u7ee7\u7eed\u76d1\u542c\u3002</small>
            </span>
          </label>
        </div>

        <div class="settings-actions">
          <!-- Buttons moved to top for sticky access -->
        </div>
      </div>
    </aside>

    <section class="captcha-overlay" id="captchaOverlay" hidden>
      <article class="captcha-modal">
        <header class="captcha-head">
          <div>
            <p class="eyebrow">\u9a8c\u8bc1</p>
            <h2>\u9700\u8981\u9a8c\u8bc1\u7801</h2>
          </div>
          <button class="button button-solid captcha-close" id="captchaCloseBtn" type="button">\u9690\u85cf</button>
        </header>
        <p class="captcha-copy" id="captchaCopy">\u5728\u4e0b\u65b9\u5b8c\u6210\u9a8c\u8bc1\u3002\u56de\u8c03\u6210\u529f\u540e\uff0c\u9762\u677f\u4f1a\u81ea\u52a8\u6536\u8d77\u3002</p>
        <iframe class="captcha-frame" id="captchaFrame" title="captcha verification"></iframe>
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
  accountInput: document.getElementById("accountInput"),
  passwordInput: document.getElementById("passwordInput"),
  hi3uidInput: document.getElementById("hi3uidInput"),
  biliHitokenInput: document.getElementById("biliHitokenInput"),
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
let settingsPinnedForPath = false;

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
  if (largeBlobKeys.has(normalizedKey) && typeof value === "string" && value.length > 96) {
    return `[redacted ${value.length} chars]`;
  }
  if (typeof value === "string") {
    return sanitizeText(value);
  }
  return value;
}

function showPayload(payload, tone = "neutral") {
  elements.responseBox.dataset.tone = tone;
  if (typeof payload === "string") {
    elements.responseBox.textContent = sanitizeText(payload);
    return;
  }
  elements.responseBox.textContent = JSON.stringify(sanitizeDisplayValue(payload), null, 2);
}

function appendLog(entry) {
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
  entries.forEach((entry) => appendLog(entry));
}

function syncInputValue(input, value) {
  if (document.activeElement !== input) {
    input.value = value ?? "";
  }
}

function syncRangeValue(input, value) {
  if (document.activeElement !== input) {
    input.value = String(value);
  }
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

function currentVersionKey(cfg) {
  const version = String(cfg?.bh_ver ?? cfg?.bhVer ?? "").trim();
  if (!version) {
    return "";
  }
  return `${version}_gf_android_bilibili`;
}

const actionTextMap = {
  monitoring: "\u76d1\u542c\u4e2d",
  stopped: "\u5df2\u505c\u6b62",
  scan_complete: "\u8bc6\u522b\u5b8c\u6210",
  launch_game: "\u5df2\u542f\u52a8\u6e38\u620f",
  scan: "\u6b63\u5728\u8bc6\u522b",
  quit_requested: "\u5df2\u8bf7\u6c42\u9000\u51fa",
  waiting_login: "\u7b49\u5f85\u767b\u5f55",
  waiting_window: "\u7b49\u5f85\u6e38\u620f\u7a97\u53e3",
  ticket_detected: "\u5df2\u8bc6\u522b\u4e8c\u7ef4\u7801",
  login: "\u6b63\u5728\u767b\u5f55",
  captcha_required: "\u7b49\u5f85\u9a8c\u8bc1",
};

function formatActionValue(value) {
  const key = String(value ?? "").trim().toLowerCase();
  if (!key || key === "none") {
    return "\u5f85\u547d";
  }
  if (actionTextMap[key]) {
    return actionTextMap[key];
  }
  return sanitizeText(key.replace(/_/g, " "));
}

function formatErrorValue(value) {
  const text = String(value ?? "").trim();
  if (!text || text.toLowerCase() === "none") {
    return "\u65e0";
  }
  return sanitizeText(text);
}

function formatDispatchStatus(state, cfg) {
  const hasCurrentBlob = Boolean((cfg.dispatch_data ?? cfg.dispatchData ?? "").trim());
  const cacheKey = currentVersionKey(cfg);
  const cache = cfg.dispatch_cache ?? cfg.dispatchCache ?? {};
  const hasCachedEntry = Boolean(cacheKey && cache && cache[cacheKey]?.data);
  const hasDispatch = Boolean(String(state.dispatchSource ?? "").trim()) || hasCurrentBlob || hasCachedEntry;
  return hasDispatch
    ? { tone: "ok", text: "\u5df2\u7f13\u5b58" }
    : { tone: "error", text: "\u9700\u8bf7\u6c42" };
}

function formatSessionStatus(state, cfg) {
  const account = String(cfg.account ?? "").trim();
  const password = String(cfg.password ?? "").trim();
  const hasCredentials = Boolean(account && password);
  const hasCachedSession = Boolean(cfg.last_login_succ && Number(cfg.uid ?? 0) > 0 && String(cfg.access_key ?? "").trim());

  if (state.captchaPending) {
    return { tone: "warn", text: "\u9700\u8981\u9a8c\u8bc1" };
  }
  if (!hasCredentials) {
    return { tone: "error", text: "\u672a\u767b\u5f55" };
  }
  if (hasCachedSession) {
    return { tone: "ok", text: "\u5df2\u7f13\u5b58" };
  }
  return { tone: "error", text: "\u5f85\u767b\u5f55" };
}

function openSettings(force = false) {
  if (force) {
    settingsPinnedForPath = true;
  }
  elements.settingsBackdrop.hidden = false;
  elements.settingsSheet.hidden = false;
}

function closeSettings(force = false) {
  if (settingsPinnedForPath && !force) {
    return;
  }
  settingsPinnedForPath = false;
  elements.settingsBackdrop.hidden = true;
  elements.settingsSheet.hidden = true;
}

function syncCaptcha(state) {
  const pending = Boolean(state.captchaPending && state.captchaURL);
  const url = pending ? String(state.captchaURL) : "";

  if (pending) {
    elements.captchaCopy.textContent = "\u5728\u4e0b\u65b9\u5b8c\u6210\u9a8c\u8bc1\u3002\u56de\u8c03\u6210\u529f\u540e\uff0c\u9762\u677f\u4f1a\u81ea\u52a8\u6536\u8d77\u3002";
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
  const cfg = state.config || {};
  const clipCheck = Boolean(cfg.clip_check ?? cfg.clipCheck);
  const autoClip = Boolean(cfg.auto_clip ?? cfg.autoClip);
  const autoClose = Boolean(cfg.auto_close ?? cfg.autoClose);
  const gamePath = cfg.game_path ?? cfg.gamePath ?? "";
  const backgroundPath = cfg.background_image ?? cfg.backgroundImage ?? "";
  const backgroundOpacity = Math.round(
    Math.max(0, Math.min(1, Number(cfg.background_opacity ?? cfg.backgroundOpacity ?? 0.35))) * 100,
  );
  const panelBlur = cfg.panel_blur ?? cfg.panelBlur ?? true;
  const session = formatSessionStatus(state, cfg);
  const dispatch = formatDispatchStatus(state, cfg);
  const versionText = String(cfg.bh_ver ?? cfg.bhVer ?? "").trim();
  const hasCachedAccessKey = Boolean(cfg.last_login_succ && Number(cfg.uid ?? 0) > 0 && String(cfg.access_key ?? "").trim());

  setStatus(
    elements.appDot,
    elements.appValue,
    appBootstrapped ? "ok" : "error",
    state.running ? "\u76d1\u542c\u4e2d" : "\u5f85\u673a",
  );
  setStatus(elements.sessionDot, elements.sessionValue, session.tone, session.text);
  setStatus(elements.dispatchDot, elements.dispatchValue, dispatch.tone, dispatch.text);
  setStatus(
    elements.gameDot,
    elements.gameValue,
    state.gamePathValid ? "ok" : "error",
    state.gamePathValid ? `\u5df2\u8bbe\u7f6e ${versionText}`.trim() : "\u672a\u8bbe\u7f6e",
  );

  elements.actionValue.textContent = formatActionValue(state.lastAction);
  elements.errorValue.textContent = formatErrorValue(state.lastError);
  elements.pathHintValue.hidden = !state.gamePathPrompt;
  elements.pathHintValue.textContent = state.gamePathPrompt || "";

  syncInputValue(elements.accountInput, cfg.account);
  // HI3UID / BILIHITOKEN from config (support multiple casing/keys)
  const hi3uidVal = cfg.HI3UID ?? cfg.hi3uid ?? cfg.hi3Uid ?? cfg["HI3UID"] ?? cfg["hi3uid"] ?? "";
  const biliHitokenVal = cfg.BILIHITOKEN ?? cfg.biliHitoken ?? cfg["BILIHITOKEN"] ?? cfg["bili-hitoken"] ?? cfg["biliHitoken"] ?? "";
  syncInputValue(elements.hi3uidInput, hi3uidVal);
  syncInputValue(elements.biliHitokenInput, biliHitokenVal);
  syncInputValue(elements.passwordInput, cfg.password);
  syncInputValue(elements.gamePathInput, gamePath);
  syncRangeValue(elements.backgroundOpacityInput, backgroundOpacity);
  elements.backgroundOpacityValue.textContent = `${backgroundOpacity}%`;
  elements.backgroundStatusValue.textContent = backgroundPath ? "\u5df2\u8bbe\u7f6e" : "\u672a\u8bbe\u7f6e";
  elements.resetBackgroundBtn.disabled = !backgroundPath;
  applySurfaceOpacity(backgroundOpacity);
  const blurEnabled = Boolean(panelBlur);
  if (elements.settingsSheet.hidden) {
    elements.panelBlurInput.checked = blurEnabled;
  }
  applyBlurEnabled(elements.panelBlurInput.checked);
  if (!backgroundPath) {
    elements.customBackground.hidden = true;
    elements.customBackground.style.backgroundImage = "";
  }
  if (elements.settingsSheet.hidden) {
    elements.clipCheckInput.checked = clipCheck;
    elements.autoClipInput.checked = autoClip;
    elements.autoCloseInput.checked = autoClose;
  }
  elements.launchGameBtn.disabled = !state.gamePathValid;
  elements.loginBtn.disabled = hasCachedAccessKey;
  elements.loginBtn.textContent = hasCachedAccessKey ? "\u5df2\u767b\u5f55" : "\u767b\u5f55";
  elements.loginBtn.classList.toggle("button-success", hasCachedAccessKey);
  elements.loginBtn.classList.toggle("button-accent", !hasCachedAccessKey);

  if (!elements.logList.childElementCount && Array.isArray(state.logs)) {
    renderLogSnapshot(state.logs);
  }

  syncCaptcha(state);

  if (!state.gamePathValid) {
    openSettings(true);
  } else if (settingsPinnedForPath) {
    closeSettings(true);
  }
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

elements.settingsBtn.addEventListener("click", () => openSettings(false));
elements.settingsCloseBtn.addEventListener("click", () => closeSettings(false));
elements.settingsBackdrop.addEventListener("click", () => closeSettings(false));

elements.browseGamePathBtn.addEventListener("click", async () => {
  const selected = await runTask(
    () => BrowseGamePath(),
    (value) => ({ selected: value || null }),
    "soft",
  );
  if (selected === null || !selected) {
    return;
  }
  elements.gamePathInput.value = selected;
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

// --- Auto-save helpers ---
// Auto-save removed to avoid interfering with captcha/login flow.
let saveTimers = {};

// Wire up inputs for live save
// Auto-save listeners removed.

elements.backgroundOpacityInput.addEventListener("input", () => {
  const percent = Number(elements.backgroundOpacityInput.value || "35");
  elements.backgroundOpacityValue.textContent = `${percent}%`;
  applySurfaceOpacity(percent);
});

elements.backgroundOpacityInput.addEventListener("change", async () => {
  const state = await runTask(
    () =>
      UpdateBackground(
        "",
        Number(elements.backgroundOpacityInput.value || "35") / 100,
      ),
    () => ({ opacity: `${elements.backgroundOpacityInput.value}%` }),
    "soft",
  );
  if (!state) {
    return;
  }
  renderState(state);
  await refreshBackground();
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
  renderState(state);
});

elements.loginBtn.addEventListener("click", async () => {
  const result = await runTask(
    () => Login(elements.accountInput.value, elements.passwordInput.value),
    (value) => value,
    "neutral",
  );
  if (result?.needsCaptcha) {
    openSettings(false);
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
  await runTask(() => ScanWindow(), (matched) => ({ matched }), "soft");
});

elements.captchaCloseBtn.addEventListener("click", () => {
  captchaDismissed = true;
  elements.captchaOverlay.hidden = true;
});

EventsOn("log", (entry) => {
  appendLog(entry);
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

Bootstrap()
  .then((state) => {
    appBootstrapped = true;
    renderState(state);
    refreshBackground().catch((error) => {
      showPayload(formatError(error), "error");
    });
    if (!state.gamePathValid) {
      showPayload(state.gamePathPrompt || "\u8bf7\u5148\u9009\u62e9\u5d29\u574f3\u6e38\u620f\u76ee\u5f55\u3002", "warn");
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
    setStatus(elements.appDot, elements.appValue, "error", "\u542f\u52a8\u5931\u8d25");
    showPayload(formatError(error), "error");
    openSettings(false);
  });
