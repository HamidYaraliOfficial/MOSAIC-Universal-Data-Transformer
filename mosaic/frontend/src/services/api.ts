// This is the ONLY file in the frontend that talks to the outside world.
// Every data operation — parsing, transforming, exporting, scheduling — is
// a call into the Go engine's local HTTP bridge. Nothing here does data
// processing itself; that split is intentional (see project brief: "Go
// مسئول تمام پردازش سنگین داده, TypeScript فقط مسئول UI").
import type {
  ImportResponse,
  PipelineDefinition,
  Job,
  QualityReport,
  Project,
  Schedule,
  SchedulerStatus,
} from "./types";

let cachedPort: number | null = null;

async function resolveEnginePort(): Promise<number> {
  if (cachedPort) return cachedPort;

  // Running inside the Tauri shell: ask the Rust side for the port the Go
  // sidecar bound to (see src-tauri/src/main.rs).
  try {
    const isTauri = "__TAURI_INTERNALS__" in window || "__TAURI__" in window;
    if (isTauri) {
      const { invoke } = await import("@tauri-apps/api/core");
      const port = await invoke<number | null>("get_engine_port");
      if (port) {
        cachedPort = port;
        return port;
      }
      // Not ready yet: wait for the one-shot "engine-ready" event.
      const { listen } = await import("@tauri-apps/api/event");
      const port2 = await new Promise<number>((resolve) => {
        listen<number>("engine-ready", (e) => resolve(e.payload));
      });
      cachedPort = port2;
      return port2;
    }
  } catch {
    // Fall through to the dev-server default below.
  }

  // `npm run dev` in a plain browser (no Tauri shell): fall back to the
  // engine's default dev port, overridable via VITE_ENGINE_PORT.
  const fallback = Number(import.meta.env.VITE_ENGINE_PORT ?? 8787);
  cachedPort = fallback;
  return fallback;
}

async function apiBase(): Promise<string> {
  const port = await resolveEnginePort();
  return `http://127.0.0.1:${port}`;
}

async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const base = await apiBase();
  const res = await fetch(`${base}${path}`, {
    ...init,
    headers: { "Content-Type": "application/json", ...(init?.headers ?? {}) },
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(body.error ?? `Request to ${path} failed with ${res.status}`);
  }
  return res.json() as Promise<T>;
}

export const api = {
  health: () => apiFetch<{ status: string; engine: string }>("/api/health"),
  formats: () => apiFetch<string[]>("/api/formats"),
  nodeTypes: () => apiFetch<string[]>("/api/nodes"),

  importFile: async (file: File, format?: string): Promise<ImportResponse> => {
    const base = await apiBase();
    const form = new FormData();
    form.append("file", file);
    if (format) form.append("format", format);
    const res = await fetch(`${base}/api/import`, { method: "POST", body: form });
    if (!res.ok) throw new Error((await res.json().catch(() => ({}))).error ?? "Import failed");
    return res.json();
  },

  validateExpression: (expression: string) =>
    apiFetch<{ valid: boolean; error?: string }>("/api/expression/validate", {
      method: "POST",
      body: JSON.stringify({ expression }),
    }),

  runPipeline: (definition: PipelineDefinition, sources: Record<string, unknown[]>) =>
    apiFetch<{ jobId: string }>("/api/pipeline/run", {
      method: "POST",
      body: JSON.stringify({ definition, sources }),
    }),

  getJob: (id: string) => apiFetch<Job>(`/api/jobs/${id}`),
  pauseJob: (id: string) => apiFetch<Job>(`/api/jobs/${id}/pause`, { method: "POST" }),
  resumeJob: (id: string) => apiFetch<Job>(`/api/jobs/${id}/resume`, { method: "POST" }),
  cancelJob: (id: string) => apiFetch<Job>(`/api/jobs/${id}/cancel`, { method: "POST" }),

  streamJob: async (id: string, onUpdate: (job: Job) => void): Promise<() => void> => {
    const base = await apiBase();
    const source = new EventSource(`${base}/api/jobs/${id}/stream`);
    source.onmessage = (e) => onUpdate(JSON.parse(e.data));
    return () => source.close();
  },

  scoreQuality: (schemaDef: unknown, rows: unknown[]) =>
    apiFetch<QualityReport>("/api/quality/score", {
      method: "POST",
      body: JSON.stringify({ schema: schemaDef, rows }),
    }),

  listProjects: () => apiFetch<Project[]>("/api/projects"),
  saveProject: (project: Project) =>
    apiFetch<Project>("/api/projects", { method: "POST", body: JSON.stringify(project) }),
  loadProject: (id: string) => apiFetch<Project>(`/api/projects/${id}`),

  setSchedule: (schedule: Schedule) =>
    apiFetch<Schedule>("/api/scheduler/", { method: "POST", body: JSON.stringify(schedule) }),
  getSchedulerStatus: (pipelineId: string) =>
    apiFetch<SchedulerStatus>(`/api/scheduler/${pipelineId}`),

  exportData: async (
    format: string,
    columns: string[],
    rows: unknown[],
    options: Record<string, unknown> = {}
  ): Promise<Blob> => {
    const base = await apiBase();
    const res = await fetch(`${base}/api/export`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ format, columns, rows, options }),
    });
    if (!res.ok) throw new Error("Export failed");
    return res.blob();
  },
};
