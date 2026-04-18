import type { HTMLAttributes } from "react";

type Tone = "neutral" | "success" | "warning" | "danger" | "info";

const toneClass: Record<Tone, string> = {
  neutral:
    "bg-surfaceMuted text-fgMuted border border-border",
  success:
    "bg-success/15 text-success border border-success/40",
  warning:
    "bg-warning/15 text-warning border border-warning/40",
  danger:
    "bg-danger/15 text-danger border border-danger/40",
  info:
    "bg-primary/15 text-primary border border-primary/40",
};

export interface BadgeProps extends HTMLAttributes<HTMLSpanElement> {
  tone?: Tone;
}

export function Badge({ tone = "neutral", className = "", ...rest }: BadgeProps) {
  return (
    <span
      className={[
        "inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium",
        toneClass[tone],
        className,
      ].join(" ")}
      {...rest}
    />
  );
}
