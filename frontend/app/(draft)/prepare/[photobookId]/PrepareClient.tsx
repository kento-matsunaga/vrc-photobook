// /prepare/<photobookId> Client Component（Upload Staging 画面）。
//
// 設計参照:
//   - docs/plan/m2-upload-staging-plan.md §6
//   - plan v2 m2-prepare-resilience-and-throughput §3.4（β-3 Frontend）
//
// 役割:
//   - 複数画像の一括選択 + concurrency=2 並列 upload + tile 状態管理
//   - 5 sec polling + exponential backoff + max 10 min duration + Page Visibility API
//   - SSR initialView.images からの reload 復元 + polling 中の server merge
//   - 「編集へ進む」押下時に attach-images bulk API → /edit/<id> 遷移
//
// セキュリティ:
//   - raw imageId / storage_key / upload URL を console / DOM / data-testid / aria-label に出さない
//   - Turnstile token / verification token / Cookie 値は state にも保持しない（catch 後即破棄）
//   - failed 時の reason は user-friendly mapping のみ
//   - localStorage は filename 補助だけに使う（imageId は key 保管のみ、UI 露出させない）

"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { ImageTile } from "@/components/Prepare/ImageTile";
import {
  addFiles,
  canProceedToEdit,
  emptyQueue,
  isAllSettled,
  markStatus,
  mergeServerImages,
  pollDelaySeconds,
  reconcileWithServer,
  selectNextRunnable,
  summary,
  type QueueState,
  type QueueTile,
  type ServerImageForMerge,
  type TileFailureReason,
} from "@/components/Prepare/UploadQueue";
import { TurnstileWidget } from "@/components/TurnstileWidget";
import {
  fetchEditViewClient,
  isEditApiError,
  prepareAttachImages,
  type EditView,
} from "@/lib/editPhotobook";
import {
  CompressionError,
  compressImageForUpload,
} from "@/lib/imageCompression";
import { lookupLabel, rememberLabel } from "@/lib/prepareLocalLabels";
import {
  completeUpload,
  issueUploadIntent,
  issueUploadVerification,
  putToR2,
  sourceFormatOf,
  validateFile,
} from "@/lib/upload";
import {
  createUploadVerificationCache,
  type UploadVerificationCache,
} from "@/lib/uploadVerificationCache";

const CONCURRENCY = 2;
const MAX_TILES = 20;
const MAX_POLL_DURATION_MS = 10 * 60 * 1000;
const SLOW_NOTICE_THRESHOLD_MS = 10 * 60 * 1000;

type Props = {
  photobookId: string;
  turnstileSiteKey: string;
  initialView: EditView;
};

type ViewState = {
  version: number;
  processingCount: number;
  failedCount: number;
  placedImageIds: Set<string>;
  placedPhotoCount: number;
  images: ServerImageForMerge[];
};

function viewToState(v: EditView): ViewState {
  const placed = new Set<string>();
  let count = 0;
  for (const page of v.pages) {
    for (const photo of page.photos) {
      placed.add(photo.imageId);
      count++;
    }
  }
  const images: ServerImageForMerge[] = v.images.map((img) => ({
    imageId: img.imageId,
    status: img.status,
    originalByteSize: img.originalByteSize,
    createdAt: img.createdAt,
  }));
  return {
    version: v.version,
    processingCount: v.processingCount,
    failedCount: v.failedCount,
    placedImageIds: placed,
    placedPhotoCount: count,
    images,
  };
}

