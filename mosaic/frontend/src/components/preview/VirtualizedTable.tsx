import { useMemo, useRef, useState } from "react";
import type { Row } from "../../services/types";

const ROW_HEIGHT = 28;
const OVERSCAN = 6;

// A dependency-free virtualization strategy: track scrollTop in state and
// slice the row array to only the rows intersecting the visible viewport
// (+ overscan). This is what keeps the Data Preview responsive for the
// "preview first few thousand rows" case even though React itself never
// sees more than a few dozen <tr> elements at once.
export default function VirtualizedTable({ columns, rows }: { columns: string[]; rows: Row[] }) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [scrollTop, setScrollTop] = useState(0);
  const [viewportHeight, setViewportHeight] = useState(240);

  const totalHeight = rows.length * ROW_HEIGHT;
  const startIndex = Math.max(0, Math.floor(scrollTop / ROW_HEIGHT) - OVERSCAN);
  const endIndex = Math.min(rows.length, Math.ceil((scrollTop + viewportHeight) / ROW_HEIGHT) + OVERSCAN);
  const visible = useMemo(() => rows.slice(startIndex, endIndex), [rows, startIndex, endIndex]);

  return (
    <div
      ref={containerRef}
      className="mosaic-scrollbar"
      style={{ overflow: "auto", height: "100%" }}
      onScroll={(e) => setScrollTop(e.currentTarget.scrollTop)}
      onLoad={(e) => setViewportHeight((e.target as HTMLDivElement).clientHeight)}
    >
      <table className="mosaic-mono ltr-force" style={{ borderCollapse: "collapse", fontSize: 11.5, width: "max-content", minWidth: "100%" }}>
        <thead>
          <tr>
            {columns.map((c) => (
              <th
                key={c}
                style={{
                  position: "sticky",
                  top: 0,
                  background: "var(--surface-2)",
                  textAlign: "start",
                  padding: "6px 10px",
                  borderBottom: "1px solid var(--surface-border)",
                  whiteSpace: "nowrap",
                }}
              >
                {c}
              </th>
            ))}
          </tr>
        </thead>
        <tbody style={{ position: "relative", display: "block", height: totalHeight }}>
          <tr style={{ display: "block", position: "relative" }}>
            <td style={{ display: "block", padding: 0, height: startIndex * ROW_HEIGHT }} />
          </tr>
          {visible.map((row, i) => (
            <tr key={startIndex + i} style={{ display: "flex", height: ROW_HEIGHT }}>
              {columns.map((c) => (
                <td
                  key={c}
                  style={{
                    padding: "5px 10px",
                    borderBottom: "1px solid var(--surface-border)",
                    whiteSpace: "nowrap",
                    minWidth: 120,
                  }}
                >
                  {formatCell(row[c])}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function formatCell(v: unknown): string {
  if (v === null || v === undefined) return "∅";
  if (typeof v === "object") return JSON.stringify(v);
  return String(v);
}
