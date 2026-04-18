import { useId, useState, type ReactNode } from "react";

export interface TabItem {
  id: string;
  label: ReactNode;
  content: ReactNode;
  disabled?: boolean;
}

export function Tabs({
  items,
  defaultTab,
  onChange,
}: {
  items: TabItem[];
  defaultTab?: string;
  onChange?: (id: string) => void;
}) {
  const [active, setActive] = useState(defaultTab ?? items[0]?.id);
  const rootId = useId();

  return (
    <div className="flex h-full flex-col">
      <div
        role="tablist"
        aria-label="result tabs"
        className="flex gap-1 border-b border-border px-3"
      >
        {items.map((item) => {
          const selected = item.id === active;
          return (
            <button
              key={item.id}
              role="tab"
              id={`${rootId}-tab-${item.id}`}
              aria-selected={selected}
              aria-controls={`${rootId}-panel-${item.id}`}
              disabled={item.disabled}
              onClick={() => {
                setActive(item.id);
                onChange?.(item.id);
              }}
              className={[
                "relative px-3 py-2.5 text-sm font-medium transition",
                selected
                  ? "text-primary"
                  : "text-fgMuted hover:text-fg disabled:opacity-40",
              ].join(" ")}
            >
              {item.label}
              {selected ? (
                <span className="absolute inset-x-1 -bottom-px h-0.5 rounded bg-primary" />
              ) : null}
            </button>
          );
        })}
      </div>
      <div className="flex-1 overflow-auto scrollbar-thin">
        {items.map((item) => (
          <div
            key={item.id}
            id={`${rootId}-panel-${item.id}`}
            role="tabpanel"
            aria-labelledby={`${rootId}-tab-${item.id}`}
            hidden={item.id !== active}
          >
            {item.id === active ? item.content : null}
          </div>
        ))}
      </div>
    </div>
  );
}
