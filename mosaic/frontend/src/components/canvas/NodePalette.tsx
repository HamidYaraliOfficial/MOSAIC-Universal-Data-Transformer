import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Blocks, Search } from "lucide-react";
import { api } from "../../services/api";

const CATEGORIES: Record<string, string[]> = {
  "Row & Column": ["selectColumns", "renameColumns", "filterRows", "sort", "deduplicate", "sampling"],
  Transform: ["typeCast", "fillMissingValues", "generateColumn", "stringTransform", "mapValues"],
  "Text & Structure": ["regexExtract", "splitColumn", "mergeColumn", "flattenJSON"],
  Combine: ["join", "union", "lookupEnrich"],
  Aggregate: ["groupByAggregate", "pivot", "unpivot"],
  Quality: ["validateSchema"],
};

export default function NodePalette({ onAdd }: { onAdd: (nodeType: string) => void }) {
  const { t } = useTranslation();
  const [available, setAvailable] = useState<string[]>([]);
  const [query, setQuery] = useState("");

  useEffect(() => {
    api
      .nodeTypes()
      .then(setAvailable)
      .catch(() => setAvailable(Object.values(CATEGORIES).flat()));
  }, []);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    const result: Record<string, string[]> = {};
    for (const [cat, types] of Object.entries(CATEGORIES)) {
      const list = types.filter((tp) => (available.length === 0 || available.includes(tp)) && tp.toLowerCase().includes(q));
      if (list.length) result[cat] = list;
    }
    return result;
  }, [query, available]);

  return (
    <div style={{ width: 200, borderInlineEnd: "1px solid var(--surface-border)", display: "flex", flexDirection: "column" }}>
      <div style={{ padding: 10, borderBottom: "1px solid var(--surface-border)" }}>
        <div style={{ display: "flex", alignItems: "center", gap: 6, marginBottom: 8 }}>
          <Blocks size={14} />
          <strong style={{ fontSize: 12.5 }}>{t("canvas.nodePalette")}</strong>
        </div>
        <div style={{ position: "relative" }}>
          <Search size={13} style={{ position: "absolute", insetInlineStart: 8, top: 8, opacity: 0.6 }} />
          <input
            className="mosaic-input"
            style={{ width: "100%", paddingInlineStart: 26 }}
            placeholder={t("canvas.search") ?? ""}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
        </div>
      </div>
      <div className="mosaic-scrollbar" style={{ overflowY: "auto", padding: 8 }}>
        {Object.entries(filtered).map(([cat, types]) => (
          <div key={cat} style={{ marginBottom: 10 }}>
            <div style={{ fontSize: 10.5, textTransform: "uppercase", letterSpacing: 0.4, color: "var(--text-secondary)", margin: "4px 6px" }}>
              {cat}
            </div>
            {types.map((tp) => (
              <button
                key={tp}
                draggable
                onDragStart={(e) => e.dataTransfer.setData("application/mosaic-node", tp)}
                onClick={() => onAdd(tp)}
                className="mosaic-btn-ghost"
                style={{ width: "100%", textAlign: "start", border: "none", marginBottom: 2, fontSize: 12 }}
              >
                {tp}
              </button>
            ))}
          </div>
        ))}
      </div>
    </div>
  );
}
