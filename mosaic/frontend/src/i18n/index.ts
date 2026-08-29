import i18n from "i18next";
import { initReactI18next } from "react-i18next";
import en from "./en";
import fa from "./fa";
import zh from "./zh";

export type SupportedLanguage = "en" | "fa" | "zh";

export const LANGUAGES: { code: SupportedLanguage; nativeName: string; dir: "ltr" | "rtl" }[] = [
  { code: "en", nativeName: "English", dir: "ltr" },
  { code: "fa", nativeName: "فارسی", dir: "rtl" },
  { code: "zh", nativeName: "中文", dir: "ltr" },
];

i18n.use(initReactI18next).init({
  resources: {
    en: { translation: en },
    fa: { translation: fa },
    zh: { translation: zh },
  },
  lng: "en",
  fallbackLng: "en",
  interpolation: { escapeValue: false },
});

// Keeps <html dir="rtl|ltr" lang="..."> in sync with the active language so
// every native browser text-direction behavior (scrollbars, selection,
// input caret) is correct — not just component-level CSS mirroring.
export function applyDocumentDirection(lang: SupportedLanguage) {
  const meta = LANGUAGES.find((l) => l.code === lang) ?? LANGUAGES[0];
  document.documentElement.dir = meta.dir;
  document.documentElement.lang = lang;
}

export default i18n;
