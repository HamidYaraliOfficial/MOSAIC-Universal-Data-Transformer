import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Clock, CalendarClock, Timer } from "lucide-react";
import { api } from "../../services/api";
import type { Schedule, SchedulerStatus, Window } from "../../services/types";

const DAYS = ["monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"] as const;
const PIPELINE_ID = "pipeline_main"; // the currently open pipeline; swappable once multi-pipeline tabs exist

function minutesToTime(min: number): string {
  const h = Math.floor(min / 60) % 24;
  const m = min % 60;
  return `${String(h).padStart(2, "0")}:${String(m).padStart(2, "0")}`;
}
function timeToMinutes(time: string): number {
  const [h, m] = time.split(":").map(Number);
  return (h || 0) * 60 + (m || 0);
}
function formatDuration(nanoseconds: number): string {
  const totalSeconds = Math.max(0, Math.round(nanoseconds / 1e9));
  const h = Math.floor(totalSeconds / 3600);
  const m = Math.floor((totalSeconds % 3600) / 60);
  const s = totalSeconds % 60;
  if (h > 0) return `${h}h ${m}m`;
  if (m > 0) return `${m}m ${s}s`;
  return `${s}s`;
}

export default function SchedulerPanel() {
  const { t } = useTranslation();
  const [enabled, setEnabled] = useState(false);
  const [timezone, setTimezone] = useState(() => Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC");
  const [days, setDays] = useState<Record<string, Window>>({});
  const [status, setStatus] = useState<SchedulerStatus | null>(null);
  const [tick, setTick] = useState(0);

  // Live countdown: re-render every second so "time until next window"
  // counts down smoothly between the periodic server re-syncs below.
  useEffect(() => {
    const id = setInterval(() => setTick((n) => n + 1), 1000);
    return () => clearInterval(id);
  }, []);

  async function refreshStatus() {
    try {
      const s = await api.getSchedulerStatus(PIPELINE_ID);
      setStatus(s);
    } catch {
      setStatus(null);
    }
  }

  useEffect(() => {
    refreshStatus();
    const id = setInterval(refreshStatus, 30_000);
    return () => clearInterval(id);
  }, []);

  function toggleDay(day: string, on: boolean) {
    setDays((prev) => {
      const next = { ...prev };
      if (on) next[day] = next[day] ?? { startMinute: 9 * 60, endMinute: 18 * 60 };
      else delete next[day];
      return next;
    });
  }

  function updateWindow(day: string, field: "startMinute" | "endMinute", time: string) {
    setDays((prev) => ({ ...prev, [day]: { ...prev[day], [field]: timeToMinutes(time) } }));
  }

  async function save() {
    const schedule: Schedule = { pipelineId: PIPELINE_ID, timezone, days, enabled };
    await api.setSchedule(schedule);
    await refreshStatus();
  }

  // Client-side smooth countdown between server syncs: subtract elapsed
  // wall-clock time since the last status fetch from the server-reported
  // remaining duration, rather than only updating once every 30s.
  const liveTimeUntilNext = status?.nextOpenAt
    ? Math.max(0, new Date(status.nextOpenAt).getTime() - Date.now()) * 1e6
    : status?.timeUntilNext ?? 0;
  void tick; // referenced to force a re-render each second for the countdown above

  return (
    <div style={{ flex: 1, overflowY: "auto", padding: 20, maxWidth: 760, margin: "0 auto", width: "100%" }}>
      <header style={{ marginBottom: 18 }}>
        <h2 style={{ display: "flex", alignItems: "center", gap: 8, margin: 0, fontSize: 18 }}>
          <Clock size={18} /> {t("scheduler.title")}
        </h2>
        <p style={{ color: "var(--text-secondary)", fontSize: 13, maxWidth: 620 }}>{t("scheduler.subtitle")}</p>
      </header>

      <StatusCard status={status} liveTimeUntilNext={liveTimeUntilNext} />

      <section className="mosaic-panel" style={{ padding: 16, marginTop: 16 }}>
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 14 }}>
          <label style={{ display: "flex", alignItems: "center", gap: 8, fontSize: 13 }}>
            <input type="checkbox" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} />
            {t("scheduler.enabled")}
          </label>
          <label style={{ display: "flex", alignItems: "center", gap: 8, fontSize: 13 }}>
            {t("scheduler.timezone")}
            <input className="mosaic-input" value={timezone} onChange={(e) => setTimezone(e.target.value)} style={{ width: 220 }} />
          </label>
        </div>

        <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
          {DAYS.map((day) => {
            const w = days[day];
            return (
              <div key={day} style={{ display: "flex", alignItems: "center", gap: 12, padding: "6px 0", borderBottom: "1px solid var(--surface-border)" }}>
                <label style={{ display: "flex", alignItems: "center", gap: 8, width: 160, fontSize: 13 }}>
                  <input type="checkbox" checked={!!w} onChange={(e) => toggleDay(day, e.target.checked)} />
                  {t(`scheduler.days.${day}`)}
                </label>
                {w ? (
                  <>
                    <span style={{ fontSize: 12, color: "var(--text-secondary)" }}>{t("scheduler.from")}</span>
                    <input
                      type="time"
                      className="mosaic-input ltr-force"
                      value={minutesToTime(w.startMinute)}
                      onChange={(e) => updateWindow(day, "startMinute", e.target.value)}
                    />
                    <span style={{ fontSize: 12, color: "var(--text-secondary)" }}>{t("scheduler.to")}</span>
                    <input
                      type="time"
                      className="mosaic-input ltr-force"
                      value={minutesToTime(w.endMinute)}
                      onChange={(e) => updateWindow(day, "endMinute", e.target.value)}
                    />
                  </>
                ) : (
                  <span style={{ fontSize: 12, color: "var(--text-secondary)" }}>{t("scheduler.statusClosed")}</span>
                )}
              </div>
            );
          })}
        </div>

        <button className="mosaic-btn-primary" onClick={save} style={{ marginTop: 16 }}>
          {t("canvas.save")}
        </button>
      </section>
    </div>
  );
}

