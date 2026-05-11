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
import { PageBlock } from "@/components/Edit/PageBlock";
import { PreviewPane } from "@/components/Edit/PreviewPane";
import { PreviewToggle, type ViewMode } from "@/components/Edit/PreviewToggle";
import { PublishSettingsPanel } from "@/components/Edit/PublishSettingsPanel";
import { PublicTopBar } from "@/components/Public/PublicTopBar";
import { SectionEyebrow } from "@/components/Public/SectionEyebrow";
import {
  addPage,
  bulkReorderPhotos,
  clearCoverImage,
  fetchEditViewClient,
  isEditApiError,
  mergePages,
  movePhoto,
  removePhoto,
  reorderPages,
  setCoverImage,
  splitPage,
  updatePageCaption,
  updatePhotobookSettings,
  updatePhotoCaption,
  type EditApiError,
  type EditPage,
  type EditSettings,
  type EditView,
  type MovePosition,
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
  // STOP P-6: edit / preview mode 切替。preview 中は mutation UI を出さない (read-only)。
  const [mode, setMode] = useState<ViewMode>("edit");

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

  // 2026-05-03 STOP α P0-β hotfix: reload は browser からの client polling 経路のため
  // credentials:"include" が必要な fetchEditViewClient を使う。SSR 用の fetchEditView
  // (Cookie ヘッダ手動転送) を browser から呼ぶと cross-origin で Cookie が送られず
  // 401 になり、「最新を取得」「polling」が常に失敗していた。
  const reload = useCallback(async () => {
    try {
      const next = await fetchEditViewClient(view.photobookId);
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

  // === page caption (STOP P-5、A 方式) ===
  // updatePageCaption は {version: N+1} を返す。setView では caption と version を同時に
  // 反映する (Backend で内部 bumpVersion されているため、view.version は +1 が正しい)。
  const onPageCaptionSave = useCallback(
    (pageId: string) => async (caption: string | null) => {
      try {
        const res = await updatePageCaption(view.photobookId, pageId, caption, view.version);
        setView((v) => ({
          ...v,
          version: res.version,
          pages: v.pages.map((p) =>
            p.pageId === pageId ? { ...p, caption: caption ?? undefined } : p,
          ),
        }));
      } catch (e) {
        handleApiError(e);
        throw e;
      }
    },
    [view.photobookId, view.version, handleApiError],
  );

  // === split page (STOP P-5、B 方式) ===
  // 成功時は更新後 EditView を setView で丸ごと反映 (presigned URL も再発行されている)。
  const onSplitPage = useCallback(
    (pageId: string) => async (splitAtPhotoId: string) => {
      try {
        const next = await splitPage(view.photobookId, pageId, splitAtPhotoId, view.version);
        setView(next);
      } catch (e) {
        handleApiError(e);
      }
    },
    [view.photobookId, view.version, handleApiError],
  );

  // === move photo between pages (STOP P-5、B 方式) ===
  const onMovePhoto = useCallback(
    async (photoId: string, targetPageId: string, position: MovePosition) => {
      try {
        const next = await movePhoto(view.photobookId, photoId, targetPageId, position, view.version);
        setView(next);
      } catch (e) {
        handleApiError(e);
      }
    },
    [view.photobookId, view.version, handleApiError],
  );

  // === merge pages (STOP P-6、B 方式) ===
  // 「上と結合」: source = 当該 page、target = ひとつ上の page。返却 EditView を setView。
  // confirm UI は PageActionBar 内 (window.confirm)、ここでは API 呼出のみ。
  // 1 page only / 先頭 page で merge button が出ないため defensive で reject される
  // ことは想定外 (UI 防御で到達不能)。
  const onMergeIntoPrev = useCallback(
    (page: EditPage) => async () => {
      const idx = view.pages.findIndex((p) => p.pageId === page.pageId);
      if (idx <= 0) return; // defensive
      const target = view.pages[idx - 1];
      try {
        const next = await mergePages(view.photobookId, page.pageId, target.pageId, view.version);
        setView(next);
      } catch (e) {
        handleApiError(e);
      }
    },
    [view.photobookId, view.version, view.pages, handleApiError],
  );

  // === reorder pages (STOP P-6、B 方式) ===
  // 隣接 swap のみ。assignments は全 page を含み、display_order は 0..N-1 の permutation。
  // adjacent swap のため必ず permutation になる (UI 側で invalid_reorder_assignments を防御)。
  const swapPagesAndReorder = useCallback(
    async (page: EditPage, targetIndex: number) => {
      const fromIdx = view.pages.findIndex((p) => p.pageId === page.pageId);
      if (fromIdx === -1 || fromIdx === targetIndex) return;
      if (targetIndex < 0 || targetIndex >= view.pages.length) return;
      const next = [...view.pages];
      const [moved] = next.splice(fromIdx, 1);
      next.splice(targetIndex, 0, moved);
      const assignments = next.map((p, i) => ({ pageId: p.pageId, displayOrder: i }));
      try {
        const res = await reorderPages(view.photobookId, assignments, view.version);
        setView(res);
      } catch (e) {
        handleApiError(e);
      }
    },
    [view.photobookId, view.version, view.pages, handleApiError],
  );

  const onPageMoveUp = useCallback(
    (page: EditPage) => async () => {
      const idx = view.pages.findIndex((p) => p.pageId === page.pageId);
      if (idx > 0) await swapPagesAndReorder(page, idx - 1);
    },
    [view.pages, swapPagesAndReorder],
  );

  const onPageMoveDown = useCallback(
    (page: EditPage) => async () => {
      const idx = view.pages.findIndex((p) => p.pageId === page.pageId);
      if (idx >= 0 && idx < view.pages.length - 1)
        await swapPagesAndReorder(page, idx + 1);
    },
    [view.pages, swapPagesAndReorder],
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
  // 2026-05-03 STOP α P0 v2: rightsAgreed checkbox 値を Backend に転送。
  // 409 response の status / reason を見て UX 別文言を出し、「最新を取得」CTA は
  // version_conflict のときのみ表示する（reason ベースの publish_precondition_failed
  // ではユーザが直す対象が別なので reload しても解消しない）。
  const onPublish = useCallback(
    async (rightsAgreed: boolean) => {
      try {
        const res = await publishPhotobook(view.photobookId, view.version, rightsAgreed);
        setPublishResult(res);
        setErrorMsg(null);
        setConflict("ok");
      } catch (e) {
        if (isPublishApiError(e)) {
          if (e.kind === "version_conflict") {
            setConflict("conflict");
            setErrorMsg("他の編集が反映されました。最新を取得して再度公開してください。");
            return;
          }
          if (e.kind === "publish_precondition_failed") {
            // reason 別の具体文言（authenticated owner 向け、reload は案内しない）
            setConflict("ok");
            switch (e.reason) {
              case "rights_not_agreed":
                setErrorMsg(
                  "公開前に権利・配慮確認への同意が必要です。チェックを入れてから公開してください。",
                );
                return;
              case "not_draft":
                setErrorMsg(
                  "このフォトブックは既に公開済み、または編集できない状態です。",
                );
                return;
              case "empty_creator":
                setErrorMsg("作者名が未設定です。現在の画面では修正できません。");
                return;
              case "empty_title":
                setErrorMsg("タイトルを入力してください。");
                return;
              case "unknown_precondition":
                setErrorMsg("公開条件を満たしていません。入力内容を確認してください。");
                return;
            }
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
    },
    [view.photobookId, view.version],
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
  // STOP P-5: split disable 理由判定。
  // - 30 page 上限到達: 全 photo で split 不可
  // - page 末尾 photo (sole も含む): 新 page が空になるため split 不可
  // 計画 §5.1 / §5.4。tooltip 文言は UI で表示するため文字列で返す。
  const MAX_PAGES = 30;
  const splitDisabledReasonOf = (page: EditPage) => (
    _photoId: string,
    idx: number,
  ): string | undefined => {
    if (view.pages.length >= MAX_PAGES) {
      return "ページ数が上限 (30) に達しています";
    }
    if (idx === page.photos.length - 1) {
      return "末尾の写真ではページを分けられません";
    }
    return undefined;
  };
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
        // M-2 STOP δ (ADR-0007): CompleteView 内で `/api/public/photobooks/{id}/ogp`
        // を 2 s 間隔で polling し、OGP readiness を判定するため photobookId を渡す。
        photobookId={publishResult.photobookId}
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

  // STOP P-6: preview mode 中は edit UI を全て隠して PreviewPane を render。
  // ViewerLayout は独自に PublicTopBar / Cover / 各 PageHero / PublicPageFooter を持つので、
  // 余計な container は付けず、上部に「編集に戻る」toggle のみ overlay で出す。
  if (mode === "preview") {
    return (
      <>
        <div
          className="fixed right-3 top-3 z-50"
          data-testid="preview-toggle-floating"
        >
          <PreviewToggle mode={mode} onToggle={() => setMode("edit")} />
        </div>
        <PreviewPane view={view} />
      </>
    );
  }

  return (
    <>
      {/* m2-design-refresh STOP β-4: PublicTopBar 統合 (`design/source/project/wf-shared.jsx:29-48`)。
          draft session 経路だが LP 戻り nav は妥当、primary CTA は draft 中の文脈で違和感のため非表示 */}
      <PublicTopBar showPrimaryCta={false} />
      <main className="mx-auto min-h-screen w-full max-w-screen-md px-4 py-6 sm:px-9 sm:py-9 lg:max-w-[1280px]">
        <header className="space-y-2 border-b border-divider-soft pb-4">
          {/* design `wf-screens-b.jsx:99` PC eyebrow「Step 3 / 3」 */}
          <SectionEyebrow>Step 3 / 3</SectionEyebrow>
          <div className="flex flex-wrap items-center justify-between gap-3">
            <h1 className="text-h1 tracking-tight text-ink sm:text-h1-lg">編集ページ</h1>
            <div className="flex items-center gap-3">
              <span className="font-num text-xs text-ink-medium">version {view.version}</span>
              {/* STOP P-6: edit / preview 切替。preview に入ると floating toggle で
                  「編集に戻る」が右上に出る (PreviewPane の上に重なる)。 */}
              <PreviewToggle
                mode={mode}
                disabled={isBusy}
                onToggle={() => setMode("preview")}
              />
            </div>
          </div>
        </header>

        {conflict === "conflict" && (
          <div
            className="mt-5 flex items-start gap-2.5 rounded-lg border-l-[3px] border-status-error bg-status-error-soft p-3.5"
            data-testid="conflict-banner"
          >
            <span
              aria-hidden="true"
              className="grid h-[18px] w-[18px] shrink-0 place-items-center rounded-full bg-status-error font-serif text-xs font-bold italic leading-none text-white"
            >
              !
            </span>
            <div className="flex-1 text-xs leading-[1.6] text-status-error">
              {errorMsg ?? "他の編集が反映されました。"}
            </div>
            <button
              type="button"
              onClick={() => void reload()}
              className="inline-flex h-8 shrink-0 items-center rounded-md border border-status-error bg-surface px-2.5 text-[11px] font-semibold text-status-error transition-colors hover:bg-status-error-soft"
            >
              最新を取得
            </button>
          </div>
        )}
        {conflict === "ok" && errorMsg && (
          <div className="mt-5 rounded-md border border-divider bg-surface-soft px-4 py-3 text-xs text-ink-medium">
            {errorMsg}
          </div>
        )}

        {(view.processingCount > 0 || view.failedCount > 0) && (
          <div className="mt-5 flex items-start gap-2.5 rounded-lg border-l-[3px] border-teal-300 bg-teal-50 p-3.5">
            <span
              aria-hidden="true"
              className="grid h-[18px] w-[18px] shrink-0 place-items-center rounded-full bg-teal-500 font-serif text-xs font-bold italic leading-none text-white"
            >
              i
            </span>
            <div className="flex-1 text-xs leading-[1.6] text-ink-strong">
              {view.processingCount > 0 && (
                <span>処理中: {view.processingCount} 枚</span>
              )}
              {view.processingCount > 0 && view.failedCount > 0 && <span> / </span>}
              {view.failedCount > 0 && (
                <span className="text-status-error">失敗: {view.failedCount} 枚</span>
              )}
            </div>
          </div>
        )}

        {/* design `wf-screens-b.jsx:112` PC `wf-grid-1-2-1` (`wireframe-styles.css:569`)。
            Mobile / md は単 col に reset、lg+ で 3 col (左 cover / 中 photo / 右 publish) */}
        <div className="mt-6 grid grid-cols-1 gap-5 lg:grid-cols-[260px_1fr_320px] lg:items-start lg:gap-5">
          {/* Left col: CoverPanel (design は ページ一覧 + CoverPanel だが、production は
              page 別 active 状態を持たないため CoverPanel のみ。ページ一覧は中央 col に section 化) */}
          <div className="space-y-4">
            <CoverPanel
              cover={view.cover}
              coverTitle={view.settings.coverTitle}
              disabled={isBusy}
              onClear={onClearCover}
            />
          </div>

          {/* Center col: pages + photo grids + edit-upload-fallback + add-page */}
          <div className="space-y-5">
            {view.pages.length === 0 ? (
              <div className="rounded-lg border-2 border-dashed border-divider-soft bg-surface-soft p-6 text-center text-sm text-ink-medium">
                まだページがありません。下のボタンからページを追加してください。
                <div className="mt-4">
                  <button
                    type="button"
                    onClick={() => void onAddPage()}
                    disabled={isBusy}
                    className="inline-flex h-12 items-center justify-center rounded-[10px] bg-brand-teal px-6 text-sm font-bold text-white shadow-sm transition-colors hover:bg-brand-teal-hover disabled:cursor-not-allowed disabled:opacity-45"
                  >
                    最初のページを追加
                  </button>
                </div>
              </div>
            ) : (
              view.pages.map((page) => (
                <PageBlock
                  key={page.pageId}
                  page={page}
                  allPages={view.pages}
                  expectedVersion={view.version}
                  isBusy={isBusy}
                  isCover={(imageId) => view.coverImageId === imageId}
                  onPhotoCaptionSave={onCaptionSave}
                  onMoveUp={onMoveUp(page)}
                  onMoveDown={onMoveDown(page)}
                  onMoveTop={onMoveTop(page)}
                  onMoveBottom={onMoveBottom(page)}
                  onSetCover={onSetCover}
                  onClearCover={onClearCover}
                  onRemovePhoto={(photoId) => onRemovePhoto(page, photoId)}
                  onPageCaptionSave={onPageCaptionSave(page.pageId)}
                  splitDisabledReasonOf={splitDisabledReasonOf(page)}
                  onSplit={onSplitPage(page.pageId)}
                  onMovePhoto={onMovePhoto}
                  onMergeIntoPrev={onMergeIntoPrev(page)}
                  onPageMoveUp={onPageMoveUp(page)}
                  onPageMoveDown={onPageMoveDown(page)}
                />
              ))
            )}

            {/* /prepare で複数画像を一括投入する導線が主。/edit ではフォールバックとして 1 枚ずつ
                追加できる導線を残す（docs/plan/m2-upload-staging-plan.md §7.1）。
                design `wf-screens-b.jsx:41-47` (M) / `:156-163` (PC) の `wf-box.dashed` 視覚 */}
            <section
              data-testid="edit-upload-fallback"
              className="space-y-3 rounded-lg border-2 border-dashed border-divider-soft bg-surface-soft p-4 sm:p-5"
            >
              <h3 className="flex items-center gap-2 text-xs font-bold tracking-[0.04em] text-ink-strong">
                <span aria-hidden="true" className="block h-3.5 w-1 rounded-sm bg-teal-500" />
                写真を 1 枚ずつ追加
              </h3>
              <p className="text-xs leading-[1.6] text-ink-medium">
                まとめて投稿したい場合は新しい photobook 作成時の「写真を追加」画面をご利用ください。
                ここからは 1 枚ずつ追加できます（JPEG / PNG / WebP、HEIC / HEIF 未対応）。
                大きい画像は送信前に自動圧縮されます。
              </p>
              <input
                type="file"
                accept="image/jpeg,image/png,image/webp"
                onChange={handleFileSelect}
                disabled={
                  isBusy ||
                  uploadStatus.kind === "verifying" ||
                  uploadStatus.kind === "uploading" ||
                  uploadStatus.kind === "completing"
                }
                className="block w-full text-xs"
              />
              {pendingFile && (
                <div className="space-y-3">
                  <p className="font-num text-xs text-ink-medium">
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
                    className="inline-flex h-10 items-center justify-center rounded-[10px] bg-brand-teal px-5 text-xs font-bold text-white shadow-sm transition-colors hover:bg-brand-teal-hover disabled:cursor-not-allowed disabled:opacity-45"
                    data-testid="upload-start"
                  >
                    アップロード開始
                  </button>
                </div>
              )}
              {uploadStatus.kind === "verifying" && (
                <p className="text-xs text-ink-medium">サーバ側で Bot 検証中…</p>
              )}
              {uploadStatus.kind === "uploading" && (
                <p className="text-xs text-ink-medium">アップロード中…</p>
              )}
              {uploadStatus.kind === "completing" && (
                <p className="text-xs text-ink-medium">完了処理中…</p>
              )}
              {uploadStatus.kind === "processing" && (
                <p className="text-xs text-brand-teal">処理中（しばらく後に表示されます）</p>
              )}
              {uploadStatus.kind === "error" && (
                <p className="text-xs text-status-error">{uploadStatus.message}</p>
              )}
            </section>

            <div className="flex justify-end pt-2">
              <button
                type="button"
                onClick={() => void onAddPage()}
                disabled={isBusy}
                className="inline-flex h-10 items-center rounded-md border border-divider bg-surface px-4 text-xs font-semibold text-ink-strong transition-colors hover:border-teal-300 hover:text-teal-700 disabled:cursor-not-allowed disabled:opacity-45"
                data-testid="add-page"
              >
                + ページを追加
              </button>
            </div>
          </div>

          {/* Right col: PublishSettingsPanel (PC sidebar / Mobile では下 stack) */}
          <div className="space-y-4">
            <PublishSettingsPanel
              initial={view.settings}
              disabled={isBusy}
              publishDisabledReason={publishDisabledReason}
              onSave={onSaveSettings}
              onPublish={onPublish}
            />
          </div>
        </div>
      </main>
    </>
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
