import { memo } from "react";
import { Handle, Position, type Node, type NodeProps } from "@xyflow/react";
import {
  Check,
  Circle,
  Loader2,
  AlertTriangle,
  GitBranch,
  Package,
  Search,
  Sparkles,
  CheckCircle2,
  XCircle,
} from "lucide-react";
import type { GraphNodeKind, GraphNodeVariant } from "../graphDefinition";
import type { NodeVisualStatus } from "../useGraphAnimation";

export interface GraphNodeData extends Record<string, unknown> {
  label: string;
  kind: GraphNodeKind;
  variant?: GraphNodeVariant;
  status: NodeVisualStatus;
  description: string;
  selected?: boolean;
}

const STATUS_STYLES: Record<NodeVisualStatus, string> = {
  idle: "border-border bg-surface text-fgMuted",
  active:
    "border-primary bg-surface text-fg animate-pulse-border ring-2 ring-primary/30",
  done: "border-success/60 bg-success/10 text-fg",
  skipped:
    "border-dashed border-border bg-surface/60 text-fgMuted opacity-60",
  error: "border-danger bg-danger/10 text-fg",
};

function KindIcon({
  kind,
  variant,
  status,
}: {
  kind: GraphNodeKind;
  variant?: GraphNodeVariant;
  status: NodeVisualStatus;
}) {
  if (status === "active")
    return <Loader2 size={16} className="animate-spin text-primary" />;
  if (status === "done") return <Check size={16} className="text-success" />;
  if (status === "error")
    return <AlertTriangle size={16} className="text-danger" />;
  if (status === "skipped")
    return <Circle size={16} className="text-fgMuted" />;

  if (kind === "decision") return <GitBranch size={16} />;
  if (kind === "llm") return <Sparkles size={16} />;
  if (kind === "terminal")
    return variant === "ok" ? (
      <CheckCircle2 size={16} className="text-success" />
    ) : variant === "error" ? (
      <XCircle size={16} className="text-danger" />
    ) : (
      <Circle size={16} />
    );
  if (kind === "step") {
    return <Package size={16} />;
  }
  return <Search size={16} />;
}

export type GraphFlowNode = Node<GraphNodeData, "graphNode">;

export const GraphNode = memo(function GraphNode(
  props: NodeProps<GraphFlowNode>,
) {
  const { data, selected } = props;
  const statusStyle = STATUS_STYLES[data.status] ?? STATUS_STYLES.idle;

  return (
    <div
      className={[
        "group min-w-[180px] max-w-[280px] cursor-pointer rounded-xl border px-3 py-2.5 shadow-sm transition",
        statusStyle,
        selected ? "ring-2 ring-primary" : "",
      ].join(" ")}
      title={data.description}
    >
      <Handle
        type="target"
        position={Position.Left}
        className="!h-2 !w-2 !border-border !bg-surface"
      />
      <div className="flex items-center gap-2">
        <KindIcon
          kind={data.kind}
          variant={data.variant}
          status={data.status}
        />
        <span className="text-sm font-medium">{data.label}</span>
      </div>
      <p className="mt-1 line-clamp-2 text-[11px] leading-4 text-fgMuted">
        {data.description}
      </p>
      <Handle
        type="source"
        position={Position.Right}
        className="!h-2 !w-2 !border-border !bg-surface"
      />
    </div>
  );
});
