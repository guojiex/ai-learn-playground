from __future__ import annotations

from typing import Any, Literal, Optional

from pydantic import BaseModel, Field


class AffiliateCopy(BaseModel):
    title: str = Field(description="An engaging affiliate copy title")
    selling_points: list[str] = Field(description="Top three selling points")
    localized_hook: str = Field(description="Localized audience hook")
    risk_notes: list[str] = Field(description="Compliance reminders")


class AffiliateDecision(BaseModel):
    decision: Literal["accepted", "rejected", "needs_review"]
    reason: str
    commission_rate: float
    copy_output: Optional[AffiliateCopy] = Field(default=None, alias="copy", serialization_alias="copy")


class TraceStep(BaseModel):
    node: str
    summary: str
    status: str = "ok"
    input_snapshot: dict[str, Any] = Field(default_factory=dict)
    output_snapshot: dict[str, Any] = Field(default_factory=dict)
