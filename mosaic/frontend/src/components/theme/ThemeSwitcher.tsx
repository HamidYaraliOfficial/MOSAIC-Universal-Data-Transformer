import { useTranslation } from "react-i18next";
import { Palette } from "lucide-react";
import { useMosaicStore, ThemeName } from "../../state/store";

const THEME_ORDER: ThemeName[] = ["windows11", "light", "dark", "amoled", "red", "blue"];

export default function ThemeSwitcher() {
  const { t } = useTranslation();
  const theme = useMosaicStore((s) => s.theme);
  const setTheme = useMosaicStore((s) => s.setTheme);

  return (
    <label style={{ display: "flex", alignItems: "center", gap: 6 }}>
      <Palette size={15} color="var(--text-secondary)" />
      <select
        aria-label={t("theme.label")}
        className="mosaic-input"
        value={theme}
        onChange={(e) => setTheme(e.target.value as ThemeName)}
      >
        {THEME_ORDER.map((name) => (
          <option key={name} value={name}>
            {t(`theme.${name}`)}
          </option>
        ))}
      </select>
    </label>
  );
}
