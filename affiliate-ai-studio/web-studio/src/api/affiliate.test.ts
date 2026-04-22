import { describe, expect, it } from "vitest";
import { normalizeAffiliateCopyPayload } from "./affiliate";

describe("normalizeAffiliateCopyPayload", () => {
  it("maps Python AffiliateCopy.model_dump() to CopyPayload", () => {
    const out = normalizeAffiliateCopyPayload({
      title: "TikTok pick: Power Bank",
      localized_hook: "Built for everyday shoppers.",
      selling_points: ["a", "b", "c"],
      risk_notes: ["Note one"],
    });
    expect(out).not.toBeNull();
    expect(out!.title).toBe("TikTok pick: Power Bank");
    expect(out!.hook).toBe("Built for everyday shoppers.");
    expect(out!.body).toContain("• a");
    expect(out!.body).toContain("Compliance:");
    expect(out!.body).toContain("• Note one");
    expect(out!.cta).toMatch(/Shop now/);
    expect(out!.tags).toEqual(["affiliate", "demo"]);
  });

  it("passes through already-UI-shaped copy", () => {
    const out = normalizeAffiliateCopyPayload({
      title: "T",
      hook: "H",
      body: "B",
      cta: "C",
      tags: ["x"],
    });
    expect(out).toEqual({
      title: "T",
      hook: "H",
      body: "B",
      cta: "C",
      tags: ["x"],
    });
  });

  it("returns null for null", () => {
    expect(normalizeAffiliateCopyPayload(null)).toBeNull();
  });

  it("prefers worker fields even when hook/body are empty strings", () => {
    const out = normalizeAffiliateCopyPayload({
      title: "T",
      hook: "",
      body: "",
      localized_hook: "Real hook",
      selling_points: ["one"],
      risk_notes: [],
    });
    expect(out!.hook).toBe("Real hook");
    expect(out!.body).toContain("• one");
  });
});
