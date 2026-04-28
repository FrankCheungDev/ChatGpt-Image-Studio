"use client";

import { useEffect, useRef, useState } from "react";
import {
  Download,
  ExternalLink,
  Eye,
  History,
  LoaderCircle,
  RefreshCw,
} from "lucide-react";
import { toast } from "sonner";

import { buildConversationSourceLabel, buildImageDataUrl, buildSourceImageUrl } from "@/app/image/view-utils";
import { AppImage } from "@/components/app-image";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { cn } from "@/lib/utils";
import {
  fetchPlatformImageConversation,
  fetchPlatformImageConversations,
} from "@/lib/api";
import {
  normalizeConversation,
  type ImageConversation,
  type ImageConversationTurn,
  type StoredImage,
  type StoredSourceImage,
} from "@/store/image-conversations";
import { formatImageErrorMessage } from "@/app/image/submit-utils";

function formatTime(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value || "-";
  }
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

function formatDateTime(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value || "-";
  }
  return new Intl.DateTimeFormat("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  }).format(date);
}

function formatStatusLabel(value: string) {
  if (value === "success") {
    return "成功";
  }
  if (value === "error") {
    return "失败";
  }
  if (value === "generating" || value === "loading") {
    return "处理中";
  }
  return value || "-";
}

function statusVariant(value: string): "success" | "danger" | "warning" | "secondary" {
  if (value === "success") {
    return "success";
  }
  if (value === "error") {
    return "danger";
  }
  if (value === "generating" || value === "loading") {
    return "warning";
  }
  return "secondary";
}

function formatModeLabel(value: string) {
  if (value === "generate") {
    return "生成";
  }
  if (value === "edit") {
    return "编辑";
  }
  return value || "-";
}

function formatSizeLabel(value?: string) {
  return String(value || "")
    .trim()
    .replace("x", "X");
}

function buildDownloadName(createdAt: string, turnId: string, index: number) {
  const date = new Date(createdAt);
  const safeIndex = String(index + 1).padStart(2, "0");
  if (Number.isNaN(date.getTime())) {
    return `chatgpt-image-${turnId.slice(0, 8)}-${safeIndex}.png`;
  }

  const yyyy = String(date.getFullYear());
  const mm = String(date.getMonth() + 1).padStart(2, "0");
  const dd = String(date.getDate()).padStart(2, "0");
  const hh = String(date.getHours()).padStart(2, "0");
  const min = String(date.getMinutes()).padStart(2, "0");
  const sec = String(date.getSeconds()).padStart(2, "0");
  return `chatgpt-image-${yyyy}${mm}${dd}-${hh}${min}${sec}-${safeIndex}.png`;
}

function DetailMeta({
  label,
  value,
}: {
  label: string;
  value: string | number | undefined | null;
}) {
  return (
    <div className="min-w-0 rounded-xl border border-stone-200 bg-white px-3 py-2">
      <div className="text-[11px] font-medium text-stone-400">{label}</div>
      <div className="mt-1 truncate text-sm font-medium text-stone-800" title={String(value || "")}>
        {value || "-"}
      </div>
    </div>
  );
}

function SourceImageGrid({ items }: { items: StoredSourceImage[] }) {
  if (items.length === 0) {
    return null;
  }

  return (
    <div className="mt-3 flex flex-wrap gap-3">
      {items.map((source) => {
        const imageUrl = buildSourceImageUrl(source);
        return (
          <a
            key={source.id}
            href={imageUrl || undefined}
            target="_blank"
            rel="noreferrer"
            className="group w-[144px] overflow-hidden rounded-xl border border-stone-200 bg-white text-left transition hover:border-stone-300 hover:bg-stone-50"
          >
            <div className="border-b border-stone-100 px-3 py-2 text-[11px] font-medium text-stone-500">
              {buildConversationSourceLabel(source)}
            </div>
            {imageUrl ? (
              <AppImage
                src={imageUrl}
                alt={source.name || buildConversationSourceLabel(source)}
                className="block h-24 w-full bg-stone-50 object-contain"
              />
            ) : (
              <div className="flex h-24 items-center justify-center bg-stone-50 px-3 text-center text-xs text-stone-400">
                无法预览
              </div>
            )}
            <div className="truncate px-3 py-2 text-xs text-stone-500" title={source.name}>
              {source.name || source.id}
            </div>
          </a>
        );
      })}
    </div>
  );
}

