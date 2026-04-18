import type { HTMLAttributes, ReactNode } from "react";

export function Card({
  className = "",
  ...rest
}: HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={[
        "rounded-2xl border border-border bg-surface shadow-sm",
        className,
      ].join(" ")}
      {...rest}
    />
  );
}

export function CardHeader({
  title,
  subtitle,
  actions,
  className = "",
}: {
  title: ReactNode;
  subtitle?: ReactNode;
  actions?: ReactNode;
  className?: string;
}) {
  return (
    <div
      className={[
        "flex items-start justify-between gap-4 border-b border-border px-5 py-4",
        className,
      ].join(" ")}
    >
      <div>
        <h3 className="text-base font-semibold">{title}</h3>
        {subtitle ? (
          <p className="mt-1 text-sm text-fgMuted">{subtitle}</p>
        ) : null}
      </div>
      {actions ? <div className="shrink-0">{actions}</div> : null}
    </div>
  );
}

export function CardBody({
  className = "",
  ...rest
}: HTMLAttributes<HTMLDivElement>) {
  return <div className={["p-5", className].join(" ")} {...rest} />;
}
