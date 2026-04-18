import { useCallback } from "react";
import { BookOpen, Github, Moon, Sun } from "lucide-react";
import { Button } from "@/components/ui/Button";
import { ErrorBanner } from "@/components/ui/ErrorBanner";
import { GraphCanvas } from "@/graph/GraphCanvas";
import { useTheme } from "@/hooks/useTheme";
import { InputPanel } from "@/panels/InputPanel";
import { NodeDetailDrawer } from "@/panels/NodeDetailDrawer";
import { ResultPanel } from "@/panels/ResultPanel";
import { runAffiliate } from "@/api/affiliate";
import { useRunStore } from "@/store/runStore";

export function App() {
  const { theme, toggle } = useTheme();
  const startRun = useRunStore((s) => s.startRun);
  const finishRun = useRunStore((s) => s.finishRun);
  const failRun = useRunStore((s) => s.failRun);
  const form = useRunStore((s) => s.form);
  const error = useRunStore((s) => s.error);

  const handleRun = useCallback(async () => {
    startRun();
    try {
      const resp = await runAffiliate(form);
      finishRun(resp);
    } catch (err) {
      failRun(err as Error);
    }
  }, [failRun, finishRun, form, startRun]);

  const handleCopyError = useCallback(() => {
    if (!error) return;
    navigator.clipboard.writeText(JSON.stringify(error, null, 2)).catch(() => {});
  }, [error]);

  return (
    <div className="flex h-full min-h-screen flex-col bg-background text-fg">
      <header className="flex items-center justify-between gap-4 border-b border-border bg-surface px-6 py-3">
        <div>
          <h1 className="text-base font-semibold leading-tight">
            Affiliate AI Studio
          </h1>
          <p className="text-xs text-fgMuted">
            Graph Workbench · LangGraph + Prompt + RAG + Tools
          </p>
        </div>
        <div className="flex items-center gap-2">
          <a
            href="/learn"
            className="inline-flex items-center gap-1 rounded-lg border border-border px-3 py-1.5 text-xs text-fgMuted hover:text-fg"
          >
            <BookOpen size={14} /> Learn
          </a>
          <a
            href="https://github.com/"
            target="_blank"
            rel="noreferrer noopener"
            className="inline-flex items-center gap-1 rounded-lg border border-border px-3 py-1.5 text-xs text-fgMuted hover:text-fg"
          >
            <Github size={14} /> GitHub
          </a>
          <Button size="sm" variant="ghost" onClick={toggle} title="切换主题">
            {theme === "dark" ? <Sun size={14} /> : <Moon size={14} />}
          </Button>
        </div>
      </header>

      {error ? (
        <div className="px-6 pt-4">
          <ErrorBanner
            error={error}
            onDismiss={() =>
              useRunStore.setState({ error: null, status: "idle" })
            }
            onCopy={handleCopyError}
          />
        </div>
      ) : null}

      <main className="grid flex-1 gap-4 p-4 md:p-6 lg:grid-cols-[320px_minmax(0,1fr)_380px]">
        <section className="lg:h-[calc(100vh-140px)]">
          <InputPanel onRun={handleRun} />
        </section>
        <section className="h-[520px] lg:h-[calc(100vh-140px)]">
          <GraphCanvas />
        </section>
        <section className="h-[520px] lg:h-[calc(100vh-140px)]">
          <ResultPanel />
        </section>
      </main>

      <NodeDetailDrawer />

      <footer className="border-t border-border px-6 py-3 text-center text-xs text-fgMuted">
        运行后 Graph 会按照 trace 顺序逐步点亮；点击节点查看详情。
      </footer>
    </div>
  );
}