function newTileId(): string {
  return `t-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
}

function mapUploadErrorToReason(kind: string): TileFailureReason {
  switch (kind) {
    case "verification_failed":
      return "verification_failed";
    case "rate_limited":
      return "rate_limited";
    case "invalid_parameters":
      return "validation_failed";
    case "upload_failed":
      return "upload_failed";
    case "complete_failed":
      return "complete_failed";
    case "validation_failed":
      return "validation_failed";
    case "network":
      return "network";
    default:
      return "unknown";
  }
}

export function PrepareClient({ photobookId, turnstileSiteKey, initialView }: Props) {
  const [view, setView] = useState<ViewState>(() => viewToState(initialView));
  // initial mount で server 復元 tile を生成しておく（reload 後も「全部消えた」状態にしない）。
  const [queue, setQueue] = useState<QueueState>(() => {
    const initialState = viewToState(initialView);
    return mergeServerImages(
      emptyQueue(),
      initialState.images,
      initialState.placedImageIds,
      (imgId) => lookupLabel(photobookId, imgId),
      newTileId,
    );
  });
  const [turnstileToken, setTurnstileToken] = useState<string>("");
  const [globalError, setGlobalError] = useState<string>("");
  const [proceeding, setProceeding] = useState<boolean>(false);
  const [proceedError, setProceedError] = useState<string>("");

  const queueRef = useRef<QueueState>(queue);
  useEffect(() => {
    queueRef.current = queue;
  }, [queue]);

  const verificationCacheRef = useRef<UploadVerificationCache | null>(null);
  if (verificationCacheRef.current === null) {
    verificationCacheRef.current = createUploadVerificationCache((tok) =>
      issueUploadVerification(photobookId, tok),
    );
  }

  const handleTurnstileVerify = useCallback((tok: string) => {
    setTurnstileToken(tok);
  }, []);
  const handleTurnstileError = useCallback(() => {
    setTurnstileToken("");
  }, []);
  const handleTurnstileExpired = useCallback(() => {
    setTurnstileToken("");
    verificationCacheRef.current?.reset();
  }, []);
  const handleTurnstileTimeout = useCallback(() => {
    setTurnstileToken("");
    verificationCacheRef.current?.reset();
  }, []);

  const onFileSelect = useCallback(
    async (e: React.ChangeEvent<HTMLInputElement>) => {
      const incoming = e.target.files ? Array.from(e.target.files) : [];
      e.target.value = "";
      if (incoming.length === 0) return;

      const accepted: File[] = [];
      let rejectedFormat = 0;
      let rejectedTooHuge = 0;
      let recompressed = 0;
      for (const f of incoming) {
        if (
          f.type !== "image/jpeg" &&
          f.type !== "image/png" &&
          f.type !== "image/webp"
        ) {
          rejectedFormat++;
          continue;
        }
        try {
          const result = await compressImageForUpload(f);
          if (result.recompressed) recompressed++;
          const v = validateFile(result.file);
          if (v) {
            rejectedTooHuge++;
            continue;
          }
          accepted.push(result.file);
        } catch (err) {
          if (err instanceof CompressionError) {
            rejectedTooHuge++;
          } else {
            rejectedTooHuge++;
          }
        }
      }

      setQueue((q) => {
        const remaining = MAX_TILES - q.tiles.length;
        const taken = accepted.slice(0, Math.max(0, remaining));
        return addFiles(q, taken, newTileId);
      });

      if (rejectedFormat > 0 || rejectedTooHuge > 0) {
        const parts: string[] = [];
        if (rejectedFormat > 0) {
          parts.push(`${rejectedFormat} 枚は対応していない形式（JPEG / PNG / WebP のみ、HEIC / HEIF 未対応）`);
        }
        if (rejectedTooHuge > 0) {
          parts.push(`${rejectedTooHuge} 枚はサイズ過大で取り込めませんでした（圧縮しても 10MB 以下に収まらず、または 50MB を超過）`);
        }
        setGlobalError(parts.join(" / "));
      } else if (recompressed > 0) {
        setGlobalError("");
      } else {
        setGlobalError("");
      }
    },
    [],
  );

  const runUpload = useCallback(
    async (tile: QueueTile) => {
      const tok = turnstileToken;
      if (typeof tok !== "string" || tok.trim() === "") {
        setQueue((q) => markStatus(q, tile.id, { kind: "failed", reason: "verification_failed" }));
        return;
      }
      const file = tile.file;
      if (file === undefined) {
        // server 復元 tile を upload chain に通すことはない（origin guard）
        return;
      }

      try {
        setQueue((q) => markStatus(q, tile.id, { kind: "verifying" }));

        const cache = verificationCacheRef.current;
        if (cache === null) {
          throw { kind: "unknown" };
        }
        const vtok = await cache.ensure(tok);

        const sf = sourceFormatOf(file.type);
        if (sf === null) {
          setQueue((q) =>
            markStatus(q, tile.id, { kind: "failed", reason: "validation_failed" }),
          );
          return;
        }
        const intent = await issueUploadIntent(
          photobookId,
          vtok,
          file.type,
          file.size,
          sf,
        );
        // upload 開始時点で imageId を取得できる。filename を localStorage に保存。
        rememberLabel(photobookId, intent.imageId, file.name);
        setQueue((q) => markStatus(q, tile.id, { kind: "uploading" }));
        await putToR2(intent.uploadUrl, file.type, file);

        setQueue((q) => markStatus(q, tile.id, { kind: "completing" }));
        await completeUpload(photobookId, intent.imageId, intent.storageKey);

        setQueue((q) =>
          markStatus(q, tile.id, { kind: "processing", imageId: intent.imageId }),
        );
      } catch (e) {
        const kind = (e as { kind?: string })?.kind ?? "unknown";
        setQueue((q) =>
          markStatus(q, tile.id, {
            kind: "failed",
            reason: mapUploadErrorToReason(kind),
          }),
        );
        if (kind === "verification_failed" || kind === "rate_limited") {
          verificationCacheRef.current?.reset();
          setTurnstileToken("");
        }
      }
    },
    [photobookId, turnstileToken],
  );

  useEffect(() => {
    const next = selectNextRunnable(queue, CONCURRENCY);
    if (next === null) return;
    if (typeof turnstileToken !== "string" || turnstileToken.trim() === "") return;
    void runUpload(next);
  }, [queue, turnstileToken, runUpload]);

  // ===== polling: edit-view を再取得し、queue を server で reconcile / merge =====
  const tickRef = useRef<number>(0);
  const startedAtRef = useRef<number>(0);
  const visibleRef = useRef<boolean>(true);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const stopPolling = useCallback(() => {
    if (timerRef.current !== null) {
      clearTimeout(timerRef.current);
      timerRef.current = null;
    }
  }, []);

  const pollOnce = useCallback(async () => {
    try {
      // β-3: client polling は credentials: include 経路を使う（401 で止まらない）
      const v = await fetchEditViewClient(photobookId);
      const next = viewToState(v);
      setView(next);
      setQueue((q) => {
        const reconciled = reconcileWithServer(q, next.placedImageIds, next.processingCount);
        return mergeServerImages(
          reconciled,
          next.images,
          next.placedImageIds,
          (imgId) => lookupLabel(photobookId, imgId),
          newTileId,
        );
      });
    } catch {
      // 失敗詳細は外に出さない（敵対者対策）。次の tick で再試行。
    }
  }, [photobookId]);

  const schedulePoll = useCallback(() => {
    stopPolling();
    if (!visibleRef.current) return;
    if (Date.now() - startedAtRef.current > MAX_POLL_DURATION_MS) return;
    const delay = pollDelaySeconds(tickRef.current) * 1000;
    timerRef.current = setTimeout(() => {
      void pollOnce().finally(() => {
        tickRef.current += 1;
        schedulePoll();
      });
    }, delay);
  }, [pollOnce, stopPolling]);

  const needPolling =
    view.processingCount > 0 ||
    queue.tiles.some(
      (t) =>
        t.status.kind === "processing" ||
        t.status.kind === "verifying" ||
        t.status.kind === "uploading" ||
        t.status.kind === "completing" ||
        t.status.kind === "queued",
    );

  useEffect(() => {
    if (!needPolling) {
      stopPolling();
      return;
    }
    if (startedAtRef.current === 0) {
      startedAtRef.current = Date.now();
      tickRef.current = 0;
    }
    schedulePoll();
    return () => stopPolling();
  }, [needPolling, schedulePoll, stopPolling]);

  useEffect(() => {
    const onVisChange = () => {
      visibleRef.current = !document.hidden;
      if (!document.hidden && needPolling) {
        tickRef.current = 0;
        schedulePoll();
      } else if (document.hidden) {
        stopPolling();
      }
    };
    document.addEventListener("visibilitychange", onVisChange);
    return () => document.removeEventListener("visibilitychange", onVisChange);
  }, [needPolling, schedulePoll, stopPolling]);

  // 10 分超過時の遅延通知（plan v2 §3.4 P0-c の progress UI 要件）
  const [slowNotice, setSlowNotice] = useState<boolean>(false);
  useEffect(() => {
    if (!needPolling) {
      setSlowNotice(false);
      return;
    }
    const t = setInterval(() => {
      if (startedAtRef.current === 0) return;
      const elapsed = Date.now() - startedAtRef.current;
      if (elapsed > SLOW_NOTICE_THRESHOLD_MS) {
        setSlowNotice(true);
      }
    }, 5000);
    return () => clearInterval(t);
  }, [needPolling]);

  // ===== UI rendering =====
  const sum = useMemo(() => summary(queue), [queue]);
  const proceed =
    !proceeding &&
    canProceedToEdit(queue, view.processingCount, view.placedPhotoCount);
  const tilesAtCap = queue.tiles.length >= MAX_TILES;
  const turnstileReady = turnstileToken !== "" && turnstileToken.trim() !== "";

  const onProceed = useCallback(async () => {
    if (proceeding) return;
    setProceedError("");
    setProceeding(true);
    try {
      await prepareAttachImages(photobookId, view.version);
      window.location.assign(`/edit/${photobookId}`);
    } catch (e) {
      let msg = "編集画面へ進めませんでした。少し時間をおいて再度お試しください。";
      if (isEditApiError(e)) {
        switch (e.kind) {
          case "unauthorized":
            msg = "セッションが切れています。トップから再度入り直してください。";
            break;
          case "not_found":
            msg = "対象のフォトブックが見つかりません。";
            break;
          case "version_conflict":
            msg = "他の操作によって状態が変わりました。画面を再読み込みしてください。";
            break;
          case "bad_request":
            msg = "リクエスト内容に問題があります。再度お試しください。";
            break;
        }
      }
      setProceedError(msg);
      setProceeding(false);
    }
  }, [photobookId, proceeding, view.version]);

  // n/m progress: completed / total（local + server-restored）
  const totalKnown = sum.total + view.placedPhotoCount;
  const completedKnown = sum.available + view.placedPhotoCount;

  return (
    <main
      data-testid="prepare-page"
      className="mx-auto min-h-screen w-full max-w-screen-md space-y-6 px-4 py-6 sm:px-6"
    >
      <header className="space-y-2 border-b border-divider pb-4">
        <h1 className="text-h1 text-ink">写真をまとめて追加</h1>
        <p className="text-sm text-ink-medium">
          フォトブックに使う写真をまとめて選んでください。すべての写真が「完了」になったら、
          「編集へ進む」で編集画面に移動できます。
        </p>
        <p className="text-xs text-ink-soft">
          JPEG / PNG / WebP、最大 10MB / 1 枚、最大 {MAX_TILES} 枚まで（HEIC / HEIF 未対応）
        </p>
      </header>

      <section
        data-testid="prepare-picker"
        className="space-y-3 rounded-lg border border-divider bg-surface p-4 shadow-sm"
      >
        <h2 className="text-h2 text-ink">画像を選ぶ</h2>
        <div className="rounded-md border border-divider bg-surface-soft p-3">
          <p className="mb-2 text-xs text-ink-medium">送信前の Bot 検証が必要です</p>
          <TurnstileWidget
            sitekey={turnstileSiteKey}
            action="upload"
            onVerify={handleTurnstileVerify}
            onError={handleTurnstileError}
            onExpired={handleTurnstileExpired}
            onTimeout={handleTurnstileTimeout}
          />
        </div>
        <input
          type="file"
          multiple
          accept="image/jpeg,image/png,image/webp"
          onChange={onFileSelect}
          disabled={!turnstileReady || tilesAtCap}
          data-testid="prepare-file-input"
          className="block w-full text-sm"
        />
        {!turnstileReady && (
          <p className="text-xs text-ink-medium">
            まず Bot 検証を完了してください（写真選択は検証後に有効になります）。
          </p>
        )}
        {tilesAtCap && (
          <p className="text-xs text-status-error">
            最大 {MAX_TILES} 枚まで追加できます。これ以上は分けて投稿してください。
          </p>
        )}
        {globalError !== "" && (
          <p
            role="alert"
            data-testid="prepare-error"
            className="text-xs text-status-error"
          >
            {globalError}
          </p>
        )}
      </section>

      <section
        data-testid="prepare-summary"
        className="rounded-lg border border-divider bg-surface p-4 text-sm text-ink-medium"
      >
        <p data-testid="prepare-progress">
          進捗 <strong className="text-ink-strong font-num">{completedKnown}</strong>
          {" / "}
          <strong className="text-ink-strong font-num">{totalKnown}</strong>
        </p>
        <p className="mt-1 text-xs text-ink-soft">
          合計 <span className="font-num">{sum.total}</span> 枚 / 完了{" "}
          <span className="text-status-success font-num">{sum.available}</span> / 処理中{" "}
          <span className="text-brand-teal font-num">{sum.processing + sum.active}</span> / 失敗{" "}
          <span className="text-status-error font-num">{sum.failed}</span>
        </p>
        {(view.processingCount > 0 || sum.processing > 0 || sum.active > 0) && !slowNotice && (
          <p className="mt-1 text-xs text-ink-soft" data-testid="prepare-normal-notice">
            画像の処理は通常 1〜2 分ほどで完了します。画面を開いたままお待ちください。
          </p>
        )}
        {slowNotice && (
          <p
            className="mt-1 text-xs text-status-warning"
            data-testid="prepare-slow-notice"
            role="status"
          >
            画像の処理に時間がかかっています（10 分以上）。混み合っている可能性があります。
            一度ブラウザを再読み込みしてもこれまでの進捗は保持されます。
          </p>
        )}
      </section>

      {queue.tiles.length > 0 && (
        <section
          data-testid="prepare-tiles"
          aria-label="選択された画像"
          className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4"
        >
          {queue.tiles.map((tile) => (
            <ImageTile key={tile.id} tile={tile} />
          ))}
        </section>
      )}

      <section className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <p className="text-xs text-ink-soft">
          画像の配置・キャプション・公開設定は次の編集画面で行います。
        </p>
        <button
          type="button"
          onClick={onProceed}
          disabled={!proceed}
          data-testid="prepare-proceed"
          className="inline-flex h-12 items-center justify-center rounded bg-brand-teal px-6 text-sm font-bold text-white hover:bg-brand-teal-hover disabled:cursor-not-allowed disabled:opacity-60"
        >
          {proceeding
            ? "準備中…"
            : proceed
              ? "編集へ進む"
              : isAllSettled(queue) && view.processingCount === 0
                ? "対象の画像がありません"
                : "全ての画像処理が終わるまでお待ちください"}
        </button>
      </section>
      {proceedError !== "" && (
        <p
          role="alert"
          data-testid="prepare-proceed-error"
          className="text-xs text-status-error"
        >
          {proceedError}
        </p>
      )}
    </main>
  );
}
