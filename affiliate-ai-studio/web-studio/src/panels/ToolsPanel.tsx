import { Wrench } from "lucide-react";
import { JsonBlock } from "@/components/ui/JsonBlock";
import type { ToolCall } from "@/api/affiliate";

export function ToolsPanel({ tools }: { tools: ToolCall[] | undefined }) {
  if (!tools || tools.length === 0) {
    return (
      <div className="p-4 text-sm text-fgMuted">
        本次运行没有工具调用记录。
      </div>
    );
  }
  return (
    <div className="space-y-3 p-4">
      {tools.map((call, idx) => (
        <details
          key={`${call.tool}-${idx}`}
          className="group rounded-lg border border-border bg-surface"
          open={idx === 0}
        >
          <summary className="flex cursor-pointer list-none items-center justify-between gap-2 px-3 py-2 text-sm">
            <span className="flex items-center gap-2">
              <Wrench size={14} className="text-primary" />
              <code className="text-sm font-medium">{call.tool}</code>
            </span>
            <span className="text-xs text-fgMuted">
              展开 / 折叠
            </span>
          </summary>
          <div className="space-y-2 border-t border-border p-3">
            <div>
              <p className="mb-1 text-xs font-medium text-fgMuted">input</p>
              <JsonBlock value={call.input} />
            </div>
            <div>
              <p className="mb-1 text-xs font-medium text-fgMuted">output</p>
              <JsonBlock value={call.output} />
            </div>
          </div>
        </details>
      ))}
    </div>
  );
}
