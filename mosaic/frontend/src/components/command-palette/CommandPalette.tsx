import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Search } from "lucide-react";
import { useMosaicStore } from "../../state/store";

interface Command {
  id: string;
  label: string;
  group: "nodes" | "actions";
  run: () => void;
}

export default function CommandPalette() {
  const { t } = useTranslation();
  const open = useMosaicStore((s) => s.commandPaletteOpen);
  const toggle = useMosaicStore((s) => s.toggleCommandPalette);
  const setView = useMosaicStore((s) => s.setView);
  const [query, setQuery] = useState("");

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        toggle();
      }
      if (e.key === "Escape") toggle(false);
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [toggle]);

  const commands: Command[] = useMemo(
    () => [
      { id: "goto-start", label: t("nav.startCenter"), group: "actions", run: () => setView("start") },
      { id: "goto-canvas", label: t("nav.canvas"), group: "actions", run: () => setView("canvas") },
      { id: "goto-scheduler", label: t("nav.scheduler"), group: "actions", run: () => setView("scheduler") },
      { id: "goto-projects", label: t("nav.projects"), group: "actions", run: () => setView("projects") },
    ],
    [t, setView]
  );

  const filtered = commands.filter((c) => c.label.toLowerCase().includes(query.toLowerCase()));

  if (!open) return null;

  return (
    <div
      role="dialog"
      aria-modal="true"
      onClick={() => toggle(false)}
      style={{
        position: "fixed",
        inset: 0,
        background: "rgba(0,0,0,0.4)",
        display: "flex",
        alignItems: "flex-start",
        justifyContent: "center",
        paddingTop: "12vh",
        zIndex: 1000,
      }}
    >
      <div
        onClick={(e) => e.stopPropagation()}
        className="mosaic-panel"
        style={{ width: 520, maxWidth: "90vw", background: "var(--surface-2)", overflow: "hidden" }}
      >
        <div style={{ display: "flex", alignItems: "center", gap: 8, padding: 12, borderBottom: "1px solid var(--surface-border)" }}>
          <Search size={15} />
          <input
            autoFocus
            className="mosaic-input"
            style={{ flex: 1, border: "none", background: "transparent" }}
            placeholder={t("commandPalette.placeholder") ?? ""}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
        </div>
        <div style={{ maxHeight: 320, overflowY: "auto" }}>
          {filtered.length === 0 && (
            <div style={{ padding: 16, fontSize: 12.5, color: "var(--text-secondary)" }}>{t("commandPalette.noResults")}</div>
          )}
          {filtered.map((c) => (
            <button
              key={c.id}
              onClick={() => {
                c.run();
                toggle(false);
              }}
              className="mosaic-btn-ghost"
              style={{ width: "100%", textAlign: "start", border: "none", borderRadius: 0, padding: "10px 14px" }}
            >
              {c.label}
            </button>
          ))}
        </div>
      </div>
    </div>
  );
}
