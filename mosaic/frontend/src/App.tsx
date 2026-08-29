import { useEffect } from "react";
import { useMosaicStore } from "./state/store";
import { applyDocumentDirection } from "./i18n";
import AppShell from "./components/layout/AppShell";

export default function App() {
  const theme = useMosaicStore((s) => s.theme);
  const language = useMosaicStore((s) => s.language);

  useEffect(() => {
    document.documentElement.setAttribute("data-theme", theme);
  }, [theme]);

  useEffect(() => {
    applyDocumentDirection(language);
  }, [language]);

  return <AppShell />;
}
