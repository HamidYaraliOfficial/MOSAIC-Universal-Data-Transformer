import { useCallback, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import ReactFlow, {
  Background,
  BackgroundVariant,
  Controls,
  MiniMap,
  Node,
  Edge,
  Connection,
  ReactFlowProvider,
  useReactFlow,
} from "reactflow";
import "reactflow/dist/style.css";
import { Play, Save, Undo2, Redo2, ZoomIn, ZoomOut, Maximize2 } from "lucide-react";
import { useMosaicStore } from "../../state/store";
import { api } from "../../services/api";
import NodePalette from "./NodePalette";
import BaseNode, { MosaicNodeData } from "./nodes/BaseNode";
import type { NodeDef } from "../../services/types";

const nodeTypes = { mosaicNode: BaseNode };
let idCounter = 1;

function CanvasInner() {
  const { t } = useTranslation();
  const wrapperRef = useRef<HTMLDivElement>(null);
  const { screenToFlowPosition, zoomIn, zoomOut, fitView } = useReactFlow();

  const storeNodes = useMosaicStore((s) => s.nodes);
  const storeEdges = useMosaicStore((s) => s.edges);
  const addNode = useMosaicStore((s) => s.addNode);
  const addEdge = useMosaicStore((s) => s.addEdge);
  const selectNode = useMosaicStore((s) => s.selectNode);
  const undo = useMosaicStore((s) => s.undo);
  const redo = useMosaicStore((s) => s.redo);
  const setActiveJobId = useMosaicStore((s) => s.setActiveJobId);

  const [running, setRunning] = useState(false);

  const flowNodes: Node<MosaicNodeData>[] = useMemo(
    () =>
      storeNodes.map((n) => ({
        id: n.id,
        type: "mosaicNode",
        position: { x: n.position[0], y: n.position[1] },
        data: { label: n.type, nodeType: n.type, disabled: n.disabled, status: "idle" },
      })),
    [storeNodes]
  );

  const flowEdges: Edge[] = useMemo(
    () =>
      storeEdges.map((e, i) => ({
        id: `e${i}-${e.from}-${e.to}`,
        source: e.from,
        target: e.to,
        sourceHandle: e.fromPort,
        targetHandle: e.toPort,
        animated: false,
        style: { stroke: "var(--accent)" },
      })),
    [storeEdges]
  );

  const onConnect = useCallback(
    (c: Connection) => {
      if (!c.source || !c.target) return;
      addEdge({ from: c.source, to: c.target, fromPort: c.sourceHandle ?? undefined, toPort: c.targetHandle ?? undefined });
    },
    [addEdge]
  );

  const addNodeAt = useCallback(
    (nodeType: string, position: { x: number; y: number }) => {
      const def: NodeDef = {
        id: `${nodeType}_${idCounter++}`,
        type: nodeType,
        config: {},
        onError: "skip",
        position: [position.x, position.y],
      };
      addNode(def);
    },
    [addNode]
  );

  function onDrop(e: React.DragEvent) {
    e.preventDefault();
    const nodeType = e.dataTransfer.getData("application/mosaic-node");
    if (!nodeType) return;
    const position = screenToFlowPosition({ x: e.clientX, y: e.clientY });
    addNodeAt(nodeType, position);
  }

  function onAddFromPaletteClick(nodeType: string) {
    addNodeAt(nodeType, { x: 120 + Math.random() * 240, y: 80 + Math.random() * 240 });
  }

  async function runPipeline() {
    setRunning(true);
    try {
      const dataset = useMosaicStore.getState().activeDataset;
      const inputNodes = storeNodes.filter((n) => n.type === "input");
      const sources: Record<string, unknown[]> = {};
      if (dataset && inputNodes[0]) sources[inputNodes[0].id] = dataset.rows;

      const { jobId } = await api.runPipeline(
        {
          version: 1,
          id: "pipeline_main",
          name: "Untitled Pipeline",
          nodes: storeNodes,
          edges: storeEdges,
        },
        sources
      );
      setActiveJobId(jobId);
      await api.streamJob(jobId, () => {
        /* the Job Engine panel (see ConsolePanel) reflects live progress */
      });
    } catch (err) {
      console.error(err);
    } finally {
      setRunning(false);
    }
  }

  return (
    <div style={{ flex: 1, display: "flex", flexDirection: "column", minWidth: 0 }}>
      <Toolbar
        onRun={runPipeline}
        running={running}
        onUndo={undo}
        onRedo={redo}
        onZoomIn={() => zoomIn()}
        onZoomOut={() => zoomOut()}
        onFit={() => fitView({ padding: 0.2 })}
      />
      <div style={{ flex: 1, display: "flex", minHeight: 0 }}>
        <NodePalette onAdd={onAddFromPaletteClick} />
        <div
          ref={wrapperRef}
          style={{ flex: 1 }}
          onDrop={onDrop}
          onDragOver={(e) => e.preventDefault()}
        >
          <ReactFlow
            nodes={flowNodes}
            edges={flowEdges}
            nodeTypes={nodeTypes}
            onConnect={onConnect}
            onNodeClick={(_, n) => selectNode(n.id)}
            onPaneClick={() => selectNode(null)}
            fitView
            proOptions={{ hideAttribution: true }}
          >
            <Background variant={BackgroundVariant.Dots} gap={18} size={1.2} color="var(--bg-canvas-dot)" />
            <Controls showInteractive={false} />
            <MiniMap
              pannable
              zoomable
              maskColor="rgba(0,0,0,0.35)"
              style={{ background: "var(--surface-1)" }}
            />
          </ReactFlow>
        </div>
      </div>
    </div>
  );
}

function Toolbar({
  onRun,
  running,
  onUndo,
  onRedo,
  onZoomIn,
  onZoomOut,
  onFit,
}: {
  onRun: () => void;
  running: boolean;
  onUndo: () => void;
  onRedo: () => void;
  onZoomIn: () => void;
  onZoomOut: () => void;
  onFit: () => void;
}) {
  const { t } = useTranslation();
  return (
    <div
      style={{
        display: "flex",
        alignItems: "center",
        gap: 6,
        padding: "6px 10px",
        borderBottom: "1px solid var(--surface-border)",
        background: "var(--surface-1)",
      }}
    >
      <button className="mosaic-btn-primary" onClick={onRun} disabled={running} style={{ display: "flex", alignItems: "center", gap: 6 }}>
        <Play size={13} /> {t("canvas.run")}
      </button>
      <button className="mosaic-btn-ghost" style={{ display: "flex", alignItems: "center", gap: 6 }}>
        <Save size={13} /> {t("canvas.save")}
      </button>
      <div style={{ width: 1, height: 20, background: "var(--surface-border)", margin: "0 4px" }} />
      <button className="mosaic-btn-ghost" onClick={onUndo} title={t("canvas.undo") ?? ""}>
        <Undo2 size={13} />
      </button>
      <button className="mosaic-btn-ghost" onClick={onRedo} title={t("canvas.redo") ?? ""}>
        <Redo2 size={13} />
      </button>
      <div style={{ width: 1, height: 20, background: "var(--surface-border)", margin: "0 4px" }} />
      <button className="mosaic-btn-ghost" onClick={onZoomIn} title={t("canvas.zoomIn") ?? ""}>
        <ZoomIn size={13} />
      </button>
      <button className="mosaic-btn-ghost" onClick={onZoomOut} title={t("canvas.zoomOut") ?? ""}>
        <ZoomOut size={13} />
      </button>
      <button className="mosaic-btn-ghost" onClick={onFit} title={t("canvas.fitView") ?? ""}>
        <Maximize2 size={13} />
      </button>
    </div>
  );
}

export default function PipelineCanvas() {
  return (
    <ReactFlowProvider>
      <CanvasInner />
    </ReactFlowProvider>
  );
}
