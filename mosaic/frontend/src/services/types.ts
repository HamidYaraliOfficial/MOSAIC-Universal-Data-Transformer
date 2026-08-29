// Mirrors of the Go backend's JSON-serialized types (see backend/internal/*
// for the source of truth). Kept intentionally plain / structurally typed
// so the UI degrades gracefully if a field is added on the Go side.

export type DataType =
  | "string" | "integer" | "float" | "boolean" | "date" | "datetime"
  | "time" | "uuid" | "json" | "array" | "object" | "binary" | "null";

export interface ColumnProfile {
  inferredType: DataType;
  nullCount: number;
  nullRate: number;
  distinctCount: number;
  duplicateRate: number;
  min?: string;
  max?: string;
  meanLength: number;
  formatPattern?: string;
  sampleValues?: string[];
}

export interface Column {
  name: string;
  type: DataType;
  nullable: boolean;
  unique?: boolean;
  description?: string;
  stats?: ColumnProfile;
}

export interface SchemaDef {
  columns: Column[];
}

export type Row = Record<string, unknown>;

export interface ProfileReport {
  schema: SchemaDef;
  rowCount: number;
  sampleRows: Row[];
  durationMs: number;
  warnings?: string[];
  columns: string[];
}

export interface ImportResponse {
  format: string;
  report: ProfileReport;
}

export type OnErrorMode = "stop" | "skip" | "collect";

export interface NodeDef {
  id: string;
  type: string;
  config: Record<string, unknown>;
  onError?: OnErrorMode;
  disabled?: boolean;
  position: [number, number];
}

export interface EdgeDef {
  from: string;
  fromPort?: string;
  to: string;
  toPort?: string;
}

export interface PipelineDefinition {
  version: number;
  id: string;
  name: string;
  nodes: NodeDef[];
  edges: EdgeDef[];
}

export type JobStatus = "queued" | "running" | "paused" | "completed" | "failed" | "cancelled";

export interface Job {
  id: string;
  pipelineName: string;
  status: JobStatus;
  startedAt: string;
  finishedAt?: string;
  rowsProcessed: number;
  rowsPerSec: number;
  memoryMb: number;
  error?: string;
}

export interface QualityIssue {
  column: string;
  dimension: string;
  message: string;
  severity: "info" | "warning" | "critical";
  affectedPct: number;
}

export interface QualityReport {
  overall: number;
  completeness: number;
  validity: number;
  uniqueness: number;
  consistency: number;
  accuracy: number;
  issues: QualityIssue[];
}

export interface Connection {
  id: string;
  name: string;
  kind: string;
  host?: string;
  database?: string;
  vaultKey?: string;
}

export interface Project {
  id: string;
  name: string;
  createdAt: string;
  updatedAt: string;
  pipelines: PipelineDefinition[];
  connections: Connection[];
  tags?: string[];
  favorite: boolean;
}

export interface Window {
  startMinute: number;
  endMinute: number;
}

export interface Schedule {
  pipelineId: string;
  timezone: string;
  days: Record<string, Window>;
  enabled: boolean;
}

export interface SchedulerStatus {
  isOpenNow: boolean;
  now: string;
  currentWindowEnd?: string;
  nextOpenAt?: string;
  timeUntilNext: number; // nanoseconds, per Go time.Duration JSON encoding
  estimatedRunTime: number;
  sampleSize: number;
}
