import { useState } from "react";
import { Check, Copy } from "lucide-react";

export function JsonBlock({ value }: { value: unknown }) {
  const [copied, setCopied] = useState(false);
  const text = JSON.stringify(value, null, 2);

  async function handleCopy() {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      setTimeout(() => setCopied(false), 1200);
    } catch {
      // ignore
    }
  }

  return (
    <div className="relative">
      <button
        type="button"
        onClick={handleCopy}
        className="absolute right-2 top-2 inline-flex items-center gap-1 rounded-md border border-border bg-surface px-2 py-1 text-xs text-fgMuted hover:text-fg"
      >
        {copied ? (
          <>
            <Check size={14} /> 已复制
          </>
        ) : (
          <>
            <Copy size={14} /> 复制
          </>
        )}
      </button>
      <pre className="max-h-[360px] overflow-auto scrollbar-thin rounded-lg border border-border bg-surfaceMuted p-3 pr-20 text-xs leading-5">
        <code>{text}</code>
      </pre>
    </div>
  );
}
