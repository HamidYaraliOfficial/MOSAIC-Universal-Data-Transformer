import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Star, Search, FileStack } from "lucide-react";
import { api } from "../../services/api";
import type { Project } from "../../services/types";

export default function ProjectExplorer() {
  const { t } = useTranslation();
  const [projects, setProjects] = useState<Project[]>([]);
  const [query, setQuery] = useState("");
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api
      .listProjects()
      .then(setProjects)
      .catch(() => setProjects([]))
      .finally(() => setLoading(false));
  }, []);

  const filtered = projects.filter((p) => p.name.toLowerCase().includes(query.toLowerCase()));

  return (
    <aside
      className="mosaic-panel mosaic-scrollbar"
      style={{ width: 240, margin: 8, marginInlineEnd: 4, display: "flex", flexDirection: "column", overflow: "hidden" }}
    >
      <div style={{ padding: 10, borderBottom: "1px solid var(--surface-border)" }}>
        <div style={{ display: "flex", alignItems: "center", gap: 6, marginBottom: 8 }}>
          <FileStack size={14} />
          <strong style={{ fontSize: 12.5 }}>{t("nav.projects")}</strong>
        </div>
        <div style={{ position: "relative" }}>
          <Search size={13} style={{ position: "absolute", insetInlineStart: 8, top: 8, opacity: 0.6 }} />
          <input
            className="mosaic-input"
            style={{ width: "100%", paddingInlineStart: 26 }}
            placeholder={t("commandPalette.placeholder") ?? ""}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
        </div>
      </div>

      <div style={{ overflowY: "auto", flex: 1, padding: 6 }}>
        {loading && <div style={{ padding: 12, fontSize: 12, color: "var(--text-secondary)" }}>…</div>}
        {!loading && filtered.length === 0 && (
          <div style={{ padding: 12, fontSize: 12, color: "var(--text-secondary)" }}>{t("startCenter.noRecents")}</div>
        )}
        {filtered.map((p) => (
          <button
            key={p.id}
            className="mosaic-btn-ghost"
            style={{
              width: "100%",
              textAlign: "start",
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              border: "none",
              marginBottom: 2,
            }}
          >
            <span style={{ fontSize: 12.5, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{p.name}</span>
            {p.favorite && <Star size={12} fill="var(--accent)" color="var(--accent)" />}
          </button>
        ))}
      </div>
    </aside>
  );
}
