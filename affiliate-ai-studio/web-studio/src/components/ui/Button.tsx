import { forwardRef, type ButtonHTMLAttributes } from "react";

type Variant = "primary" | "secondary" | "ghost" | "danger";
type Size = "sm" | "md";

const variantClass: Record<Variant, string> = {
  primary:
    "bg-primary text-primaryFg hover:opacity-90 active:opacity-80 disabled:opacity-60",
  secondary:
    "bg-surfaceMuted text-fg hover:bg-border/40 border border-border disabled:opacity-60",
  ghost:
    "bg-transparent text-fg hover:bg-surfaceMuted disabled:opacity-60",
  danger:
    "bg-danger text-white hover:opacity-90 disabled:opacity-60",
};

const sizeClass: Record<Size, string> = {
  sm: "h-8 px-3 text-sm",
  md: "h-10 px-4 text-sm",
};

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
  size?: Size;
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ variant = "primary", size = "md", className = "", ...rest }, ref) => {
    return (
      <button
        ref={ref}
        className={[
          "inline-flex items-center justify-center gap-2 rounded-lg font-medium transition disabled:cursor-not-allowed",
          variantClass[variant],
          sizeClass[size],
          className,
        ].join(" ")}
        {...rest}
      />
    );
  },
);
Button.displayName = "Button";
