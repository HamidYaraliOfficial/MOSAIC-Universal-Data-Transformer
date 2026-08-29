import { useTranslation } from "react-i18next";
import { Settings2, AlertTriangle, Gauge } from "lucide-react";
import { useMosaicStore } from "../../state/store";

export default function Inspector() {
  const { t } = useTranslation();
  const nodes = useMosaicStore((s) => s.nodes);
  const selectedId = useMosaicStore((s) => s.selectedNodeId);
  const updateNodeConfig = useMosaicStore((s) => s.updateNodeConfig);
  const node = nodes.find((n) => n.id === selectedId);

  return (
    <aside
      className="mosaic-panel mosaic-scrollbar"
      style={{ width: 300, margin: 8, marginInlineStart: 4, display: "flex", flexDirection: "column", overflow: "hidden" }}
    >
      <div style={{ padding: 10, borderBottom: "1px solid var(--surface-border)", display: "flex", alignItems: "center", gap: 6 }}>
        <Settings2 size={14} />
        <strong style={{ fontSize: 12.5 }}>{t("inspector.title")}</strong>
      </div>

      {!node && (
        <div style={{ padding: 16, fontSize: 12.5, color: "var(--text-secondary)" }}>{t("inspector.noSelection")}</div>
      )}

      {node && (
        <div style={{ overflowY: "auto", padding: 12, display: "flex", flexDirection: "column", gap: 14 }}>
          <div>
            <div className="mosaic-badge">{node.type}</div>
            <div className="mosaic-mono ltr-force" style={{ fontSize: 11, color: "var(--text-secondary)", marginTop: 4 }}>
              {node.id}
            </div>
          </div>

          <section>
            <h4 style={{ fontSize: 11.5, textTransform: "uppercase", letterSpacing: 0.4, color: "var(--text-secondary)", margin: "0 0 6px" }}>
              {t("inspector.config")}
            </h4>
            <ConfigEditor
              config={node.config}
              onChange={(cfg) => updateNodeConfig(node.id, cfg)}
            />
          </section>

          <section>
            <h4 style={{ fontSize: 11.5, textTransform: "uppercase", letterSpacing: 0.4, color: "var(--text-secondary)", margin: "0 0 6px" }}>
              {t("inspector.onError")}
            </h4>
            <select
              className="mosaic-input"
              style={{ width: "100%" }}
              value={node.onError ?? "skip"}
              onChange={(e) => updateNodeConfig(node.id, { __onError: e.target.value })}
            >
              <option value="stop">{t("inspector.onErrorStop")}</option>
              <option value="skip">{t("inspector.onErrorSkip")}</option>
              <option value="collect">{t("inspector.onErrorCollect")}</option>
            </select>
          </section>

          <section>
            <h4 style={{ fontSize: 11.5, textTransform: "uppercase", letterSpacing: 0.4, color: "var(--text-secondary)", margin: "0 0 6px", display: "flex", alignItems: "center", gap: 4 }}>
              <Gauge size={12} /> {t("inspector.metrics")}
            </h4>
            <div style={{ fontSize: 12, color: "var(--text-secondary)" }}>
              {t("inspector.rowsIn")} / {t("inspector.rowsOut")} — {t("inspector.duration")}
            </div>
          </section>
        </div>
      )}
    </aside>
  );
}

// A dependency-free key/value editor for a node's JSON config object — good
// enough for scalar fields; array/object-valued fields (join keys,
// aggregation lists, etc.) are edited as raw JSON in the same table.
function ConfigEditor({
  config,
  onChange,
}: {
  config: Record<string, unknown>;
  onChange: (cfg: Record<string, unknown>) => void;
}) {
  const entries = Object.entries(config);

  function setValue(key: string, raw: string) {
    let value: unknown = raw;
    try {
      value = JSON.parse(raw);
    } catch {
      /* keep as plain string if it isn't valid JSON */
    }
    onChange({ [key]: value });
  }

  function addField() {
    const key = prompt("Config key name");
    if (key) onChange({ [key]: "" });
  }

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
      {entries.length === 0 && (
        <div style={{ fontSize: 12, color: "var(--text-secondary)" }}>
          <AlertTriangle size={12} style={{ verticalAlign: "-2px", marginInlineEnd: 4 }} />
          No configuration fields yet.
        </div>
      )}
      {entries.map(([key, value]) => (
        <label key={key} style={{ display: "flex", flexDirection: "column", gap: 3 }}>
          <span style={{ fontSize: 11.5, color: "var(--text-secondary)" }}>{key}</span>
          <input
            className="mosaic-input"
            defaultValue={typeof value === "string" ? value : JSON.stringify(value)}
            onBlur={(e) => setValue(key, e.target.value)}
          />
        </label>
      ))}
      <button className="mosaic-btn-ghost" onClick={addField}>
        + Add field
      </button>
    </div>
  );
}
