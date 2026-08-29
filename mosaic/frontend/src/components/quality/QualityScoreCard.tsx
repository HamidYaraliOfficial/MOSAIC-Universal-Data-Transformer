import { useTranslation } from "react-i18next";
import type { QualityReport } from "../../services/types";

const DIMENSIONS: (keyof Pick<QualityReport, "completeness" | "validity" | "uniqueness" | "consistency" | "accuracy">)[] = [
  "completeness",
  "validity",
  "uniqueness",
  "consistency",
  "accuracy",
];

export default function QualityScoreCard({ report }: { report: QualityReport }) {
  const { t } = useTranslation();
  const pct = (v: number) => `${Math.round(v * 100)}%`;
  const scoreColor = (v: number) => (v > 0.85 ? "var(--success)" : v > 0.6 ? "var(--warning)" : "var(--danger)");

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 10, padding: 12 }}>
      <div style={{ display: "flex", alignItems: "baseline", gap: 8 }}>
        <span style={{ fontSize: 26, fontWeight: 700, color: scoreColor(report.overall) }}>{pct(report.overall)}</span>
        <span style={{ fontSize: 12, color: "var(--text-secondary)" }}>{t("quality.overall")}</span>
      </div>

      <div style={{ display: "grid", gridTemplateColumns: "repeat(5, 1fr)", gap: 8 }}>
        {DIMENSIONS.map((dim) => (
          <div key={dim}>
            <div style={{ fontSize: 10.5, color: "var(--text-secondary)", marginBottom: 3 }}>{t(`quality.${dim}`)}</div>
            <div style={{ height: 6, borderRadius: 3, background: "var(--surface-2)", overflow: "hidden" }}>
              <div style={{ width: pct(report[dim]), height: "100%", background: scoreColor(report[dim]) }} />
            </div>
            <div style={{ fontSize: 11, marginTop: 3 }}>{pct(report[dim])}</div>
          </div>
        ))}
      </div>

      {report.issues?.length > 0 && (
        <ul className="mosaic-scrollbar" style={{ listStyle: "none", margin: 0, padding: 0, maxHeight: 120, overflowY: "auto" }}>
          {report.issues.slice(0, 30).map((issue, i) => (
            <li
              key={i}
              style={{
                display: "flex",
                justifyContent: "space-between",
                gap: 8,
                fontSize: 11.5,
                padding: "4px 0",
                borderBottom: "1px solid var(--surface-border)",
              }}
            >
              <span>
                <strong>{issue.column}</strong> — {issue.message}
              </span>
              <span
                className="mosaic-badge"
                style={{
                  color: issue.severity === "critical" ? "var(--danger)" : issue.severity === "warning" ? "var(--warning)" : "var(--text-secondary)",
                  background: "transparent",
                }}
              >
                {issue.severity}
              </span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
