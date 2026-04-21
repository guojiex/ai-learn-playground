import dagre from "@dagrejs/dagre";
import type { GraphEdgeSpec, GraphNodeSpec } from "./graphDefinition";

/** 与 GraphNode `max-w` / 双行说明一致，dagre 用其计算间距 */
export const LAYOUT_NODE_WIDTH = 280;
export const LAYOUT_NODE_HEIGHT = 104;

/**
 * 使用 dagre 对 DAG 做分层横向布局，返回 React Flow 用的左上角坐标。
 */
export function layoutNodesWithDagre(
  nodes: GraphNodeSpec[],
  edges: GraphEdgeSpec[],
): Record<string, { x: number; y: number }> {
  const g = new dagre.graphlib.Graph().setDefaultEdgeLabel(() => ({}));
  g.setGraph({
    rankdir: "LR",
    nodesep: 56,
    ranksep: 100,
    marginx: 28,
    marginy: 28,
  });

  for (const n of nodes) {
    g.setNode(n.id, {
      width: LAYOUT_NODE_WIDTH,
      height: LAYOUT_NODE_HEIGHT,
    });
  }
  for (const e of edges) {
    g.setEdge(e.source, e.target);
  }

  dagre.layout(g);

  const positions: Record<string, { x: number; y: number }> = {};
  for (const n of nodes) {
    const laid = g.node(n.id) as { x?: number; y?: number } | undefined;
    if (laid?.x === undefined || laid?.y === undefined) {
      positions[n.id] = { x: 0, y: 0 };
      continue;
    }
    positions[n.id] = {
      x: laid.x - LAYOUT_NODE_WIDTH / 2,
      y: laid.y - LAYOUT_NODE_HEIGHT / 2,
    };
  }
  return positions;
}
