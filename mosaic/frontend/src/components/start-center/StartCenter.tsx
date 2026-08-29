import { useRef } from "react";
import { useTranslation } from "react-i18next";
import { FilePlus2, UploadCloud, FileJson2, FileSpreadsheet, FileCode2, ScrollText, Database, Sparkles } from "lucide-react";
import { useMosaicStore } from "../../state/store";
import { api } from "../../services/api";

const TEMPLATE_ICONS: Record<string, typeof FileJson2> = {
  csvToJson: FileJson2,
  excelToSql: FileSpreadsheet,
  xmlToJson: FileCode2,
  logToCsv: ScrollText,
  jsonToExcel: FileSpreadsheet,
  dataCleaning: Sparkles,
  etl: Database,
  dbImportExport: Database,
};

export default function StartCenter() {
  const { t } = useTranslation();
  const setView = useMosaicStore((s) => s.setView);
  const setActiveDataset = useMosaicStore((s) => s.setActiveDataset);
  const fileInputRef = useRef<HTMLInputElement>(null);

  async function handleFile(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    try {
      const result = await api.importFile(file);
      setActiveDataset(result.report.columns, result.report.sampleRows, result.report);
      setView("canvas");
    } catch (err) {
      console.error(err);
      alert(String(err));
    } finally {
      e.target.value = "";
    }
  }

  return (
    <div style={{ flex: 1, overflowY: "auto", padding: "32px 24px", maxWidth: 980, margin: "0 auto", width: "100%" }}>
      <div style={{ textAlign: "center", marginBottom: 32 }}>
        <h1 style={{ fontSize: 28, margin: "0 0 6px", letterSpacing: -0.5 }}>{t("appName")}</h1>
        <p style={{ color: "var(--text-secondary)", margin: 0 }}>{t("tagline")}</p>
      </div>

      <div style={{ display: "flex", gap: 12, justifyContent: "center", marginBottom: 36 }}>
        <button className="mosaic-btn-primary" onClick={() => setView("canvas")} style={{ display: "flex", alignItems: "center", gap: 8, padding: "10px 18px" }}>
          <FilePlus2 size={16} /> {t("startCenter.newPipeline")}
        </button>
        <button
          className="mosaic-btn-ghost"
          onClick={() => fileInputRef.current?.click()}
          style={{ display: "flex", alignItems: "center", gap: 8, padding: "10px 18px" }}
        >
          <UploadCloud size={16} /> {t("startCenter.importData")}
        </button>
        <input ref={fileInputRef} type="file" hidden onChange={handleFile} />
      </div>

      <section style={{ marginBottom: 32 }}>
        <h3 style={{ fontSize: 13, textTransform: "uppercase", letterSpacing: 0.5, color: "var(--text-secondary)" }}>
          {t("startCenter.templates")}
        </h3>
        <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(200px, 1fr))", gap: 10 }}>
          {Object.entries(TEMPLATE_ICONS).map(([key, Icon]) => (
            <button
              key={key}
              className="mosaic-panel"
              onClick={() => setView("canvas")}
              style={{
                display: "flex",
                alignItems: "center",
                gap: 10,
                padding: 14,
                textAlign: "start",
                border: "1px solid var(--surface-border)",
                background: "var(--surface-1)",
              }}
            >
              <Icon size={18} color="var(--accent)" />
              <span style={{ fontSize: 13 }}>{t(`templates.${key}`)}</span>
            </button>
          ))}
        </div>
      </section>

      <section>
        <h3 style={{ fontSize: 13, textTransform: "uppercase", letterSpacing: 0.5, color: "var(--text-secondary)" }}>
          {t("startCenter.recentProjects")}
        </h3>
        <div className="mosaic-panel" style={{ padding: 24, textAlign: "center", color: "var(--text-secondary)", fontSize: 13 }}>
          {t("startCenter.noRecents")}
        </div>
      </section>
    </div>
  );
}
