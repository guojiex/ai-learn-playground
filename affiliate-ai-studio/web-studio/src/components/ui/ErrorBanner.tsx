import { AlertTriangle, X } from "lucide-react";
import type { RunErrorInfo } from "@/store/runStore";
import { Button } from "./Button";

export function ErrorBanner({
  error,
  onDismiss,
  onCopy,
}: {
  error: RunErrorInfo;
  onDismiss?: () => void;
  onCopy?: () => void;
}) {
  return (
    <div className="flex items-start gap-3 rounded-xl border border-danger/40 bg-danger/10 p-4 text-sm text-danger">
      <AlertTriangle size={18} className="mt-0.5 shrink-0" />
      <div className="flex-1 space-y-1">
        <div className="flex flex-wrap items-center gap-2">
          <span className="font-semibold">运行失败</span>
          <code className="rounded bg-danger/20 px-1.5 py-0.5 text-xs">
            {error.code}
          </code>
          {error.httpStatus ? (
            <code className="rounded bg-danger/20 px-1.5 py-0.5 text-xs">
              HTTP {error.httpStatus}
            </code>
          ) : null}
        </div>
        <p className="break-words text-fg/90">{error.message}</p>
        {(error.action || error.session_id) && (
          <p className="text-xs text-fgMuted">
            {error.action ? (
              <>
                action=<code>{error.action}</code>
                {error.session_id ? " · " : null}
              </>
            ) : null}
            {error.session_id ? (
              <>
                session=<code>{error.session_id}</code>
              </>
            ) : null}
          </p>
        )}
        <div className="flex gap-2 pt-1">
          {onCopy ? (
            <Button size="sm" variant="secondary" onClick={onCopy}>
              复制错误详情
            </Button>
          ) : null}
        </div>
      </div>
      {onDismiss ? (
        <button
          onClick={onDismiss}
          className="rounded p-1 text-danger/70 hover:text-danger"
          aria-label="关闭"
        >
          <X size={16} />
        </button>
      ) : null}
    </div>
  );
}
