import builtInZhCN from "./locales/zh-CN.json";
import builtInEnUS from "./locales/en-US.json";

import { LoadLocaleMessages } from "../wailsjs/go/main/App";

const STORAGE_KEY = "hi3loader.locale";
const DEFAULT_LOCALE = "zh-CN";
const FALLBACK_LOCALE = "en-US";

const builtInCatalog = Object.freeze({
  "zh-CN": builtInZhCN,
  "en-US": builtInEnUS,
});

const messages = Object.fromEntries(
  Object.entries(builtInCatalog).map(([locale, entries]) => [locale, { ...entries }]),
);

function normalizeLocale(locale) {
  const candidate = String(locale || "").trim();
  if (messages[candidate]) {
    return candidate;
  }
  if (candidate.toLowerCase().startsWith("zh")) {
    return "zh-CN";
  }
  if (candidate.toLowerCase().startsWith("en")) {
    return "en-US";
  }
  return candidate;
}

function detectLocale() {
  try {
    const stored = normalizeLocale(window.localStorage.getItem(STORAGE_KEY));
    if (stored) {
      return stored;
    }
  } catch (_) {
  }

  if (typeof navigator !== "undefined") {
    const detected = normalizeLocale(navigator.language || navigator.userLanguage);
    if (detected) {
      return detected;
    }
  }

  return DEFAULT_LOCALE;
}

function normalizeMessageEntries(entries) {
  if (!entries || typeof entries !== "object") {
    return {};
  }

  return Object.fromEntries(
    Object.entries(entries)
      .filter(([key]) => String(key || "").trim() !== "")
      .map(([key, value]) => [String(key), String(value ?? "")]),
  );
}

function mergeCatalog(catalog) {
  if (!catalog || typeof catalog !== "object") {
    return;
  }

  Object.entries(catalog).forEach(([locale, entries]) => {
    const normalizedLocale = normalizeLocale(locale);
    if (!normalizedLocale) {
      return;
    }
    if (!messages[normalizedLocale]) {
      messages[normalizedLocale] = {};
    }
    Object.assign(messages[normalizedLocale], normalizeMessageEntries(entries));
  });
}

let currentLocale = detectLocale();
let initPromise = null;

function exposeGlobal() {
  if (typeof window === "undefined") {
    return;
  }
  window.hi3loaderI18n = {
    getLocale,
    setLocale,
    applyLocale,
    listLocales,
    initI18n,
    t,
    translateMessage,
  };
}

export async function initI18n() {
  if (!initPromise) {
    initPromise = (async () => {
      try {
        const externalCatalog = await LoadLocaleMessages();
        mergeCatalog(externalCatalog);
      } catch (_) {
      }

      const normalizedLocale = normalizeLocale(currentLocale);
      currentLocale = normalizedLocale || DEFAULT_LOCALE;
      exposeGlobal();
      return messages;
    })();
  }
  return initPromise;
}

export function getLocale() {
  return currentLocale;
}

export function setLocale(locale) {
  const normalized = normalizeLocale(locale);
  if (!normalized) {
    return false;
  }
  currentLocale = normalized;
  try {
    window.localStorage.setItem(STORAGE_KEY, normalized);
  } catch (_) {
  }
  exposeGlobal();
  return true;
}

export function applyLocale(locale) {
  if (!setLocale(locale)) {
    return false;
  }
  if (typeof window !== "undefined" && window.location) {
    window.location.reload();
  }
  return true;
}

export function listLocales() {
  return Object.keys(messages).sort();
}

export function t(key, vars = {}) {
  const template =
    messages[currentLocale]?.[key] ??
    messages[DEFAULT_LOCALE]?.[key] ??
    messages[FALLBACK_LOCALE]?.[key] ??
    key;

  return String(template).replace(/\{(\w+)\}/g, (_, name) => String(vars[name] ?? ""));
}

export function translateMessage(ref, fallback = "") {
  const code = String(ref?.code ?? "").trim();
  if (!code) {
    return String(fallback ?? "");
  }
  return t(code, ref?.params ?? {});
}

exposeGlobal();
