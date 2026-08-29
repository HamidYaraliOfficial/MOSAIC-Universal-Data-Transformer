import { useTranslation } from "react-i18next";
import { Search, Boxes, LayoutGrid, Clock, FolderKanban } from "lucide-react";
import { useMosaicStore, ViewName } from "../../state/store";
import ThemeSwitcher from "../theme/ThemeSwitcher";
import LanguageSwitcher from "../theme/LanguageSwitcher";

const NAV: { key: ViewName; icon: typeof LayoutGrid }[] = [
  { key: "start", icon: LayoutGrid },
  { key: "canvas", icon: Boxes },
  { key: "scheduler", icon: Clock },
  { key: "projects", icon: FolderKanban },
];

export default function TitleBar() {
  const { t } = useTranslation();
  const view = useMosaicStore((s) => s.view);
  const setView = useMosaicStore((s) => s.setView);
  const toggleCommandPalette = useMosaicStore((s) => s.toggleCommandPalette);

  return (
    <header className="mosaic-titlebar">
      <strong style={{ fontSize: 14, letterSpacing: 0.3 }}>{t("appName")}</strong>
      <span style={{ color: "var(--text-secondary)", fontSize: 12 }}>{t("tagline")}</span>

      <nav style={{ display: "flex", gap: 4, marginInlineStart: 16 }}>
        {NAV.map(({ key, icon: Icon }) => (
          <button
            key={key}
            onClick={() => setView(key)}
            className="mosaic-btn-ghost"
            style={{
              display: "flex",
              alignItems: "center",
              gap: 6,
              border: "none",
              background: view === key ? "var(--accent-soft)" : "transparent",
              color: view === key ? "var(--accent)" : "var(--text-primary)",
            }}
          >
            <Icon size={14} />
            {t(`nav.${key}`)}
          </button>
        ))}
      </nav>

      <div style={{ flex: 1 }} />

      <button className="mosaic-btn-ghost" onClick={() => toggleCommandPalette(true)} style={{ display: "flex", alignItems: "center", gap: 6 }}>
        <Search size={13} />
        <span className="mosaic-mono ltr-force" style={{ fontSize: 11, opacity: 0.7 }}>
          Ctrl+K
        </span>
      </button>
      <ThemeSwitcher />
      <LanguageSwitcher />
    </header>
  );
}
