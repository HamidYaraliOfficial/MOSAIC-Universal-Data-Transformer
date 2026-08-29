import { useTranslation } from "react-i18next";
import { Languages } from "lucide-react";
import { LANGUAGES, applyDocumentDirection, SupportedLanguage } from "../../i18n";
import { useMosaicStore } from "../../state/store";

export default function LanguageSwitcher() {
  const { t, i18n } = useTranslation();
  const language = useMosaicStore((s) => s.language);
  const setLanguage = useMosaicStore((s) => s.setLanguage);

  function onChange(lang: SupportedLanguage) {
    setLanguage(lang);
    i18n.changeLanguage(lang);
    applyDocumentDirection(lang);
  }

  return (
    <label style={{ display: "flex", alignItems: "center", gap: 6 }}>
      <Languages size={15} color="var(--text-secondary)" />
      <select
        aria-label={t("language.label")}
        className="mosaic-input"
        value={language}
        onChange={(e) => onChange(e.target.value as SupportedLanguage)}
      >
        {LANGUAGES.map((l) => (
          <option key={l.code} value={l.code}>
            {l.nativeName}
          </option>
        ))}
      </select>
    </label>
  );
}
