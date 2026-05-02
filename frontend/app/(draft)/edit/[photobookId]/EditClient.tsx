// Edit ページの Client Component（orchestrator）。
//
// 設計参照:
//   - docs/plan/m2-frontend-edit-ui-fullspec-plan.md §6
//
// 役割:
//   - Server Component から渡された initial edit-view を起点に、
//     caption 編集 / reorder / cover 設定 / settings 保存 / 写真削除 / page 追加 の操作を扱う
//   - 409 conflict 時は「最新を取得」誘導を表示
//   - 5 秒間隔の simple polling で processing 件数を更新
"use client";

import { useCallback, useEffect, useState } from "react";

import { TurnstileWidget } from "@/components/TurnstileWidget";
import { CompleteView } from "@/components/Complete/CompleteView";
import { CoverPanel } from "@/components/Edit/CoverPanel";
import { PhotoGrid } from "@/components/Edit/PhotoGrid";
import { PublishSettingsPanel } from "@/components/Edit/PublishSettingsPanel";
import {
  addPage,
  bulkReorderPhotos,
  clearCoverImage,
  fetchEditView,
  isEditApiError,
  removePhoto,
  setCoverImage,
  updatePhotobookSettings,
  updatePhotoCaption,
  type EditApiError,
  type EditPage,
  type EditSettings,
  type EditView,
} from "@/lib/editPhotobook";
import {
  publishPhotobook,
  isPublishApiError,
  type PublishResult,
} from "@/lib/publishPhotobook";
import { compressImageForUpload } from "@/lib/imageCompression";
import {
  completeUpload,
  issueUploadIntent,
  issueUploadVerification,
  putToR2,
  sourceFormatOf,
  validateFile,
} from "@/lib/upload";
import { formatRetryAfterDisplay } from "@/lib/retryAfter";

const POLL_INTERVAL_MS = 5_000;

type ConflictState = "ok" | "conflict";

type UploadStatus =
  | { kind: "idle" }
  | { kind: "selected"; file: File }
  | { kind: "verifying" }
  | { kind: "uploading" }
  | { kind: "completing" }
  | { kind: "processing" }
  | { kind: "error"; message: string };

const UPLOAD_ERROR_MESSAGES: Record<string, string> = {
  verification_failed: "Bot 検証に失敗しました。もう一度お試しください。",
  turnstile_unavailable: "一時的に検証サービスに接続できません。再試行してください。",
  invalid_parameters: "ファイル形式 / サイズが不正です。",
  upload_failed: "アップロードに失敗しました。再試行してください。",
  complete_failed: "アップロード完了処理に失敗しました。",
  validation_failed: "アップロード内容が検証で失敗しました。",
  network: "通信エラーが発生しました。再試行してください。",
};

type Props = {
  initial: EditView;
  turnstileSiteKey: string;
};

