import { useEffect, useMemo, useRef } from "react";
import {
  Background,
  BackgroundVariant,
  Controls,
  MarkerType,
  ReactFlow,
  useNodesInitialized,
  useNodesState,
  useReactFlow,
  type Edge,
  type NodeTypes,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { useRunStore } from "@/store/runStore";
import {
  LAYOUT_NODE_HEIGHT,
  LAYOUT_NODE_WIDTH,
  layoutNodesWithDagre,
} from "./dagreLayout";
import { GRAPH_EDGES, GRAPH_NODES } from "./graphDefinition";
import {
  useNodeStates,
  useTraceAnimator,
} from "./useGraphAnimation";
import { GraphNode, type GraphFlowNode } from "./nodes/GraphNode";

const nodeTypes: NodeTypes = {
  graphNode: GraphNode as unknown as NodeTypes[string],
};

const FIT_VIEW_OPTIONS = { padding: 0.2, duration: 0 } as const;

function buildInitialFlowNodes(): GraphFlowNode[] {
  const positions = layoutNodesWithDagre(GRAPH_NODES, GRAPH_EDGES);
  return GRAPH_NODES.map((spec) => ({
    id: spec.id,
    type: "graphNode",
    position: positions[spec.id] ?? { x: 0, y: 0 },
    width: LAYOUT_NODE_WIDTH,
    height: LAYOUT_NODE_HEIGHT,
    data: {
      label: spec.label,
      kind: spec.kind,
      variant: spec.variant,
      status: "idle",
      description: spec.description,
    },
    selected: false,
  }));
}

/**
 * 在自定义节点尚未被测量宽高时调用 fitView，会把内容框算得过小，
 * 视口缩放异常，节点看起来「全挤在一起」。等节点初始化后再 fit 一次即可。
 */
function FitViewWhenNodesMeasured() {
  const { fitView } = useReactFlow();
  const nodesReady = useNodesInitialized();
  const didFit = useRef(false);

  useEffect(() => {
    if (!nodesReady || didFit.current) return;
    didFit.current = true;
    fitView(FIT_VIEW_OPTIONS);
  }, [fitView, nodesReady]);

  return null;
}

export function GraphCanvas() {
  const response = useRunStore((s) => s.response);
  const selectedNode = useRunStore((s) => s.selectedNode);
  const selectNode = useRunStore((s) => s.selectNode);
  const nodeStates = useNodeStates();

  const initialNodesRef = useRef<GraphFlowNode[] | null>(null);
  if (initialNodesRef.current === null) {
    initialNodesRef.current = buildInitialFlowNodes();
  }
  const [nodes, setNodes, onNodesChange] = useNodesState<GraphFlowNode>(
    initialNodesRef.current,
  );

  useTraceAnimator(response);

  useEffect(() => {
    setNodes((nds) =>
      nds.map((n) => {
        const spec = GRAPH_NODES.find((s) => s.id === n.id);
        if (!spec) return n;
        const state = nodeStates[spec.id];
        return {
          ...n,
          data: {
            label: spec.label,
            kind: spec.kind,
            variant: spec.variant,
            status: state?.status ?? "idle",
            description: spec.description,
          },
          selected: selectedNode === spec.id,
        };
      }),
    );
  }, [nodeStates, selectedNode, setNodes]);

  const edges = useMemo<Edge[]>(() => {
    const decision = response?.result?.decision ?? null;
    return GRAPH_EDGES.map((edge) => {
      const srcState = nodeStates[edge.source]?.status;
      const tgtState = nodeStates[edge.target]?.status;

      let isActive = false;
      let isSkipped = false;
      if (edge.branch && decision) {
        isActive = edge.branch === decision;
        isSkipped = edge.branch !== decision;
      }
      if (!edge.branch) {
        isActive =
          (srcState === "done" || srcState === "error") &&
          (tgtState === "done" || tgtState === "active");
      }
      if (tgtState === "skipped" || srcState === "skipped") isSkipped = true;

      return {
        id: edge.id,
        source: edge.source,
        target: edge.target,
        label: edge.label,
        animated: isActive && !isSkipped,
        style: {
          stroke: isSkipped
            ? "rgb(var(--border))"
            : isActive
              ? "rgb(var(--primary))"
              : "rgb(var(--border))",
          strokeWidth: isActive ? 2 : 1.2,
          strokeDasharray: isSkipped ? "4 4" : undefined,
          opacity: isSkipped ? 0.6 : 1,
        },
        labelStyle: {
          fill: "rgb(var(--fg-muted))",
          fontSize: 11,
          fontWeight: 500,
        },
        labelBgStyle: {
          fill: "rgb(var(--surface))",
        },
        markerEnd: {
          type: MarkerType.ArrowClosed,
          color: isSkipped
            ? "rgb(var(--border))"
            : isActive
              ? "rgb(var(--primary))"
              : "rgb(var(--border))",
        },
      };
    });
  }, [nodeStates, response]);

  return (
    <div className="relative h-full w-full overflow-hidden rounded-2xl border border-border bg-surface">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        onNodesChange={onNodesChange}
        nodesDraggable
        nodesConnectable={false}
        elementsSelectable
        proOptions={{ hideAttribution: true }}
        onNodeClick={(_, node) => selectNode(node.id)}
        onPaneClick={() => selectNode(null)}
      >
        <FitViewWhenNodesMeasured />
        <Background
          variant={BackgroundVariant.Dots}
          gap={18}
          size={1.2}
          color="rgb(var(--border))"
        />
        <Controls
          position="bottom-right"
          showInteractive={false}
          className="!rounded-lg !border !border-border !bg-surface/90 [&_button]:!bg-transparent [&_button]:!text-fg [&_button]:!border-border"
        />
      </ReactFlow>
    </div>
  );
}
