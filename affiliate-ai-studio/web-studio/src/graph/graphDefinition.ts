export type GraphNodeKind = "step" | "decision" | "llm" | "terminal";
export type GraphNodeVariant = "ok" | "error" | "neutral";

export interface GraphNodeSpec {
  id: string;
  label: string;
  kind: GraphNodeKind;
  variant?: GraphNodeVariant;
  /** 对应 python graph 里的 node 函数名（用于从 trace 里匹配） */
  traceKey: string;
  /** 若该节点会产生一个 tool call，在这里给出 tool 名（用于在详情抽屉里匹配） */
  tool?: string;
  /** 简短说明，用于鼠标悬浮提示 */
  description: string;
}

export interface GraphEdgeSpec {
  id: string;
  source: string;
  target: string;
  label?: string;
  /** 当该边属于"只有满足某条件才走"时给出条件 key，用于 skip 判定 */
  branch?: "accepted" | "rejected";
}

export const GRAPH_NODES: GraphNodeSpec[] = [
  {
    id: "normalize_input",
    label: "Normalize",
    kind: "step",
    traceKey: "normalize_input",
    description: "整理入参（URL / 描述 / 平台 / 语言 / 风格）",
  },
  {
    id: "fetch_product_info",
    label: "Fetch Product",
    kind: "step",
    traceKey: "fetch_product_info",
    tool: "fetch_product_info",
    description: "调用商品查询工具，未命中时返回 fallback fixture",
  },
  {
    id: "check_commission",
    label: "Check Commission",
    kind: "decision",
    traceKey: "check_commission",
    tool: "check_commission",
    description: "比对佣金率与阈值，决定 accepted / rejected",
  },
  {
    id: "retrieve_policy_context",
    label: "Retrieve KB",
    kind: "step",
    traceKey: "retrieve_policy_context",
    tool: "search_policy_kb",
    description: "从政策知识库检索证据并（可选）压缩",
  },
  {
    id: "generate_structured_copy",
    label: "Generate Copy",
    kind: "llm",
    traceKey: "generate_structured_copy",
    description: "调 Prompt + LLM 生成结构化营销文案",
  },
  {
    id: "finalize_accepted",
    label: "Accept",
    kind: "terminal",
    variant: "ok",
    traceKey: "finalize_accepted",
    description: "组装 accepted 响应",
  },
  {
    id: "finalize_rejected",
    label: "Reject",
    kind: "terminal",
    variant: "error",
    traceKey: "finalize_rejected",
    description: "佣金不达标，跳过生成，直接拒绝",
  },
];

export const GRAPH_EDGES: GraphEdgeSpec[] = [
  { id: "e1", source: "normalize_input", target: "fetch_product_info" },
  { id: "e2", source: "fetch_product_info", target: "check_commission" },
  {
    id: "e3",
    source: "check_commission",
    target: "retrieve_policy_context",
    label: "accepted",
    branch: "accepted",
  },
  {
    id: "e4",
    source: "check_commission",
    target: "finalize_rejected",
    label: "rejected",
    branch: "rejected",
  },
  {
    id: "e5",
    source: "retrieve_policy_context",
    target: "generate_structured_copy",
  },
  {
    id: "e6",
    source: "generate_structured_copy",
    target: "finalize_accepted",
  },
];

/** O(1) 查找，避免在动画循环里对每条边 / 每个节点反复 `find` */
export const GRAPH_NODE_BY_ID: ReadonlyMap<string, GraphNodeSpec> = new Map(
  GRAPH_NODES.map((n) => [n.id, n]),
);

export function findNodeSpec(id: string): GraphNodeSpec | undefined {
  return GRAPH_NODE_BY_ID.get(id);
}
