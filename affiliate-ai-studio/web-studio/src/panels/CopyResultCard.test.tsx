import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { CopyResultCard } from "./CopyResultCard";
import type { ResultPayload, ProductInfo } from "@/api/affiliate";

const ACCEPTED: ResultPayload = {
  decision: "accepted",
  reason: "Commission threshold met",
  commission_rate: 0.18,
  copy: {
    title: "Charge Anywhere Power Bank",
    hook: "10000mAh 随身快充",
    body: "通勤一整天不怕没电",
    cta: "立即抢购",
    tags: ["#travel", "#tech"],
  },
};

const REJECTED: ResultPayload = {
  decision: "rejected",
  reason: "Commission below minimum threshold",
  commission_rate: 0.05,
  copy: null,
};

const FALLBACK_PRODUCT: ProductInfo = {
  id: "fallback-unknown",
  title: "fallback title",
  fallback: true,
  category: "unknown",
};

describe("CopyResultCard", () => {
  it("accepted 时渲染 title / hook / body / cta / tags", () => {
    render(<CopyResultCard result={ACCEPTED} />);
    expect(
      screen.getByText("Charge Anywhere Power Bank"),
    ).toBeInTheDocument();
    expect(screen.getByText("10000mAh 随身快充")).toBeInTheDocument();
    expect(screen.getByText("通勤一整天不怕没电")).toBeInTheDocument();
    expect(screen.getByText(/立即抢购/)).toBeInTheDocument();
    expect(screen.getByText(/#travel/)).toBeInTheDocument();
  });

  it("rejected 时展示 reason 且不渲染 copy 字段", () => {
    render(<CopyResultCard result={REJECTED} />);
    expect(screen.getByText("未生成文案")).toBeInTheDocument();
    expect(
      screen.getByText(/Commission below minimum threshold/),
    ).toBeInTheDocument();
    expect(screen.queryByText("Title")).not.toBeInTheDocument();
  });

  it("productInfo.fallback=true 时显示 fallback 徽章", () => {
    render(
      <CopyResultCard result={ACCEPTED} productInfo={FALLBACK_PRODUCT} />,
    );
    expect(screen.getByText("fallback fixture")).toBeInTheDocument();
  });
});
