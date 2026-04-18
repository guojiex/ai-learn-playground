import { Card, CardHeader } from "@/components/ui/Card";
import { Tabs } from "@/components/ui/Tabs";
import { Skeleton } from "@/components/ui/Skeleton";
import { useRunStore } from "@/store/runStore";
import { CopyResultCard } from "./CopyResultCard";
import { RetrievalPanel } from "./RetrievalPanel";
import { ToolsPanel } from "./ToolsPanel";
import { DebugPanel } from "./DebugPanel";

export function ResultPanel() {
  const status = useRunStore((s) => s.status);
  const response = useRunStore((s) => s.response);

  return (
    <Card className="flex h-full flex-col">
      <CardHeader
        title="Result"
        subtitle={
          response?.session_id
            ? `session: ${response.session_id}`
            : "等待运行"
        }
      />
      <div className="flex flex-1 flex-col overflow-hidden">
        {status === "running" ? (
          <RunningSkeleton />
        ) : !response ? (
          <EmptyState />
        ) : (
          <Tabs
            items={[
              {
                id: "copy",
                label: "Copy",
                content: (
                  <CopyResultCard
                    result={response.result}
                    productInfo={response.product_info}
                  />
                ),
              },
              {
                id: "retrieval",
                label: `Retrieval`,
                content: <RetrievalPanel data={response.retrieval} />,
              },
              {
                id: "tools",
                label: `Tools (${response.tools?.length ?? 0})`,
                content: <ToolsPanel tools={response.tools} />,
              },
              {
                id: "debug",
                label: "Debug",
                content: <DebugPanel response={response} />,
              },
            ]}
          />
        )}
      </div>
    </Card>
  );
}

function RunningSkeleton() {
  return (
    <div className="space-y-3 p-4">
      <Skeleton className="h-6 w-1/3" />
      <Skeleton className="h-24 w-full" />
      <Skeleton className="h-4 w-2/3" />
      <Skeleton className="h-4 w-1/2" />
      <Skeleton className="h-32 w-full" />
    </div>
  );
}

function EmptyState() {
  return (
    <div className="flex h-full flex-col items-center justify-center gap-2 p-10 text-center text-sm text-fgMuted">
      <p>在左侧填好参数，点击</p>
      <p className="font-medium">运行完整流程</p>
      <p>观察中间 Graph 逐步点亮，这里展示结构化文案。</p>
    </div>
  );
}