function StatusCard({ status, liveTimeUntilNext }: { status: SchedulerStatus | null; liveTimeUntilNext: number }) {
  const { t } = useTranslation();
  if (!status) {
    return (
      <div className="mosaic-panel" style={{ padding: 16, display: "flex", alignItems: "center", gap: 10, color: "var(--text-secondary)", fontSize: 13 }}>
        <CalendarClock size={16} />
        {t("scheduler.noHistoryYet")}
      </div>
    );
  }
  return (
    <div className="mosaic-panel" style={{ padding: 16, display: "flex", gap: 24, flexWrap: "wrap" }}>
      <div>
        <div
          className="mosaic-badge"
          style={{
            background: status.isOpenNow ? "color-mix(in srgb, var(--success) 18%, transparent)" : "var(--surface-2)",
            color: status.isOpenNow ? "var(--success)" : "var(--text-secondary)",
          }}
        >
          {status.isOpenNow ? t("scheduler.statusOpen") : t("scheduler.statusClosed")}
        </div>
        {status.isOpenNow && status.currentWindowEnd && (
          <div style={{ fontSize: 12, color: "var(--text-secondary)", marginTop: 6 }}>
            {t("scheduler.closesAt", { time: new Date(status.currentWindowEnd).toLocaleTimeString() })}
          </div>
        )}
        {!status.isOpenNow && status.nextOpenAt && (
          <div style={{ fontSize: 12, color: "var(--text-secondary)", marginTop: 6 }}>
            {t("scheduler.nextOpenAt", { time: new Date(status.nextOpenAt).toLocaleString() })}
          </div>
        )}
      </div>

      {!status.isOpenNow && (
        <div>
          <div style={{ fontSize: 11, color: "var(--text-secondary)", display: "flex", alignItems: "center", gap: 5 }}>
            <Timer size={12} /> {t("scheduler.timeUntilNext")}
          </div>
          <div className="mosaic-mono ltr-force" style={{ fontSize: 18, fontWeight: 700 }}>
            {formatDuration(liveTimeUntilNext)}
          </div>
        </div>
      )}

      <div>
        <div style={{ fontSize: 11, color: "var(--text-secondary)" }}>{t("scheduler.estimatedDuration")}</div>
        <div className="mosaic-mono ltr-force" style={{ fontSize: 18, fontWeight: 700 }}>
          {status.sampleSize > 0 ? formatDuration(status.estimatedRunTime) : "—"}
        </div>
        <div style={{ fontSize: 11, color: "var(--text-secondary)" }}>
          {status.sampleSize > 0 ? t("scheduler.basedOnRuns", { count: status.sampleSize }) : t("scheduler.noHistoryYet")}
        </div>
      </div>
    </div>
  );
}
