// /prepare/<photobookId> Client Component（Upload Staging 画面）。
//
// 設計参照: docs/plan/m2-upload-staging-plan.md §6
//
// 役割:
//   - 複数画像の一括選択 + concurrency=2 並列 upload + tile 状態管理
//   - 5 sec polling + exponential backoff + max 10 min duration + Page Visibility API
//   - 全 available になったら「編集へ進む」ボタンで /edit/<photobookId> 遷移
//
// セキュリティ:
//   - raw imageId / storage_key / upload URL を console / DOM に出さない
//   - Turnstile token / verification token / Cookie 値は state にも保持しない（catch 後即破棄）
//   - failed 時の reason は user-friendly mapping のみ

"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { ImageTile } from "@/components/Prepare/ImageTile";
import {
  addFiles,
  canProceedToEdit,
  emptyQueue,
  isAllSettled,
  markStatus,
  pollDelaySeconds,
  reconcileWithServer,
  selectNextRunnable,
  summary,
  type QueueState,
  type QueueTile,
  type TileFailureReason,
} from "@/components/Prepare/UploadQueue";
import { TurnstileWidget } from "@/components/TurnstileWidget";
import { fetchEditView, type EditView } from "@/lib/editPhotobook";
import {
  CompressionError,
  compressImageForUpload,
} from "@/lib/imageCompression";
import {
  completeUpload,
  issueUploadIntent,
  issueUploadVerification,
  putToR2,
  sourceFormatOf,
  validateFile,
} from "@/lib/upload";

const CONCURRENCY = 2;
const MAX_TILES = 20; // upload-verification session の allowed_intent_count 上限と整合
const MAX_POLL_DURATION_MS = 10 * 60 * 1000; // 10 分

type Props = {
  photobookId: string;
  turnstileSiteKey: string;
  initialView: EditView;
};

