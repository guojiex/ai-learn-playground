import { useEffect, useMemo, useRef } from "react";
import type { RunAffiliateResponse, TraceEntry } from "@/api/affiliate";
import { useRunStore } from "@/store/runStore";
import { GRAPH_NODES, type GraphNodeSpec } from "./graphDefinition";

export type NodeVisualStatus =
  | "idle"
  | "active"
  | "done"
  | "skipped"
  | "error";

export interface NodeVisualState {
  status: NodeVisualStatus;
  traceIndex: number | null;
  trace: TraceEntry | null;
  summary?: string;
}

export const STEP_INTERVAL_MS = 350;

/**
 * 给定后端 trace 和当前动画进度（activeStep），推导每个节点的视觉状态。
 * - 已在 trace 中出现且 index<=activeStep → done（若 index===activeStep 且仍在 running → active）
 * - 出现在 trace 但 index>activeStep → idle（还没演到）
 * - trace 跑完后：没出现在 trace 的节点，如果位于被绕开的分支 → skipped
 */
export function computeNodeStates(
  trace: TraceEntry[] | undefined,
  activeStep: number,
  opts: { isRunning: boolean; decision?: "accepted" | "rejected" | null },
): Record<string, NodeVisualState> {
  const map: Record<string, NodeVisualState> = {};
  const traceNodes = new Set<string>();
  const traceIndexByNode = new Map<string, number>();

  (trace ?? []).forEach((entry, idx) => {
    traceNodes.add(entry.node);
    traceIndexByNode.set(entry.node, idx);
  });

  const runFinished = !opts.isRunning;
  const allVisible = runFinished || activeStep >= (trace?.length ?? 0) - 1;

  for (const spec of GRAPH_NODES) {
    const traceIdx = traceIndexByNode.get(spec.traceKey);
    const traceEntry = traceIdx !== undefined ? trace![traceIdx] : null;

    if (traceIdx === undefined) {
      // 后端没跑到这个节点
      if (allVisible) {
        const skipped = isNodeInSkippedBranch(spec, opts.decision);
        map[spec.id] = {
          status: skipped ? "skipped" : "idle",
          traceIndex: null,
          trace: null,
        };
      } else {
        map[spec.id] = { status: "idle", traceIndex: null, trace: null };
      }
      continue;
    }

    let status: NodeVisualStatus;
    if (traceEntry?.status && traceEntry.status !== "ok") {
      status = "error";
    } else if (opts.isRunning) {
      if (traceIdx < activeStep) status = "done";
      else if (traceIdx === activeStep) status = "active";
      else status = "idle";
    } else {
      status = "done";
    }

    map[spec.id] = {
      status,
      traceIndex: traceIdx,
      trace: traceEntry,
      summary: traceEntry?.summary,
    };
  }

  return map;
}

function isNodeInSkippedBranch(
  spec: GraphNodeSpec,
  decision?: "accepted" | "rejected" | null,
): boolean {
  if (!decision) return false;
  if (decision === "rejected") {
    return [
      "retrieve_policy_context",
      "generate_structured_copy",
      "finalize_accepted",
    ].includes(spec.id);
  }
  return spec.id === "finalize_rejected";
}

/**
 * 在 running -> success 期间，按节奏把 activeStep 从 0 逐步推到 trace.length-1。
 * 超过末尾后触发 store 切换到 success 终态。
 */
export function useTraceAnimator(response: RunAffiliateResponse | null) {
  const status = useRunStore((s) => s.status);
  const activeStep = useRunStore((s) => s.activeStep);
  const setActiveStep = useRunStore((s) => s.setActiveStep);

  // 只在 response 真正变化时启动一次动画
  const currentSessionRef = useRef<string | null>(null);

  useEffect(() => {
    if (!response) return;
    if (response.session_id === currentSessionRef.current) return;
    currentSessionRef.current = response.session_id;

    if (!response.trace || response.trace.length === 0) {
      setActiveStep(-1);
      return;
    }

    setActiveStep(0);
  }, [response, setActiveStep]);

  useEffect(() => {
    if (status !== "success") return;
    if (!response?.trace) return;
    if (activeStep < 0) return;
    if (activeStep >= response.trace.length - 1) return;

    const timer = window.setTimeout(() => {
      setActiveStep(activeStep + 1);
    }, STEP_INTERVAL_MS);
    return () => window.clearTimeout(timer);
  }, [activeStep, response, setActiveStep, status]);
}

export function useNodeStates(): Record<string, NodeVisualState> {
  const status = useRunStore((s) => s.status);
  const response = useRunStore((s) => s.response);
  const activeStep = useRunStore((s) => s.activeStep);

  const decision = response?.result?.decision ?? null;
  const traceLen = response?.trace?.length ?? 0;
  const isRunning =
    status === "success" && activeStep >= 0 && activeStep < traceLen - 1;

  return useMemo(
    () =>
      computeNodeStates(response?.trace, activeStep, {
        isRunning,
        decision,
      }),
    [response, activeStep, isRunning, decision],
  );
}