function ResultImageCard({
  image,
  turn,
  index,
}: {
  image: StoredImage;
  turn: ImageConversationTurn;
  index: number;
}) {
  const imageUrl = buildImageDataUrl(image);
  const downloadName = buildDownloadName(turn.createdAt, turn.id, index);

  if (image.status === "error") {
    return (
      <div className="flex min-h-[220px] items-center justify-center rounded-xl border border-rose-100 bg-rose-50 px-5 py-6 text-center text-sm leading-6 text-rose-700">
        {formatImageErrorMessage(image.error || turn.error || "处理失败")}
      </div>
    );
  }

  if (image.status !== "success" || !imageUrl) {
    return (
      <div className="flex min-h-[220px] flex-col items-center justify-center gap-3 rounded-xl border border-stone-200 bg-stone-50 px-5 py-6 text-center text-sm text-stone-500">
        <LoaderCircle className="size-5 animate-spin" />
        图片处理中
      </div>
    );
  }

  return (
    <div className="overflow-hidden rounded-xl border border-stone-200 bg-white">
      <a href={imageUrl} target="_blank" rel="noreferrer" className="block bg-stone-50">
        <AppImage
          src={imageUrl}
          alt={`Generated result ${index + 1}`}
          className="block h-auto max-h-[360px] w-full object-contain"
        />
      </a>
      <div className="flex flex-wrap items-center justify-between gap-2 border-t border-stone-100 px-3 py-2">
        <div className="truncate text-xs text-stone-500">
          {image.revised_prompt || `结果 ${index + 1}`}
        </div>
        <div className="flex items-center gap-1.5">
          <a
            href={imageUrl}
            target="_blank"
            rel="noreferrer"
            className="inline-flex size-8 items-center justify-center rounded-full border border-stone-200 bg-white text-stone-600 transition hover:bg-stone-100 hover:text-stone-900"
            title="打开"
            aria-label="打开"
          >
            <ExternalLink className="size-4" />
          </a>
          <a
            href={imageUrl}
            download={downloadName}
            className="inline-flex size-8 items-center justify-center rounded-full border border-stone-200 bg-white text-stone-600 transition hover:bg-stone-100 hover:text-stone-900"
            title="下载"
            aria-label="下载"
          >
            <Download className="size-4" />
          </a>
        </div>
      </div>
    </div>
  );
}

function ConversationDetail({ item }: { item: ImageConversation }) {
  const turns = item.turns || [];

  return (
    <div className="space-y-5">
      <div className="grid gap-3 rounded-2xl border border-stone-200 bg-stone-50 p-3 sm:grid-cols-2 lg:grid-cols-4">
        <DetailMeta label="用户 ID" value={item.userId} />
        <DetailMeta label="创建时间" value={formatDateTime(item.createdAt)} />
        <DetailMeta label="会话 ID" value={item.id} />
        <div className="rounded-xl border border-stone-200 bg-white px-3 py-2">
          <div className="text-[11px] font-medium text-stone-400">状态</div>
          <Badge variant={statusVariant(item.status)} className="mt-1 rounded-md px-2 py-1">
            {formatStatusLabel(item.status)}
          </Badge>
        </div>
      </div>

      {turns.map((turn, turnIndex) => (
        <article key={turn.id} className="rounded-2xl border border-stone-200 bg-white p-4">
          <div className="flex flex-col gap-3 border-b border-stone-100 pb-4 lg:flex-row lg:items-start lg:justify-between">
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-2">
                <span className="text-sm font-semibold text-stone-900">第 {turnIndex + 1} 轮</span>
                <Badge variant={statusVariant(turn.status)} className="rounded-md px-2 py-1">
                  {formatStatusLabel(turn.status)}
                </Badge>
              </div>
              <div className="mt-2 flex flex-wrap gap-2 text-xs text-stone-500">
                <span className="rounded-full bg-stone-100 px-3 py-1.5">{formatModeLabel(turn.mode)}</span>
                <span className="rounded-full bg-stone-100 px-3 py-1.5">{turn.model}</span>
                <span className="rounded-full bg-stone-100 px-3 py-1.5">{turn.count} 张</span>
                {turn.size ? (
                  <span className="rounded-full bg-stone-100 px-3 py-1.5">{formatSizeLabel(turn.size)}</span>
                ) : null}
                {turn.quality ? (
                  <span className="rounded-full bg-stone-100 px-3 py-1.5">Quality {turn.quality}</span>
                ) : null}
                {turn.scale ? (
                  <span className="rounded-full bg-stone-100 px-3 py-1.5">{turn.scale}</span>
                ) : null}
                <span className="rounded-full bg-stone-100 px-3 py-1.5">{formatDateTime(turn.createdAt)}</span>
              </div>
            </div>
            <div className="text-sm font-medium text-stone-500">{turn.images.length} 张结果</div>
          </div>

          {turn.sourceImages && turn.sourceImages.length > 0 ? (
            <div className="pt-4">
              <div className="text-xs font-medium text-stone-400">输入图片</div>
              <SourceImageGrid items={turn.sourceImages} />
            </div>
          ) : null}

          <div className="pt-4">
            <div className="text-xs font-medium text-stone-400">提示词</div>
            <div className="mt-2 whitespace-pre-wrap break-words rounded-xl bg-stone-50 px-4 py-3 text-sm leading-6 text-stone-800">
              {turn.prompt || "无额外提示词"}
            </div>
          </div>

          {turn.error ? (
            <div className="mt-4 whitespace-pre-line rounded-xl border border-rose-100 bg-rose-50 px-4 py-3 text-sm leading-6 text-rose-700">
              {formatImageErrorMessage(turn.error)}
            </div>
          ) : null}

          {turn.images.length > 0 ? (
            <div
              className={cn(
                "mt-4 grid gap-4",
                turn.images.length === 1 ? "grid-cols-1" : "grid-cols-1 lg:grid-cols-2",
              )}
            >
              {turn.images.map((image, index) => (
                <ResultImageCard key={image.id} image={image} turn={turn} index={index} />
              ))}
            </div>
          ) : null}
        </article>
      ))}
    </div>
  );
}

