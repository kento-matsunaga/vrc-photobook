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
  completeUpload,
  issueUploadIntent,
  issueUploadVerification,
  putToR2,
  sourceFormatOf,
  validateFile,
} from "@/lib/upload";

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

  // Upload widget の state
  const [pendingFile, setPendingFile] = useState<File | null>(null);
  const [turnstileToken, setTurnstileToken] = useState<string | null>(null);
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
  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const f = e.target.files?.[0];
    if (!f) return;
    const v = validateFile(f);
    if (v) {
      const map: Record<string, string> = {
        too_large: "10MB 以下のファイルを選択してください。",
        heic_unsupported: "HEIC / HEIF は現在未対応です。JPEG / PNG / WebP に変換してください。",
        invalid_type: "対応していないファイル形式です（JPEG / PNG / WebP のみ）。",
      };
      setUploadStatus({ kind: "error", message: map[v.kind] ?? "選択できないファイルです。" });
      e.target.value = "";
      return;
    }
    setPendingFile(f);
    setTurnstileToken(null);
    setUploadStatus({ kind: "selected", file: f });
  };

  const startUpload = async () => {
    if (!pendingFile || !turnstileToken) return;
    const file = pendingFile;
    const sf = sourceFormatOf(file.type);
    if (!sf) {
      setUploadStatus({ kind: "error", message: UPLOAD_ERROR_MESSAGES.invalid_parameters });
      return;
    }
    try {
      setUploadStatus({ kind: "verifying" });
      const uv = await issueUploadVerification(view.photobookId, turnstileToken);
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
      setUploadStatus({ kind: "error", message: UPLOAD_ERROR_MESSAGES[kind] ?? UPLOAD_ERROR_MESSAGES.network });
    }
  };

  const isBusy = conflict === "conflict";

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

      <section className="space-y-3 rounded-lg border border-divider bg-surface p-4 shadow-sm">
        <h2 className="text-h2 text-ink">写真を追加</h2>
        <p className="text-sm text-ink-medium">
          JPEG / PNG / WebP、最大 10MB。HEIC / HEIF は未対応です。
        </p>
        <input
          type="file"
          accept="image/jpeg,image/png,image/webp"
          onChange={handleFileSelect}
          disabled={isBusy || uploadStatus.kind === "verifying" || uploadStatus.kind === "uploading" || uploadStatus.kind === "completing"}
          className="block w-full text-sm"
        />
        {pendingFile && (
          <div className="space-y-3">
            <p className="text-sm text-ink-medium">
              選択中: {pendingFile.size.toLocaleString()} byte
            </p>
            <TurnstileWidget
              sitekey={turnstileSiteKey}
              action="upload"
              onVerify={(t) => setTurnstileToken(t)}
              onError={() => setTurnstileToken(null)}
              onExpired={() => setTurnstileToken(null)}
            />
            <button
              type="button"
              disabled={!turnstileToken || isBusy}
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
        onSave={onSaveSettings}
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
