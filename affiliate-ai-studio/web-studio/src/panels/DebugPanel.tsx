import { JsonBlock } from "@/components/ui/JsonBlock";
import type { RunAffiliateResponse } from "@/api/affiliate";

export function DebugPanel({ response }: { response: RunAffiliateResponse }) {
  return (
    <div className="space-y-4 p-4 text-sm">
      <section>
        <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-fgMuted">
          Rendered Prompt
        </h3>
        <pre className="max-h-[260px] overflow-auto scrollbar-thin whitespace-pre-wrap rounded-lg border border-border bg-surfaceMuted p-3 text-xs leading-5">
{response.debug?.rendered_prompt || "（空）"}
        </pre>
      </section>
      <section>
        <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-fgMuted">
          Raw Model Output
        </h3>
        <pre className="max-h-[220px] overflow-auto scrollbar-thin whitespace-pre-wrap rounded-lg border border-border bg-surfaceMuted p-3 text-xs leading-5">
{response.debug?.raw_output || "（空）"}
        </pre>
      </section>
      <section>
        <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-fgMuted">
          Full Response JSON
        </h3>
        <JsonBlock value={response} />
      </section>
    </div>
  );
}
