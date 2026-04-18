import { X } from "lucide-react";
import { Badge } from "@/components/ui/Badge";
import { JsonBlock } from "@/components/ui/JsonBlock";
import { findNodeSpec } from "@/graph/graphDefinition";
import { useNodeStates } from "@/graph/useGraphAnimation";
import { useRunStore } from "@/store/runStore";

const STATUS_TONE = {
  idle: "neutral",
  active: "info",
  done: "success",
  skipped: "neutral",
  error: "danger",
} as const;

const STATUS_LABEL = {
  idle: "未开始",
  active: "运行中",
  done: "完成",
  skipped: "已跳过",
  error: "失败",
} as const;

export function NodeDetailDrawer() {
  const selectedNode = useRunStore((s) => s.selectedNode);
  const selectNode = useRunStore((s) => s.selectNode);
  const response = useRunStore((s) => s.response);
  const nodeStates = useNodeStates();

  if (!selectedNode) return null;
  const spec = findNodeSpec(selectedNode);
  if (!spec) return null;
  const state = nodeStates[selectedNode];
  const status = state?.status ?? "idle";

  const toolCall = spec.tool
    ? response?.tools?.find((t) => t.tool === spec.tool)
    : undefined;

  return (
    <>
      <div
        className="fixed inset-0 z-30 bg-black/30 backdrop-blur-sm"
        onClick={() => selectNode(null)}
        aria-hidden
      />
      <aside
        role="dialog"
        aria-label={`${spec.label} 节点详情`}
        className="fixed inset-y-0 right-0 z-40 flex w-full max-w-md flex-col overflow-hidden border-l border-border bg-surface shadow-xl animate-in slide-in-from-right"
      >
        <div className="flex items-start justify-between gap-3 border-b border-border p-4">
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <h2 className="text-lg font-semibold">{spec.label}</h2>
              <Badge tone={STATUS_TONE[status]}>{STATUS_LABEL[status]}</Badge>
            </div>
            <p className="mt-1 text-sm text-fgMuted">{spec.description}</p>
            <p className="mt-1 font-mono text-xs text-fgMuted">
              python/graph/affiliate_graph.py · <b>{spec.traceKey}</b>
            </p>
          </div>
          <button
            className="rounded p-1 text-fgMuted hover:text-fg"
            onClick={() => selectNode(null)}
            aria-label="关闭"
          >
            <X size={18} />
          </button>
        </div>

        <div className="flex-1 space-y-5 overflow-auto scrollbar-thin p-4 text-sm">
          {state?.trace ? (
            <section>
              <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-fgMuted">
                Trace
              </h3>
              <div className="rounded-lg border border-border bg-surfaceMuted p-3">
                <p>{state.trace.summary}</p>
                <p className="mt-1 text-xs text-fgMuted">
                  status: <code>{state.trace.status}</code>
                </p>
              </div>
              {state.trace.output_snapshot ? (
                <div className="mt-2">
                  <p className="mb-1 text-xs font-medium text-fgMuted">
                    output_snapshot
                  </p>
                  <JsonBlock value={state.trace.output_snapshot} />
                </div>
              ) : null}
              {state.trace.input_snapshot ? (
                <div className="mt-2">
                  <p className="mb-1 text-xs font-medium text-fgMuted">
                    input_snapshot
                  </p>
                  <JsonBlock value={state.trace.input_snapshot} />
                </div>
              ) : null}
            </section>
          ) : status === "skipped" ? (
            <section className="rounded-lg border border-dashed border-border p-4 text-fgMuted">
              该节点位于被绕开的分支，本次运行中没有执行。
            </section>
          ) : (
            <section className="rounded-lg border border-dashed border-border p-4 text-fgMuted">
              本次运行中该节点未产生 trace。
            </section>
          )}

          {toolCall ? (
            <section>
              <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-fgMuted">
                Tool Call · <code>{toolCall.tool}</code>
              </h3>
              <div className="space-y-2">
                <div>
                  <p className="mb-1 text-xs font-medium text-fgMuted">input</p>
                  <JsonBlock value={toolCall.input} />
                </div>
                <div>
                  <p className="mb-1 text-xs font-medium text-fgMuted">output</p>
                  <JsonBlock value={toolCall.output} />
                </div>
              </div>
            </section>
          ) : null}
        </div>
      </aside>
    </>
  );
}