export function EditClient({ initial, turnstileSiteKey }: Props) {
  const [view, setView] = useState<EditView>(initial);
  const [conflict, setConflict] = useState<ConflictState>("ok");
  const [errorMsg, setErrorMsg] = useState<string | null>(null);
  const [publishResult, setPublishResult] = useState<PublishResult | null>(null);

  // Upload widget の state
  const [pendingFile, setPendingFile] = useState<File | null>(null);
  const [turnstileToken, setTurnstileToken] = useState<string | null>(null);

  // Turnstile callback は useCallback で安定参照を維持する。TurnstileWidget 内部は
  // useRef で最新版を呼ぶため再 mount は起きないが、防御的に親側でも安定化する
  // （`.agents/rules/turnstile-defensive-guard.md` L0 / PR36-0 横展開）。
  const handleTurnstileVerify = useCallback((t: string) => {
    setTurnstileToken(t);
  }, []);
  const handleTurnstileError = useCallback(() => {
    setTurnstileToken(null);
  }, []);
  const handleTurnstileExpired = useCallback(() => {
    setTurnstileToken(null);
  }, []);
  const handleTurnstileTimeout = useCallback(() => {
    setTurnstileToken(null);
  }, []);
  const [uploadStatus, setUploadStatus] = useState<UploadStatus>({ kind: "idle" });

  const reload = useCallback(async () => {
    try {
      const next = await fetchEditView(view.photobookId, "");
      setView(next);
      setConflict("ok");
      setErrorMsg(null);
    } catch (e) {
      if (isEditApiError(e)) {
        setErrorMsg(`再取得に失敗しました（${e.kind}）。ページを再読み込みしてください。`);
      } else {
        setErrorMsg("再取得に失敗しました。ページを再読み込みしてください。");
      }
    }
  }, [view.photobookId]);

  // simple polling for processing count（PR26 §6.4）
  useEffect(() => {
    if (view.processingCount === 0) return;
    const t = setInterval(() => {
      void reload();
    }, POLL_INTERVAL_MS);
    return () => clearInterval(t);
  }, [view.processingCount, reload]);

  const handleApiError = useCallback((e: unknown) => {
    if (isEditApiError(e)) {
      const err = e as EditApiError;
      if (err.kind === "version_conflict") {
        setConflict("conflict");
        setErrorMsg("他の編集が反映されました。最新を取得してください。");
        return;
      }
      if (err.kind === "unauthorized") {
        setErrorMsg("認可セッションが切れました。draft URL から入り直してください。");
        return;
      }
      setErrorMsg(`操作に失敗しました（${err.kind}）。`);
      return;
    }
    setErrorMsg("操作に失敗しました。");
  }, []);

  // === caption ===
  const onCaptionSave = useCallback(
    async (photoId: string, caption: string | null) => {
      try {
        await updatePhotoCaption(view.photobookId, photoId, caption, view.version);
        setView((v) => bumpVersion(applyPhotoCaption(v, photoId, caption)));
      } catch (e) {
        handleApiError(e);
        throw e;
      }
    },
    [view.photobookId, view.version, handleApiError],
  );

  // === reorder ===
  const reorderTo = useCallback(
    async (page: EditPage, photoId: string, newIndex: number) => {
      const photos = page.photos;
      const fromIdx = photos.findIndex((p) => p.photoId === photoId);
      if (fromIdx === -1 || newIndex === fromIdx) return;
      const next = [...photos];
      const [moved] = next.splice(fromIdx, 1);
      next.splice(newIndex, 0, moved);
      const assignments = next.map((p, i) => ({ photoId: p.photoId, displayOrder: i }));
      try {
        await bulkReorderPhotos(view.photobookId, page.pageId, assignments, view.version);
        setView((v) => bumpVersion(applyReorder(v, page.pageId, next.map((p, i) => ({ ...p, displayOrder: i })))));
      } catch (e) {
        handleApiError(e);
      }
    },
    [view.photobookId, view.version, handleApiError],
  );

  const onMoveUp = (page: EditPage) => async (photoId: string) => {
    const i = page.photos.findIndex((p) => p.photoId === photoId);
    if (i > 0) await reorderTo(page, photoId, i - 1);
  };
  const onMoveDown = (page: EditPage) => async (photoId: string) => {
    const i = page.photos.findIndex((p) => p.photoId === photoId);
    if (i >= 0 && i < page.photos.length - 1) await reorderTo(page, photoId, i + 1);
  };
  const onMoveTop = (page: EditPage) => async (photoId: string) => {
    await reorderTo(page, photoId, 0);
  };
  const onMoveBottom = (page: EditPage) => async (photoId: string) => {
    await reorderTo(page, photoId, page.photos.length - 1);
  };

  // === cover ===
  const onSetCover = useCallback(
    async (imageId: string) => {
      try {
        await setCoverImage(view.photobookId, imageId, view.version);
        await reload(); // cover variant URL は再取得が必要
      } catch (e) {
        handleApiError(e);
      }
    },
    [view.photobookId, view.version, reload, handleApiError],
  );

  const onClearCover = useCallback(async () => {
    try {
      await clearCoverImage(view.photobookId, view.version);
      await reload();
    } catch (e) {
      handleApiError(e);
    }
  }, [view.photobookId, view.version, reload, handleApiError]);

  // === photo remove ===
  const onRemovePhoto = useCallback(
    async (page: EditPage, photoId: string) => {
      try {
        await removePhoto(view.photobookId, page.pageId, photoId, view.version);
        await reload();
      } catch (e) {
        handleApiError(e);
      }
    },
    [view.photobookId, view.version, reload, handleApiError],
  );

  // === settings ===
  const onSaveSettings = useCallback(
    async (next: EditSettings) => {
      try {
        await updatePhotobookSettings(view.photobookId, next, view.version);
        setView((v) => bumpVersion({ ...v, settings: next }));
      } catch (e) {
        handleApiError(e);
        throw e;
      }
    },
    [view.photobookId, view.version, handleApiError],
  );

  // === publish ===
  const onPublish = useCallback(async () => {
    try {
      const res = await publishPhotobook(view.photobookId, view.version);
      setPublishResult(res);
      setErrorMsg(null);
      setConflict("ok");
    } catch (e) {
      if (isPublishApiError(e)) {
        if (e.kind === "version_conflict") {
          setConflict("conflict");
          setErrorMsg("公開条件に合致しません。最新を取得して再度確認してください。");
          return;
        }
        if (e.kind === "unauthorized") {
          setErrorMsg("認可セッションが切れました。draft URL から入り直してください。");
          return;
        }
        if (e.kind === "rate_limited") {
          // PR36: 1 時間 5 冊上限到達時の文言（業務知識 v4 §3.7）
          setErrorMsg(
            `公開操作の上限に達しました。1 時間あたりの公開数には上限があります。${formatRetryAfterDisplay(e.retryAfterSeconds)}時間をおいて再度お試しください。`,
          );
          return;
        }
        setErrorMsg(`公開に失敗しました（${e.kind}）。`);
        return;
      }
      setErrorMsg("公開に失敗しました。");
    }
  }, [view.photobookId, view.version]);

  // === page 追加 ===
  const onAddPage = useCallback(async () => {
    try {
      await addPage(view.photobookId, view.version);
      await reload();
    } catch (e) {
      handleApiError(e);
    }
  }, [view.photobookId, view.version, reload, handleApiError]);

  // === upload widget（既存 PR22 流用、ページに合わせた callback） ===
  // VRChat PNG（13-18MB）は client-side 圧縮を通して 10MB に収める。
  // HEIC / 非画像は圧縮前に拒否。圧縮後の File に対して validateFile を保険として再実行。
  const handleFileSelect = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const f = e.target.files?.[0];
    e.target.value = "";
    if (!f) return;
    if (
      f.type !== "image/jpeg" &&
      f.type !== "image/png" &&
      f.type !== "image/webp"
    ) {
      setUploadStatus({
        kind: "error",
        message:
          f.type.startsWith("image/heic") || f.name.toLowerCase().match(/\.(heic|heif|hif)$/)
            ? "HEIC / HEIF は現在未対応です。JPEG / PNG / WebP に変換してください。"
            : "対応していないファイル形式です（JPEG / PNG / WebP のみ）。",
      });
      return;
    }
    let prepared: File;
    try {
      const result = await compressImageForUpload(f);
      prepared = result.file;
    } catch {
      setUploadStatus({
        kind: "error",
        message: "サイズ過大で取り込めませんでした（圧縮しても 10MB に収まらず、または 50MB 超）。",
      });
      return;
    }
    const v = validateFile(prepared);
    if (v) {
      const map: Record<string, string> = {
        too_large: "サイズ過大で取り込めませんでした（圧縮しても 10MB 以下に収まりません）。",
        heic_unsupported: "HEIC / HEIF は現在未対応です。JPEG / PNG / WebP に変換してください。",
        invalid_type: "対応していないファイル形式です（JPEG / PNG / WebP のみ）。",
      };
      setUploadStatus({ kind: "error", message: map[v.kind] ?? "選択できないファイルです。" });
      return;
    }
    setPendingFile(prepared);
    setTurnstileToken(null);
    setUploadStatus({ kind: "selected", file: prepared });
  };

  // L1+L2: 多層防御 Turnstile ガード（`.agents/rules/turnstile-defensive-guard.md`）。
  // 空白のみのトークンは未完了扱い（widget 中断 / 古い state 残存対策）。
  const isTurnstileReady =
    typeof turnstileToken === "string" && turnstileToken.trim() !== "";

  const startUpload = async () => {
    // L2: button disable があっても JS 強制発火 / race condition の保険。
    if (!pendingFile || !isTurnstileReady) return;
    const file = pendingFile;
    const sf = sourceFormatOf(file.type);
    if (!sf) {
      setUploadStatus({ kind: "error", message: UPLOAD_ERROR_MESSAGES.invalid_parameters });
      return;
    }
    try {
      setUploadStatus({ kind: "verifying" });
      // 上の isTurnstileReady ガードで non-empty を保証済み（!== null）
      const uv = await issueUploadVerification(view.photobookId, turnstileToken!);
      setUploadStatus({ kind: "uploading" });
      const intent = await issueUploadIntent(
        view.photobookId,
        uv.uploadVerificationToken,
        file.type,
        file.size,
        sf,
      );
      await putToR2(intent.uploadUrl, file.type, file);
      setUploadStatus({ kind: "completing" });
      await completeUpload(view.photobookId, intent.imageId, intent.storageKey);
      setUploadStatus({ kind: "processing" });
      setPendingFile(null);
      setTurnstileToken(null);
      // upload 完了後は edit-view を再取得して photo grid を最新化
      await reload();
    } catch (e) {
      const kind = (e as { kind?: string })?.kind ?? "network";
      // PR36: rate_limited は retryAfterSeconds を含む特殊形式
      if (kind === "rate_limited") {
        const retryAfter = (e as { retryAfterSeconds?: number })?.retryAfterSeconds ?? 60;
        const msg = `短時間にアップロード操作を繰り返しています。${formatRetryAfterDisplay(retryAfter)}時間をおいて再度お試しください。`;
        setUploadStatus({ kind: "error", message: msg });
        return;
      }
      setUploadStatus({ kind: "error", message: UPLOAD_ERROR_MESSAGES[kind] ?? UPLOAD_ERROR_MESSAGES.network });
    }
  };

  const isBusy = conflict === "conflict";
  const appBaseUrl = process.env.NEXT_PUBLIC_BASE_URL ?? "";
  const totalAvailablePhotos = view.pages.reduce((n, p) => n + p.photos.length, 0);
  const publishDisabledReason =
    totalAvailablePhotos === 0
      ? "公開には最低 1 枚の写真が必要です。"
      : view.processingCount > 0
        ? "処理中の写真があります。完了してから公開してください。"
        : undefined;

  if (publishResult) {
    return (
      <CompleteView
        appBaseUrl={appBaseUrl}
        publicUrlPath={publishResult.publicUrlPath}
        manageUrlPath={publishResult.manageUrlPath}
        onBackToEdit={() => {
          // 公開済になったため edit-view fetch は 409 になる。
          // ユーザーが「編集に戻る」を押した場合は manage URL で入り直してもらう想定。
          // 本リリースでは draft session が revoke されているので「編集ページに戻る」=
          // 一覧トップへの遷移にとどめる。
          window.location.href = "/";
        }}
      />
    );
  }

  return (
    <main className="mx-auto max-w-screen-md space-y-6 p-4 sm:p-6">
      <header className="flex items-center justify-between border-b border-divider pb-3">
        <h1 className="text-h1 text-ink">編集ページ</h1>
        <span className="text-xs text-ink-medium font-num">v{view.version}</span>
      </header>

      {conflict === "conflict" && (
        <div
          className="flex items-center justify-between rounded-md border border-status-error bg-status-error-soft px-4 py-3 text-sm text-status-error"
          data-testid="conflict-banner"
        >
          <span>{errorMsg ?? "他の編集が反映されました。"}</span>
          <button
            type="button"
            onClick={() => void reload()}
            className="rounded-sm border border-status-error px-3 py-1 text-xs text-status-error hover:bg-status-error-soft"
          >
            最新を取得
          </button>
        </div>
      )}
      {conflict === "ok" && errorMsg && (
        <div className="rounded-md border border-divider bg-surface-soft px-4 py-3 text-sm text-ink-medium">
          {errorMsg}
        </div>
      )}

      {(view.processingCount > 0 || view.failedCount > 0) && (
        <div className="rounded-md border border-divider bg-surface-soft px-4 py-3 text-sm text-ink-medium">
          {view.processingCount > 0 && (
            <span>処理中: {view.processingCount} 枚</span>
          )}
          {view.processingCount > 0 && view.failedCount > 0 && <span> / </span>}
          {view.failedCount > 0 && (
            <span className="text-status-error">失敗: {view.failedCount} 枚</span>
          )}
        </div>
      )}

      {view.pages.length === 0 ? (
        <div className="rounded-lg border border-dashed border-divider bg-surface-soft p-6 text-center text-sm text-ink-medium">
          まだページがありません。下のボタンからページを追加してください。
          <div className="mt-4">
            <button
              type="button"
              onClick={() => void onAddPage()}
              disabled={isBusy}
              className="rounded-md bg-brand-teal px-4 py-2 text-sm font-medium text-white hover:bg-brand-teal-hover disabled:opacity-50"
            >
              最初のページを追加
            </button>
          </div>
        </div>
      ) : (
        view.pages.map((page) => (
          <section key={page.pageId} className="space-y-3">
            <h2 className="text-h2 text-ink">ページ {page.displayOrder + 1}</h2>
            <PhotoGrid
              page={page}
              expectedVersion={view.version}
              isCover={(imageId) => view.coverImageId === imageId}
              isBusy={isBusy}
              onCaptionSave={onCaptionSave}
              onMoveUp={onMoveUp(page)}
              onMoveDown={onMoveDown(page)}
              onMoveTop={onMoveTop(page)}
              onMoveBottom={onMoveBottom(page)}
              onSetCover={onSetCover}
              onClearCover={onClearCover}
              onRemovePhoto={(photoId) => onRemovePhoto(page, photoId)}
            />
          </section>
        ))
      )}

      {/* /prepare で複数画像を一括投入する導線が主。/edit ではフォールバックとして 1 枚ずつ
          追加できる導線を残す（docs/plan/m2-upload-staging-plan.md §7.1）。 */}
      <section
        data-testid="edit-upload-fallback"
        className="space-y-2 rounded-md border border-divider bg-surface-soft p-3 text-sm"
      >
        <h3 className="text-sm font-bold text-ink">写真を 1 枚ずつ追加</h3>
        <p className="text-xs text-ink-medium">
          まとめて投稿したい場合は新しい photobook 作成時の「写真を追加」画面をご利用ください。
          ここからは 1 枚ずつ追加できます（JPEG / PNG / WebP、最大 10MB、HEIC / HEIF 未対応）。
        </p>
        <input
          type="file"
          accept="image/jpeg,image/png,image/webp"
          onChange={handleFileSelect}
          disabled={isBusy || uploadStatus.kind === "verifying" || uploadStatus.kind === "uploading" || uploadStatus.kind === "completing"}
          className="block w-full text-xs"
        />
        {pendingFile && (
          <div className="space-y-3">
            <p className="text-sm text-ink-medium">
              選択中: {pendingFile.size.toLocaleString()} byte
            </p>
            <TurnstileWidget
              sitekey={turnstileSiteKey}
              action="upload"
              onVerify={handleTurnstileVerify}
              onError={handleTurnstileError}
              onExpired={handleTurnstileExpired}
              onTimeout={handleTurnstileTimeout}
            />
            <button
              type="button"
              disabled={!isTurnstileReady || isBusy}
              onClick={() => void startUpload()}
              className="rounded-md bg-brand-teal px-4 py-2 text-sm font-medium text-white hover:bg-brand-teal-hover disabled:opacity-50"
              data-testid="upload-start"
            >
              アップロード開始
            </button>
          </div>
        )}
        {uploadStatus.kind === "verifying" && <p className="text-sm">サーバ側で Bot 検証中…</p>}
        {uploadStatus.kind === "uploading" && <p className="text-sm">アップロード中…</p>}
        {uploadStatus.kind === "completing" && <p className="text-sm">完了処理中…</p>}
        {uploadStatus.kind === "processing" && <p className="text-sm text-brand-teal">処理中（しばらく後に表示されます）</p>}
        {uploadStatus.kind === "error" && <p className="text-sm text-status-error">{uploadStatus.message}</p>}
      </section>

      <CoverPanel
        cover={view.cover}
        coverTitle={view.settings.coverTitle}
        disabled={isBusy}
        onClear={onClearCover}
      />

      <PublishSettingsPanel
        initial={view.settings}
        disabled={isBusy}
        publishDisabledReason={publishDisabledReason}
        onSave={onSaveSettings}
        onPublish={onPublish}
      />

      <div className="flex justify-end pt-2">
        <button
          type="button"
          onClick={() => void onAddPage()}
          disabled={isBusy}
          className="rounded-md border border-divider px-4 py-2 text-sm text-ink-medium hover:bg-surface-soft disabled:opacity-50"
          data-testid="add-page"
        >
          ページを追加
        </button>
      </div>
    </main>
  );
}

// === 状態更新ヘルパ ===

function bumpVersion(v: EditView): EditView {
  return { ...v, version: v.version + 1 };
}

function applyPhotoCaption(v: EditView, photoId: string, caption: string | null): EditView {
  return {
    ...v,
    pages: v.pages.map((p) => ({
      ...p,
      photos: p.photos.map((ph) => (ph.photoId === photoId ? { ...ph, caption: caption ?? undefined } : ph)),
    })),
  };
}

function applyReorder(v: EditView, pageId: string, nextPhotos: EditView["pages"][number]["photos"]): EditView {
  return {
    ...v,
    pages: v.pages.map((p) => (p.pageId === pageId ? { ...p, photos: nextPhotos } : p)),
  };
}
