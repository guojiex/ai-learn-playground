export interface RunAffiliateRequest {
  product_url: string;
  product_text: string;
  platform: string;
  locale: string;
  style: string;
  min_commission_rate: number;
  enable_compression: boolean;
  debug?: boolean;
}

export interface ProductInfo {
  id: string;
  url?: string;
  title: string;
  price?: number;
  category?: string;
  highlights?: string[];
  fallback?: boolean;
}

export interface CopyPayload {
  title: string;
  hook: string;
  body: string;
  cta: string;
  tags?: string[];
}

/**
 * Worker 返回的 `copy` 与 Pydantic `AffiliateCopy.model_dump()` 一致（localized_hook、selling_points…），
 * UI 使用 hook/body/cta；在此统一成 CopyPayload。
 */
export function normalizeAffiliateCopyPayload(
  raw: unknown,
): CopyPayload | null {
  if (raw === null || raw === undefined) return null;
  if (typeof raw !== "object") return null;
  const o = raw as Record<string, unknown>;
  const title = typeof o.title === "string" ? o.title : "";

  const sellingRaw = o.selling_points ?? o.sellingPoints;
  const risksRaw = o.risk_notes ?? o.riskNotes;
  const hookRaw = o.localized_hook ?? o.localizedHook ?? o.hook;
  const looksLikeWorkerAffiliateCopy =
    "localized_hook" in o ||
    "localizedHook" in o ||
    "selling_points" in o ||
    "sellingPoints" in o ||
    "risk_notes" in o ||
    "riskNotes" in o;

  if (
    !looksLikeWorkerAffiliateCopy &&
    typeof o.hook === "string" &&
    typeof o.body === "string"
  ) {
    return {
      title,
      hook: o.hook,
      body: o.body,
      cta: typeof o.cta === "string" ? o.cta : "Shop now",
      tags: Array.isArray(o.tags)
        ? o.tags.filter((t): t is string => typeof t === "string")
        : undefined,
    };
  }

  const hook = typeof hookRaw === "string" ? hookRaw : "";

  const selling = Array.isArray(sellingRaw)
    ? sellingRaw.filter((x): x is string => typeof x === "string")
    : [];
  const risks = Array.isArray(risksRaw)
    ? risksRaw.filter((x): x is string => typeof x === "string")
    : [];

  const body =
    selling.length > 0 || risks.length > 0
      ? [
          selling.length > 0
            ? selling.map((s) => `• ${s}`).join("\n")
            : "",
          risks.length > 0
            ? `\nCompliance:\n${risks.map((r) => `• ${r}`).join("\n")}`
            : "",
        ]
          .join("")
          .trim()
      : typeof o.body === "string"
        ? o.body
        : "";

  const cta =
    typeof o.cta === "string" && o.cta.trim() !== ""
      ? o.cta
      : "Shop now — see link in bio";

  const tags =
    Array.isArray(o.tags) && o.tags.every((t): t is string => typeof t === "string")
      ? o.tags
      : selling.length > 0
        ? ["affiliate", "demo"]
        : undefined;

  return { title, hook, body, cta, tags };
}

export interface ResultPayload {
  decision: "accepted" | "rejected";
  reason: string;
  commission_rate: number;
  copy: CopyPayload | null;
  product_info?: ProductInfo;
}

export interface RetrievalHit {
  id: string;
  title: string;
  text?: string;
  source?: string;
  score?: number;
  excerpt?: string;
}

export interface RetrievalPayload {
  hits: RetrievalHit[];
  compressed_context: RetrievalHit[];
}

export interface ToolCall {
  tool: string;
  input: Record<string, unknown>;
  output: unknown;
}

export interface TraceEntry {
  node: string;
  summary: string;
  status: string;
  input_snapshot?: Record<string, unknown>;
  output_snapshot?: Record<string, unknown>;
}

export interface DebugPayload {
  rendered_prompt: string;
  raw_output: string;
}

export interface RunAffiliateResponse {
  session_id: string;
  result: ResultPayload;
  retrieval: RetrievalPayload;
  tools: ToolCall[];
  trace: TraceEntry[];
  debug?: DebugPayload;
  product_info?: ProductInfo;
}

export interface ApiErrorPayload {
  status: "error";
  error: {
    code: string;
    message: string;
    action?: string;
    session_id?: string;
    trace?: string;
  };
}

export class ApiError extends Error {
  public readonly status: number;
  public readonly payload: ApiErrorPayload | null;

  constructor(status: number, payload: ApiErrorPayload | null, message: string) {
    super(message);
    this.status = status;
    this.payload = payload;
  }
}

export async function runAffiliate(
  req: RunAffiliateRequest,
  signal?: AbortSignal,
): Promise<RunAffiliateResponse> {
  const resp = await fetch("/api/affiliate/run", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(req),
    signal,
  });
  const body = await resp.json().catch(() => null);
  if (!resp.ok) {
    const payload = (body as ApiErrorPayload | null) ?? null;
    const message =
      payload?.error?.message ?? `请求失败（HTTP ${resp.status}）`;
    throw new ApiError(resp.status, payload, message);
  }
  const data = body as RunAffiliateResponse;
  // 后端没在顶层返回 product_info；从 tools 里把 fetch_product_info 的 output 拎出来
  // 供前端展示（fallback 徽章需要）。
  if (!data.product_info) {
    const call = data.tools?.find((t) => t.tool === "fetch_product_info");
    if (call && typeof call.output === "object" && call.output !== null) {
      data.product_info = call.output as ProductInfo;
    }
  }
  if (data.result) {
    data.result = {
      ...data.result,
      copy: normalizeAffiliateCopyPayload(data.result.copy),
    };
  }
  return data;
}
