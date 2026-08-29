import { Handle, Position, NodeProps } from "reactflow";
import { AlertCircle, CheckCircle2, CircleDashed } from "lucide-react";

export interface MosaicNodeData {
  label: string;
  nodeType: string;
  status?: "idle" | "ok" | "error";
  rowsIn?: number;
  rowsOut?: number;
  disabled?: boolean;
}

const STATUS_ICON = {
  idle: CircleDashed,
  ok: CheckCircle2,
  error: AlertCircle,
};

export default function BaseNode({ data, selected }: NodeProps<MosaicNodeData>) {
  const StatusIcon = STATUS_ICON[data.status ?? "idle"];
  const statusColor =
    data.status === "error" ? "var(--danger)" : data.status === "ok" ? "var(--success)" : "var(--text-secondary)";

  return (
    <div
      style={{
        minWidth: 168,
        borderRadius: "var(--radius-md)",
        background: "var(--node-bg)",
        border: `1.5px solid ${selected ? "var(--accent)" : "var(--surface-border)"}`,
        boxShadow: selected ? "0 0 0 3px var(--accent-soft)" : "var(--shadow-1)",
        opacity: data.disabled ? 0.5 : 1,
        overflow: "hidden",
      }}
    >
      <Handle type="target" position={Position.Left} style={{ background: "var(--accent)" }} />
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: 6,
          padding: "8px 10px",
          fontSize: 12.5,
          fontWeight: 600,
          borderBottom: "1px solid var(--surface-border)",
        }}
      >
        <StatusIcon size={13} color={statusColor} />
        <span style={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{data.label}</span>
      </div>
      <div style={{ padding: "6px 10px", fontSize: 10.5, color: "var(--text-secondary)", display: "flex", justifyContent: "space-between" }}>
        <span>{data.nodeType}</span>
        {(data.rowsIn !== undefined || data.rowsOut !== undefined) && (
          <span className="mosaic-mono ltr-force">
            {data.rowsIn ?? 0}→{data.rowsOut ?? 0}
          </span>
        )}
      </div>
      <Handle type="source" position={Position.Right} style={{ background: "var(--accent)" }} />
    </div>
  );
}
