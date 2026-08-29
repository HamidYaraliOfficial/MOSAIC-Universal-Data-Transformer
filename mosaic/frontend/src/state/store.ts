import { create } from "zustand";
import type { NodeDef, EdgeDef, Row, ProfileReport } from "../services/types";
import type { SupportedLanguage } from "../i18n";

export type ThemeName = "windows11" | "light" | "dark" | "amoled" | "red" | "blue";
export type ViewName = "start" | "canvas" | "scheduler" | "projects";

interface CanvasSnapshot {
  nodes: NodeDef[];
  edges: EdgeDef[];
}

interface MosaicState {
  theme: ThemeName;
  language: SupportedLanguage;
  view: ViewName;
  commandPaletteOpen: boolean;

  nodes: NodeDef[];
  edges: EdgeDef[];
  selectedNodeId: string | null;
  history: CanvasSnapshot[];
  future: CanvasSnapshot[];

  activeDataset: { columns: string[]; rows: Row[]; report?: ProfileReport } | null;
  activeJobId: string | null;

  setTheme: (t: ThemeName) => void;
  setLanguage: (l: SupportedLanguage) => void;
  setView: (v: ViewName) => void;
  toggleCommandPalette: (open?: boolean) => void;

  addNode: (node: NodeDef) => void;
  updateNodeConfig: (id: string, config: Record<string, unknown>) => void;
  removeNode: (id: string) => void;
  addEdge: (edge: EdgeDef) => void;
  selectNode: (id: string | null) => void;
  undo: () => void;
  redo: () => void;

  setActiveDataset: (columns: string[], rows: Row[], report?: ProfileReport) => void;
  setActiveJobId: (id: string | null) => void;
}

function pushHistory(state: MosaicState): Pick<MosaicState, "history" | "future"> {
  return {
    history: [...state.history, { nodes: state.nodes, edges: state.edges }].slice(-50),
    future: [],
  };
}

export const useMosaicStore = create<MosaicState>((set, get) => ({
  theme: "windows11",
  language: "en",
  view: "start",
  commandPaletteOpen: false,

  nodes: [],
  edges: [],
  selectedNodeId: null,
  history: [],
  future: [],

  activeDataset: null,
  activeJobId: null,

  setTheme: (theme) => set({ theme }),
  setLanguage: (language) => set({ language }),
  setView: (view) => set({ view }),
  toggleCommandPalette: (open) =>
    set((s) => ({ commandPaletteOpen: open ?? !s.commandPaletteOpen })),

  addNode: (node) =>
    set((s) => ({ ...pushHistory(s), nodes: [...s.nodes, node] })),

  updateNodeConfig: (id, config) =>
    set((s) => ({
      ...pushHistory(s),
      nodes: s.nodes.map((n) => (n.id === id ? { ...n, config: { ...n.config, ...config } } : n)),
    })),

  removeNode: (id) =>
    set((s) => ({
      ...pushHistory(s),
      nodes: s.nodes.filter((n) => n.id !== id),
      edges: s.edges.filter((e) => e.from !== id && e.to !== id),
      selectedNodeId: s.selectedNodeId === id ? null : s.selectedNodeId,
    })),

  addEdge: (edge) => set((s) => ({ ...pushHistory(s), edges: [...s.edges, edge] })),

  selectNode: (id) => set({ selectedNodeId: id }),

  undo: () => {
    const s = get();
    const prev = s.history[s.history.length - 1];
    if (!prev) return;
    set({
      nodes: prev.nodes,
      edges: prev.edges,
      history: s.history.slice(0, -1),
      future: [{ nodes: s.nodes, edges: s.edges }, ...s.future].slice(0, 50),
    });
  },

  redo: () => {
    const s = get();
    const next = s.future[0];
    if (!next) return;
    set({
      nodes: next.nodes,
      edges: next.edges,
      future: s.future.slice(1),
      history: [...s.history, { nodes: s.nodes, edges: s.edges }].slice(-50),
    });
  },

  setActiveDataset: (columns, rows, report) => set({ activeDataset: { columns, rows, report } }),
  setActiveJobId: (id) => set({ activeJobId: id }),
}));
