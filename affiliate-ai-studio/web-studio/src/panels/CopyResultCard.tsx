import { useMemo } from "react";
import {
  AlertTriangle,
  CheckCircle2,
  Link as LinkIcon,
  Tag,
  XCircle,
} from "lucide-react";
import { Badge } from "@/components/ui/Badge";
import {
  normalizeAffiliateCopyPayload,
  type ProductInfo,
  type ResultPayload,
} from "@/api/affiliate";
import { useRunStore } from "@/store/runStore";

export function CopyResultCard({
  result,
  productInfo,
}: {
  result: ResultPayload;
  productInfo?: ProductInfo;
}) {
  const threshold = useRunStore((s) => s.form.min_commission_rate);
  const isAccepted = result.decision === "accepted";
  const isFallback = productInfo?.fallback === true;

  /** 与 API 层一致；组件内再算一次，避免未走 runAffiliate 或旧 bundle 时 Hook/Body 空白 */
  const copy = useMemo(
    () => normalizeAffiliateCopyPayload(result.copy as unknown),
    [result.copy],
  );

  return (
    <div className="space-y-4 p-4">
      <div className="flex flex-wrap items-center gap-2">
        <Badge tone={isAccepted ? "success" : "danger"}>
          {isAccepted ? (
            <>
              <CheckCircle2 size={12} /> accepted
            </>
          ) : (
            <>
              <XCircle size={12} /> rejected
            </>
          )}
        </Badge>
        <span className="text-xs text-fgMuted">
          commission <b>{(result.commission_rate * 100).toFixed(1)}%</b>
        </span>
        {isFallback ? (
          <Badge
            tone="warning"
            title="未匹配到真实商品，这是 fallback fixture"
          >
            <AlertTriangle size={12} /> fallback fixture
          </Badge>
        ) : null}
      </div>

      {productInfo ? <ProductBlock product={productInfo} /> : null}

      {isAccepted && copy ? (
        <div className="space-y-3 rounded-xl border border-success/40 bg-success/5 p-4">
          <div>
            <p className="text-xs font-medium uppercase tracking-wide text-fgMuted">
              Title
            </p>
            <p className="text-lg font-semibold leading-snug">
              {copy.title}
            </p>
          </div>
          <div>
            <p className="text-xs font-medium uppercase tracking-wide text-fgMuted">
              Hook
            </p>
            <p className="text-sm text-fg/90">{copy.hook}</p>
          </div>
          <div>
            <p className="text-xs font-medium uppercase tracking-wide text-fgMuted">
              Body
            </p>
            <p className="whitespace-pre-wrap text-sm text-fg/90">
              {copy.body}
            </p>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <span className="inline-flex items-center gap-1 rounded-full bg-primary px-3 py-1 text-xs font-medium text-primaryFg">
              <LinkIcon size={12} /> {copy.cta}
            </span>
            {copy.tags?.map((t) => (
              <Badge key={t} tone="info">
                <Tag size={10} /> {t}
              </Badge>
            ))}
          </div>
        </div>
      ) : (
        <div className="space-y-2 rounded-xl border border-danger/40 bg-danger/5 p-4 text-sm">
          <p className="font-semibold text-danger">未生成文案</p>
          <p className="text-fg/80">{result.reason}</p>
          <CommissionGauge rate={result.commission_rate} threshold={threshold} />
        </div>
      )}
    </div>
  );
}

function ProductBlock({ product }: { product: ProductInfo }) {
  return (
    <div className="flex gap-3 rounded-xl border border-border bg-surfaceMuted p-3 text-sm">
      <div className="flex h-14 w-14 items-center justify-center rounded-lg bg-surface text-fgMuted">
        <LinkIcon size={20} />
      </div>
      <div className="min-w-0 flex-1">
        <p className="truncate font-medium">{product.title}</p>
        <p className="truncate text-xs text-fgMuted">
          {product.url ?? "(no url)"}
        </p>
        <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-fgMuted">
          {product.price ? <span>¥ {product.price}</span> : null}
          {product.category ? (
            <Badge tone="neutral">{product.category}</Badge>
          ) : null}
          <span className="font-mono">id={product.id}</span>
        </div>
      </div>
    </div>
  );
}

function CommissionGauge({
  rate,
  threshold,
}: {
  rate: number;
  threshold: number;
}) {
  const max = Math.max(rate, threshold, 0.3);
  const ratePct = (rate / max) * 100;
  const thresholdPct = (threshold / max) * 100;
  return (
    <div className="pt-2">
      <div className="relative h-2 rounded-full bg-surfaceMuted">
        <div
          className="absolute top-0 h-2 rounded-full bg-danger"
          style={{ width: `${ratePct}%` }}
        />
        <div
          className="absolute -top-1 h-4 w-0.5 bg-fg/70"
          style={{ left: `${thresholdPct}%` }}
          title={`阈值 ${(threshold * 100).toFixed(1)}%`}
        />
      </div>
      <div className="mt-1 flex justify-between text-[11px] text-fgMuted">
        <span>actual {(rate * 100).toFixed(1)}%</span>
        <span>threshold {(threshold * 100).toFixed(1)}%</span>
      </div>
    </div>
  );
}

