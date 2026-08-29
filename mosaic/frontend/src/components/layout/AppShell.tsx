import { useMosaicStore } from "../../state/store";
import TitleBar from "./TitleBar";
import ProjectExplorer from "./ProjectExplorer";
import Inspector from "./Inspector";
import ConsolePanel from "./ConsolePanel";
import PipelineCanvas from "../canvas/PipelineCanvas";
import StartCenter from "../start-center/StartCenter";
import SchedulerPanel from "../scheduler/SchedulerPanel";
import CommandPalette from "../command-palette/CommandPalette";

export default function AppShell() {
  const view = useMosaicStore((s) => s.view);
  const isCanvas = view === "canvas";

  return (
    <div className="mosaic-app-shell">
      <TitleBar />
      <div className="mosaic-workspace">
        {isCanvas && <ProjectExplorer />}

        <main style={{ display: "flex", flexDirection: "column", overflow: "hidden", minWidth: 0 }}>
          {view === "start" && <StartCenter />}
          {view === "canvas" && <PipelineCanvas />}
          {view === "scheduler" && <SchedulerPanel />}
          {view === "projects" && <ProjectsView />}
        </main>

        {isCanvas && <Inspector />}
        {isCanvas && <ConsolePanel />}
      </div>
      <CommandPalette />
    </div>
  );
}

function ProjectsView() {
  return <ProjectExplorerFullPage />;
}

// A full-page variant reusing the same panel for the dedicated "Projects" nav
// destination (as opposed to the docked sidebar shown while on the canvas).
function ProjectExplorerFullPage() {
  return (
    <div style={{ padding: 24, maxWidth: 720, margin: "0 auto", width: "100%" }}>
      <ProjectExplorer />
    </div>
  );
}
