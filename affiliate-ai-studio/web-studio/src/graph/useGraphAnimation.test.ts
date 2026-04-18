import { describe, expect, it } from "vitest";
import type { TraceEntry } from "@/api/affiliate";
import { computeNodeStates } from "./useGraphAnimation";

const acceptedTrace: TraceEntry[] = [
  { node: "normalize_input", summary: "", status: "ok" },
  { node: "fetch_product_info", summary: "", status: "ok" },
  { node: "check_commission", summary: "", status: "ok" },
  { node: "retrieve_policy_context", summary: "", status: "ok" },
  { node: "generate_structured_copy", summary: "", status: "ok" },
  { node: "finalize_accepted", summary: "", status: "ok" },
];

const rejectedTrace: TraceEntry[] = [
  { node: "normalize_input", summary: "", status: "ok" },
  { node: "fetch_product_info", summary: "", status: "ok" },
  { node: "check_commission", summary: "", status: "ok" },
  { node: "finalize_rejected", summary: "", status: "ok" },
];

describe("computeNodeStates", () => {
  it("在 running 中把 activeStep 之前的节点标 done、当前 active、之后 idle", () => {
    const states = computeNodeStates(acceptedTrace, 2, {
      isRunning: true,
      decision: "accepted",
    });
    expect(states.normalize_input.status).toBe("done");
    expect(states.fetch_product_info.status).toBe("done");
    expect(states.check_commission.status).toBe("active");
    expect(states.retrieve_policy_context.status).toBe("idle");
    expect(states.generate_structured_copy.status).toBe("idle");
    expect(states.finalize_accepted.status).toBe("idle");
  });

  it("跑完（accepted）时 finalize_rejected 应标 skipped", () => {
    const states = computeNodeStates(acceptedTrace, acceptedTrace.length - 1, {
      isRunning: false,
      decision: "accepted",
    });
    expect(states.finalize_rejected.status).toBe("skipped");
    expect(states.finalize_accepted.status).toBe("done");
  });

  it("跑完（rejected）时 retrieve/generate/finalize_accepted 应标 skipped", () => {
    const states = computeNodeStates(rejectedTrace, rejectedTrace.length - 1, {
      isRunning: false,
      decision: "rejected",
    });
    expect(states.retrieve_policy_context.status).toBe("skipped");
    expect(states.generate_structured_copy.status).toBe("skipped");
    expect(states.finalize_accepted.status).toBe("skipped");
    expect(states.finalize_rejected.status).toBe("done");
  });

  it("没有 decision 且没跑到的节点保持 idle", () => {
    const states = computeNodeStates([], -1, {
      isRunning: false,
      decision: null,
    });
    for (const id of Object.keys(states)) {
      expect(states[id].status).toBe("idle");
    }
  });

  it("trace 中 status 非 ok 的节点应标 error", () => {
    const trace: TraceEntry[] = [
      { node: "normalize_input", summary: "", status: "ok" },
      { node: "fetch_product_info", summary: "", status: "error" },
    ];
    const states = computeNodeStates(trace, trace.length - 1, {
      isRunning: false,
      decision: null,
    });
    expect(states.fetch_product_info.status).toBe("error");
  });
});
