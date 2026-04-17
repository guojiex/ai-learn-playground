import type { Config } from "tailwindcss";

const config: Config = {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  darkMode: "class",
  theme: {
    extend: {
      colors: {
        background: "rgb(var(--bg) / <alpha-value>)",
        surface: "rgb(var(--surface) / <alpha-value>)",
        surfaceMuted: "rgb(var(--surface-muted) / <alpha-value>)",
        border: "rgb(var(--border) / <alpha-value>)",
        fg: "rgb(var(--fg) / <alpha-value>)",
        fgMuted: "rgb(var(--fg-muted) / <alpha-value>)",
        primary: "rgb(var(--primary) / <alpha-value>)",
        primaryFg: "rgb(var(--primary-fg) / <alpha-value>)",
        success: "rgb(var(--success) / <alpha-value>)",
        warning: "rgb(var(--warning) / <alpha-value>)",
        danger: "rgb(var(--danger) / <alpha-value>)",
      },
      fontFamily: {
        sans: [
          "-apple-system",
          "BlinkMacSystemFont",
          "Segoe UI",
          "Helvetica Neue",
          "Arial",
          "sans-serif",
        ],
        mono: [
          "ui-monospace",
          "SFMono-Regular",
          "Menlo",
          "Consolas",
          "monospace",
        ],
      },
      keyframes: {
        pulseBorder: {
          "0%, 100%": { boxShadow: "0 0 0 0 rgba(91,140,255,0.55)" },
          "50%": { boxShadow: "0 0 0 6px rgba(91,140,255,0)" },
        },
      },
      animation: {
        "pulse-border": "pulseBorder 1.4s ease-in-out infinite",
      },
    },
  },
  plugins: [],
};

export default config;
