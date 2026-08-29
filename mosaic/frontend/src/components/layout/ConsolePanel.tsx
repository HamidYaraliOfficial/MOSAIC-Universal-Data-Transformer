import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Table2, Terminal, ShieldCheck } from "lucide-react";
import { useMosaicStore } from "../../state/store";
import VirtualizedTable from "../preview/VirtualizedTable";
import QualityScoreCard from "../quality/QualityScoreCard";
import { api } from "../../services/api";
import type { QualityReport } from "../../services/types";

type Tab = "preview" | "log" | "quality";

export default function ConsolePanel() {
  const { t } = useTranslation();
  const [tab, setTab] = useState<Tab>("preview");
  const dataset = useMosaicStore((s) => s.activeDataset);
  const [quality, setQuality] = useState<QualityReport | null>(null);
  const [logs] = useState<string[]>([
    "MOSAIC engine ready.",
    "Waiting for a pipeline run or import…",
  ]);

  async function loadQuality() {
    if (!dataset?.report) return;
    const r = await api.scoreQuality(dataset.report.schema, dataset.rows);
    setQuality(r);
  }

  return (
    <div
      className="mosaic-panel"
      style={{
        gridColumn: "1 / -1",
        margin: 8,
        marginTop: 4,
        height: 220,
        display: "flex",
        flexDirection: "column",
        overflow: "hidden",
      }}
    >
      <div style={{ display: "flex", gap: 4, padding: 6, borderBottom: "1px solid var(--surface-border)" }}>
        <TabButton icon={Table2} label={t("console.dataPreview")} active={tab === "preview"} onClick={() => setTab("preview")} />
        <TabButton icon={Terminal} label={t("console.log")} active={tab === "log"} onClick={() => setTab("log")} />
        <TabButton
          icon={ShieldCheck}
          label={t("console.qualityScore")}
          active={tab === "quality"}
          onClick={() => {
            setTab("quality");
            loadQuality();
          }}
        />
      </div>

      <div style={{ flex: 1, overflow: "hidden" }}>
        {tab === "preview" &&
          (dataset ? (
            <VirtualizedTable columns={dataset.columns} rows={dataset.rows} />
          ) : (
            <EmptyState text={t("startCenter.noRecents")} />
          ))}
        {tab === "log" && (
          <div className="mosaic-mono ltr-force mosaic-scrollbar" style={{ padding: 10, fontSize: 11.5, overflowY: "auto", height: "100%" }}>
            {logs.map((l, i) => (
              <div key={i} style={{ color: "var(--text-secondary)" }}>
                [{new Date().toLocaleTimeString()}] {l}
              </div>
            ))}
          </div>
        )}
        {tab === "quality" && (quality ? <QualityScoreCard report={quality} /> : <EmptyState text={t("startCenter.noRecents")} />)}
      </div>
    </div>
  );
}

function TabButton({
  icon: Icon,
  label,
  active,
  onClick,
}: {
  icon: typeof Table2;
  label: string;
  active: boolean;
  onClick: () => void;
}) {
  return (
    <button
      onClick={onClick}
      className="mosaic-btn-ghost"
      style={{
        border: "none",
        display: "flex",
        alignItems: "center",
        gap: 6,
        background: active ? "var(--accent-soft)" : "transparent",
        color: active ? "var(--accent)" : "var(--text-primary)",
      }}
    >
      <Icon size={13} />
      {label}
    </button>
  );
}

function EmptyState({ text }: { text: string }) {
  return (
    <div style={{ display: "flex", alignItems: "center", justifyContent: "center", height: "100%", color: "var(--text-secondary)", fontSize: 12.5 }}>
      {text}
    </div>
  );
}