export default function PlatformHistoryPage() {
  const [items, setItems] = useState<ImageConversation[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [selectedConversation, setSelectedConversation] =
    useState<ImageConversation | null>(null);
  const [isDetailOpen, setIsDetailOpen] = useState(false);
  const [isDetailLoading, setIsDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState("");
  const detailRequestRef = useRef(0);

  const loadItems = async () => {
    setIsLoading(true);
    try {
      const payload = await fetchPlatformImageConversations();
      setItems(payload.items.map(normalizeConversation));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "读取平台历史失败");
    } finally {
      setIsLoading(false);
    }
  };

  const openConversationDetail = async (id: string) => {
    const requestId = detailRequestRef.current + 1;
    detailRequestRef.current = requestId;
    setSelectedId(id);
    setSelectedConversation(null);
    setDetailError("");
    setIsDetailOpen(true);
    setIsDetailLoading(true);
    try {
      const payload = await fetchPlatformImageConversation(id);
      if (detailRequestRef.current === requestId) {
        setSelectedConversation(normalizeConversation(payload.item));
      }
    } catch (error) {
      const message = error instanceof Error ? error.message : "读取平台历史详情失败";
      if (detailRequestRef.current === requestId) {
        setDetailError(message);
      }
      toast.error(message);
    } finally {
      if (detailRequestRef.current === requestId) {
        setIsDetailLoading(false);
      }
    }
  };

  useEffect(() => {
    void loadItems();
  }, []);

  return (
    <>
      <section className="h-full overflow-y-auto">
        <div className="mx-auto max-w-[1280px] px-1 py-1">
          <div className="rounded-[28px] border border-stone-200 bg-[#fcfcfb] px-4 py-5 shadow-[0_14px_40px_rgba(15,23,42,0.05)] sm:px-6">
            <div className="flex flex-col gap-6">
              <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
                <div className="flex items-center gap-4">
                  <span className="inline-flex size-12 items-center justify-center rounded-[18px] bg-stone-950 text-white">
                    <History className="size-5" />
                  </span>
                  <div>
                    <h1 className="text-2xl font-semibold tracking-tight text-stone-950">平台历史</h1>
                  </div>
                </div>
                <Button
                  type="button"
                  variant="outline"
                  className="h-10 rounded-full border-stone-200 bg-white"
                  onClick={() => void loadItems()}
                  disabled={isLoading}
                >
                  <RefreshCw className={isLoading ? "size-4 animate-spin" : "size-4"} />
                  刷新
                </Button>
              </div>

              <div className="overflow-hidden rounded-2xl border border-stone-200 bg-white">
                <div className="hidden grid-cols-[150px_1fr_120px_120px_120px_96px] border-b border-stone-100 bg-stone-50 px-4 py-3 text-xs font-medium uppercase tracking-[0.16em] text-stone-400 lg:grid">
                  <div>时间</div>
                  <div>内容</div>
                  <div>用户</div>
                  <div>状态</div>
                  <div>图片</div>
                  <div>操作</div>
                </div>
                <div className="divide-y divide-stone-100">
                  {items.map((item) => {
                    const itemLoading = isDetailLoading && selectedId === item.id;
                    return (
                      <button
                        key={item.id}
                        type="button"
                        className="grid w-full gap-3 px-4 py-4 text-left text-sm transition hover:bg-stone-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-stone-400 lg:grid-cols-[150px_1fr_120px_120px_120px_96px] lg:items-center"
                        onClick={() => void openConversationDetail(item.id)}
                        aria-label={`查看平台历史 ${item.title || item.prompt || item.id}`}
                      >
                        <div className="text-stone-600">{formatTime(item.createdAt)}</div>
                        <div className="min-w-0">
                          <div className="truncate font-medium text-stone-900">{item.title || item.prompt || item.id}</div>
                          <div className="mt-1 line-clamp-2 text-xs leading-5 text-stone-500">{item.prompt || "-"}</div>
                        </div>
                        <div className="truncate text-xs text-stone-500" title={String(item.userId || "")}>
                          {item.userId || "-"}
                        </div>
                        <Badge variant={statusVariant(item.status)} className="w-fit rounded-md px-2 py-1">
                          {formatStatusLabel(item.status)}
                        </Badge>
                        <div className="text-stone-600">{item.images?.length ?? 0} 张</div>
                        <div className="inline-flex w-fit items-center gap-1.5 rounded-full border border-stone-200 bg-white px-3 py-1.5 text-xs font-medium text-stone-600">
                          {itemLoading ? <LoaderCircle className="size-3.5 animate-spin" /> : <Eye className="size-3.5" />}
                          查看
                        </div>
                      </button>
                    );
                  })}
                  {!isLoading && items.length === 0 ? (
                    <div className="px-4 py-12 text-center text-sm text-stone-500">暂无平台历史</div>
                  ) : null}
                  {isLoading ? (
                    <div className="flex items-center justify-center gap-2 px-4 py-12 text-sm text-stone-500">
                      <LoaderCircle className="size-4 animate-spin" />
                      正在读取平台历史
                    </div>
                  ) : null}
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      <Dialog
        open={isDetailOpen}
        onOpenChange={(open) => {
          setIsDetailOpen(open);
          if (!open) {
            detailRequestRef.current += 1;
            setSelectedId(null);
            setSelectedConversation(null);
            setDetailError("");
            setIsDetailLoading(false);
          }
        }}
      >
        <DialogContent className="grid max-h-[90vh] w-[min(96vw,1120px)] grid-rows-[auto_minmax(0,1fr)] gap-0 overflow-hidden rounded-2xl p-0">
          <DialogHeader className="border-b border-stone-100 px-5 py-4 pr-12">
            <DialogTitle className="truncate text-lg">
              {selectedConversation?.title || selectedConversation?.prompt || selectedId || "平台历史详情"}
            </DialogTitle>
            <DialogDescription className="truncate">
              {selectedConversation
                ? `${selectedConversation.userId || "-"} · ${formatDateTime(selectedConversation.createdAt)}`
                : selectedId || "正在读取"}
            </DialogDescription>
          </DialogHeader>
          <div className="min-h-0 overflow-y-auto bg-[#fcfcfb] px-5 py-5">
            {isDetailLoading ? (
              <div className="flex min-h-[360px] flex-col items-center justify-center gap-3 text-sm text-stone-500">
                <LoaderCircle className="size-6 animate-spin" />
                正在读取详情
              </div>
            ) : detailError ? (
              <div className="rounded-2xl border border-rose-100 bg-rose-50 px-5 py-4 text-sm leading-6 text-rose-700">
                {detailError}
              </div>
            ) : selectedConversation ? (
              <ConversationDetail item={selectedConversation} />
            ) : (
              <div className="px-4 py-12 text-center text-sm text-stone-500">请选择一条平台历史</div>
            )}
          </div>
        </DialogContent>
      </Dialog>
    </>
  );
}
