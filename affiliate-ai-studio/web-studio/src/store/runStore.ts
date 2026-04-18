import { create } from "zustand";
import { persist } from "zustand/middleware";
import type {
  ApiError,
  RunAffiliateRequest,
  RunAffiliateResponse,
} from "@/api/affiliate";

export type RunStatus = "idle" | "running" | "success" | "error";

export interface FormValues extends RunAffiliateRequest {}

export interface RunErrorInfo {
  code: string;
  message: string;
  action?: string;
  session_id?: string;
  httpStatus?: number;
}

interface RunState {
  form: FormValues;
  status: RunStatus;
  /** 当前正在演绎的 trace 步骤索引（从 0 开始，-1 表示未开始） */
  activeStep: number;
  response: RunAffiliateResponse | null;
  error: RunErrorInfo | null;
  selectedNode: string | null;
  setField: <K extends keyof FormValues>(key: K, value: FormValues[K]) => void;
  resetForm: () => void;
  startRun: () => void;
  finishRun: (resp: RunAffiliateResponse) => void;
  failRun: (err: ApiError | Error) => void;
  setActiveStep: (index: number) => void;
  selectNode: (nodeId: string | null) => void;
}

export const DEFAULT_FORM: FormValues = {
  product_url: "https://shop.example/power-bank-10000",
  product_text: "",
  platform: "TikTok",
  locale: "id-ID",
  style: "casual",
  min_commission_rate: 0.1,
  enable_compression: true,
  debug: true,
};

export const useRunStore = create<RunState>()(
  persist(
    (set) => ({
      form: { ...DEFAULT_FORM },
      status: "idle",
      activeStep: -1,
      response: null,
      error: null,
      selectedNode: null,
      setField: (key, value) =>
        set((state) => ({ form: { ...state.form, [key]: value } })),
      resetForm: () => set({ form: { ...DEFAULT_FORM } }),
      startRun: () =>
        set({
          status: "running",
          activeStep: -1,
          response: null,
          error: null,
          selectedNode: null,
        }),
      finishRun: (resp) =>
        set({
          status: "success",
          response: resp,
          error: null,
          activeStep: -1,
        }),
      failRun: (err) => {
        const apiErr = err as ApiError;
        const payload = apiErr?.payload?.error;
        set({
          status: "error",
          error: {
            code: payload?.code ?? "client_error",
            message: payload?.message ?? err.message ?? "未知错误",
            action: payload?.action,
            session_id: payload?.session_id,
            httpStatus: apiErr?.status,
          },
        });
      },
      setActiveStep: (index) => set({ activeStep: index }),
      selectNode: (nodeId) => set({ selectedNode: nodeId }),
    }),
    {
      name: "affiliate-ai-studio:v2",
      partialize: (state) => ({ form: state.form }),
      version: 1,
    },
  ),
);
