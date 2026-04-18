import { FileText, Scissors } from "lucide-react";
import { Badge } from "@/components/ui/Badge";
import type { RetrievalHit, RetrievalPayload } from "@/api/affiliate";

export function RetrievalPanel({ data }: { data: RetrievalPayload | undefined }) {
  if (!data || (data.hits.length === 0 && data.compressed_context.length === 0)) {
    return (
      <div className="p-4 text-sm text-fgMuted">
        本次运行没有检索结果（可能被拒早退或知识库为空）。
      </div>
    );
  }
  return (
    <div className="space-y-5 p-4 text-sm">
      <section>
        <h3 className="mb-2 flex items-center gap-1.5 text-xs font-semibold uppercase tracking-wide text-fgMuted">
          <FileText size={13} /> Hits ({data.hits.length})
        </h3>
        <div className="space-y-2">
          {data.hits.map((hit) => (
            <HitCard key={hit.id} hit={hit} />
          ))}
        </div>
      </section>
      <section>
        <h3 className="mb-2 flex items-center gap-1.5 text-xs font-semibold uppercase tracking-wide text-fgMuted">
          <Scissors size={13} /> Compressed ({data.compressed_context.length})
        </h3>
        <div className="space-y-2">
          {data.compressed_context.map((c) => (
            <div
              key={c.id + ":c"}
              className="rounded-lg border border-border bg-surfaceMuted p-3"
            >
              <p className="text-xs font-medium">{c.title}</p>
              <p className="mt-1 text-sm">{c.excerpt ?? c.text}</p>
            </div>
          ))}
        </div>
      </section>
    </div>
  );
}

function HitCard({ hit }: { hit: RetrievalHit }) {
  return (
    <div className="rounded-lg border border-border bg-surface p-3">
      <div className="flex items-start justify-between gap-2">
        <p className="font-medium">{hit.title}</p>
        {typeof hit.score === "number" ? (
          <Badge tone="info">score {hit.score}</Badge>
        ) : null}
      </div>
      {hit.source ? (
        <p className="mt-0.5 text-xs text-fgMuted">{hit.source}</p>
      ) : null}
      {hit.text ? (
        <p className="mt-2 text-sm text-fg/90">{hit.text}</p>
      ) : null}
    </div>
  );
}
