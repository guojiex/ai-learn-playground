import { Play, RotateCcw } from "lucide-react";
import { Button } from "@/components/ui/Button";
import { Card, CardBody, CardHeader } from "@/components/ui/Card";
import { DEFAULT_FORM, useRunStore, type FormValues } from "@/store/runStore";

const PLATFORMS = ["TikTok", "Shopee", "Instagram", "Facebook", "YouTube"];
const LOCALES = ["id-ID", "en-US", "zh-CN", "th-TH", "vi-VN"];
const STYLES = ["casual", "luxury", "urgency", "review", "storytelling"];

export function InputPanel({ onRun }: { onRun: () => void }) {
  const form = useRunStore((s) => s.form);
  const setField = useRunStore((s) => s.setField);
  const resetForm = useRunStore((s) => s.resetForm);
  const status = useRunStore((s) => s.status);

  return (
    <Card className="flex h-full flex-col">
      <CardHeader
        title="Input"
        subtitle="表单自动保存到当前浏览器"
        actions={
          <Button
            size="sm"
            variant="ghost"
            onClick={resetForm}
            title="重置为默认值"
          >
            <RotateCcw size={14} />
            重置
          </Button>
        }
      />
      <CardBody className="flex flex-1 flex-col gap-4 overflow-auto scrollbar-thin">
        <Field label="商品链接">
          <input
            className={inputCls}
            value={form.product_url}
            onChange={(e) => setField("product_url", e.target.value)}
            placeholder="https://shop.example/..."
          />
        </Field>

        <Field
          label="商品描述"
          hint="URL 优先。描述用于 fallback 模糊匹配。"
        >
          <textarea
            className={`${inputCls} min-h-[72px] resize-y`}
            value={form.product_text}
            onChange={(e) => setField("product_text", e.target.value)}
            placeholder="10000mAh 快充充电宝，便携 USB-C ..."
          />
        </Field>

        <div className="grid grid-cols-2 gap-3">
          <Field label="平台">
            <Select
              value={form.platform}
              options={PLATFORMS}
              onChange={(v) => setField("platform", v)}
            />
          </Field>
          <Field label="地区 / 语言">
            <Select
              value={form.locale}
              options={LOCALES}
              onChange={(v) => setField("locale", v)}
            />
          </Field>
        </div>

        <Field label="风格">
          <Select
            value={form.style}
            options={STYLES}
            onChange={(v) => setField("style", v)}
          />
        </Field>

        <Field label={`最低佣金阈值（当前 ${form.min_commission_rate.toFixed(2)}）`}>
          <input
            className={inputCls}
            type="number"
            min={0}
            max={1}
            step={0.01}
            value={form.min_commission_rate}
            onChange={(e) =>
              setField(
                "min_commission_rate",
                Number.parseFloat(e.target.value) || 0,
              )
            }
          />
        </Field>

        <label className="flex items-center gap-2 text-sm">
          <input
            type="checkbox"
            checked={form.enable_compression}
            onChange={(e) => setField("enable_compression", e.target.checked)}
            className="h-4 w-4 rounded border-border"
          />
          <span>启用检索压缩（compress）</span>
        </label>
      </CardBody>
      <div className="border-t border-border p-4">
        <Button
          className="w-full"
          onClick={onRun}
          disabled={status === "running"}
        >
          <Play size={16} />
          {status === "running" ? "运行中…" : "运行完整流程"}
        </Button>
        <p className="mt-2 text-center text-xs text-fgMuted">
          默认 URL：<code>{DEFAULT_FORM.product_url}</code>
        </p>
      </div>
    </Card>
  );
}

const inputCls =
  "w-full rounded-lg border border-border bg-surfaceMuted px-3 py-2 text-sm text-fg placeholder:text-fgMuted/60 focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary/60";

function Field({
  label,
  hint,
  children,
}: {
  label: string;
  hint?: string;
  children: React.ReactNode;
}) {
  return (
    <label className="flex flex-col gap-1.5">
      <span className="text-xs font-medium uppercase tracking-wide text-fgMuted">
        {label}
      </span>
      {children}
      {hint ? <span className="text-xs text-fgMuted">{hint}</span> : null}
    </label>
  );
}

function Select<K extends keyof FormValues>({
  value,
  options,
  onChange,
}: {
  value: string;
  options: string[];
  onChange: (value: FormValues[K] & string) => void;
}) {
  return (
    <select
      className={inputCls}
      value={value}
      onChange={(e) => onChange(e.target.value as FormValues[K] & string)}
    >
      {!options.includes(value) ? <option value={value}>{value}</option> : null}
      {options.map((o) => (
        <option key={o} value={o}>
          {o}
        </option>
      ))}
    </select>
  );
}