type ViewState = {
  processingCount: number;
  failedCount: number;
  placedImageIds: Set<string>;
  placedPhotoCount: number;
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
  return {
    processingCount: v.processingCount,
    failedCount: v.failedCount,
    placedImageIds: placed,
    placedPhotoCount: count,
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
  const [queue, setQueue] = useState<QueueState>(() => emptyQueue());
  const [view, setView] = useState<ViewState>(() => viewToState(initialView));
  const [turnstileToken, setTurnstileToken] = useState<string>("");
  const [verificationToken, setVerificationToken] = useState<string>("");
  const [globalError, setGlobalError] = useState<string>("");

  // queue の最新値を非同期処理から参照するため、ref で保持。
  const queueRef = useRef<QueueState>(queue);
  useEffect(() => {
    queueRef.current = queue;
  }, [queue]);

  // turnstile callback は useCallback で安定化（widget の remount loop 回避）。
  const handleTurnstileVerify = useCallback((tok: string) => {
    setTurnstileToken(tok);
  }, []);
  const handleTurnstileError = useCallback(() => {
    setTurnstileToken("");
  }, []);
  const handleTurnstileExpired = useCallback(() => {
    setTurnstileToken("");
    setVerificationToken("");
  }, []);
  const handleTurnstileTimeout = useCallback(() => {
    setTurnstileToken("");
    setVerificationToken("");
  }, []);

  // ファイル選択 → 軽量検証（HEIC など）→ クライアント圧縮（VRChat PNG 13-18MB を JPEG 化）
  // → validateFile（10MB 以下 / 形式チェック）→ queue 追加
  const onFileSelect = useCallback(
    async (e: React.ChangeEvent<HTMLInputElement>) => {
      const incoming = e.target.files ? Array.from(e.target.files) : [];
      e.target.value = ""; // 同じファイルを再度選択できるよう
      if (incoming.length === 0) return;

      const accepted: File[] = [];
      let rejectedFormat = 0;
      let rejectedTooHuge = 0;
      let recompressed = 0;
      for (const f of incoming) {
        // 形式チェック（HEIC / 非画像）は圧縮前に実施。type が未知の場合も拒否。
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
          // 念のため最終 validate（target>=10MB を満たす想定）
          const v = validateFile(result.file);
          if (v) {
            rejectedTooHuge++;
            continue;
          }
          accepted.push(result.file);
        } catch (err) {
          if (err instanceof CompressionError) {
            // input_too_large / still_too_large / decode_failed / encode_failed をひと括りに「過大」扱い
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
        setGlobalError(""); // 再エンコードは正常動作のため error 表示しない（必要なら toast 化を P1 で）
      } else {
        setGlobalError("");
      }
    },
    [],
  );

  // 1 tile の upload chain（verifying → uploading → completing → processing）を駆動
  const runUpload = useCallback(
    async (tile: QueueTile) => {
      // L2: Turnstile token 必須（widget 完了前に submit を素通りさせない）
      const tok = turnstileToken;
      if (typeof tok !== "string" || tok.trim() === "") {
        setQueue((q) => markStatus(q, tile.id, { kind: "failed", reason: "verification_failed" }));
        return;
      }

      // verifying（必要なら upload-verification を 1 度だけ取得し session として再利用）
      try {
        setQueue((q) => markStatus(q, tile.id, { kind: "verifying" }));
        let vtok = verificationToken;
        if (vtok === "") {
          const uv = await issueUploadVerification(photobookId, tok);
          vtok = uv.uploadVerificationToken;
          setVerificationToken(vtok);
        }

        // uploading
        const sf = sourceFormatOf(tile.file.type);
        if (sf === null) {
          setQueue((q) =>
            markStatus(q, tile.id, { kind: "failed", reason: "validation_failed" }),
          );
          return;
        }
        const intent = await issueUploadIntent(
          photobookId,
          vtok,
          tile.file.type,
          tile.file.size,
          sf,
        );
        setQueue((q) => markStatus(q, tile.id, { kind: "uploading" }));
        await putToR2(intent.uploadUrl, tile.file.type, tile.file);

        // completing
        setQueue((q) => markStatus(q, tile.id, { kind: "completing" }));
        await completeUpload(photobookId, intent.imageId, intent.storageKey);

        // processing（image-processor が available にするまで polling 待ち）
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
        // verification 系の失敗は session も破棄（次の tile で再取得させる）
        if (kind === "verification_failed" || kind === "rate_limited") {
          setVerificationToken("");
          setTurnstileToken("");
        }
      }
    },
    [photobookId, turnstileToken, verificationToken],
  );

  // queue の状態が変わるたびに、concurrency 上限まで queued tile を起動
  useEffect(() => {
    const next = selectNextRunnable(queue, CONCURRENCY);
    if (next === null) return;
    if (typeof turnstileToken !== "string" || turnstileToken.trim() === "") return;
    void runUpload(next);
    // 直後の state 更新で再 run も検知できるよう、useEffect を queue に依存させる
  }, [queue, turnstileToken, runUpload]);

  // ===== polling: edit-view を再取得し、processing tile を reconcile =====
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
      const v = await fetchEditView(photobookId, "");
      const next = viewToState(v);
      setView(next);
      setQueue((q) => reconcileWithServer(q, next.placedImageIds, next.processingCount));
    } catch {
      // 失敗は無視（次の tick で再試行、敵対者対策で詳細はログに出さない）
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

  // queue に processing / active がある間、または server processing が残っている間 polling
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

  // Page Visibility API: background 中は polling を一時停止、復帰時に再開
  useEffect(() => {
    const onVisChange = () => {
      visibleRef.current = !document.hidden;
      if (!document.hidden && needPolling) {
        // 復帰時は backoff を初期化して即時 1 回 fetch
        tickRef.current = 0;
        schedulePoll();
      } else if (document.hidden) {
        stopPolling();
      }
    };
    document.addEventListener("visibilitychange", onVisChange);
    return () => document.removeEventListener("visibilitychange", onVisChange);
  }, [needPolling, schedulePoll, stopPolling]);

  // ===== UI rendering =====
  const sum = useMemo(() => summary(queue), [queue]);
  const proceed = canProceedToEdit(queue, view.processingCount, view.placedPhotoCount);
  const tilesAtCap = queue.tiles.length >= MAX_TILES;
  const turnstileReady = turnstileToken !== "" && turnstileToken.trim() !== "";

  const onProceed = () => {
    window.location.assign(`/edit/${photobookId}`);
  };

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
        <p>
          合計 <strong className="text-ink-strong font-num">{sum.total}</strong> 枚 / 完了{" "}
          <span className="text-status-success font-num">{sum.available}</span> / 処理中{" "}
          <span className="text-brand-teal font-num">{sum.processing + sum.active}</span> / 失敗{" "}
          <span className="text-status-error font-num">{sum.failed}</span>
        </p>
        {view.processingCount > 0 && (
          <p className="mt-1 text-xs text-ink-soft">
            画像処理は最大 5 分ほどかかることがあります。画面を開いたままお待ちください。
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
          {proceed
            ? "編集へ進む"
            : isAllSettled(queue) && view.processingCount === 0
              ? "対象の画像がありません"
              : "全ての画像処理が終わるまでお待ちください"}
        </button>
      </section>
    </main>
  );
}
