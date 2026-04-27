// Edit page upload UI の Client Component。
//
// 設計参照:
//   - docs/plan/m2-frontend-upload-ui-plan.md §5 / §7
//
// セキュリティ:
//   - Turnstile token / upload_verification_token / presigned URL / Cookie 値を
//     UI / console / URL に出さない
//   - upload 完了後の表示は image_id と status のみ（image preview / file name は出さない）
"use client";

import { useCallback, useState } from "react";

import { TurnstileWidget } from "@/components/TurnstileWidget";
import {
  completeUpload,
  issueUploadIntent,
  issueUploadVerification,
  putToR2,
  sourceFormatOf,
  validateFile,
  type UploadError,
} from "@/lib/upload";

type UploadStatus =
  | { kind: "idle" }
  | { kind: "selected"; file: File }
  | { kind: "verifying" }
  | { kind: "uploading" }
  | { kind: "completing" }
  | { kind: "processing"; imageId: string }
  | { kind: "error"; message: string };

type CompletedImage = {
  imageId: string;
  status: string;
};

const ERROR_MESSAGES: Record<UploadError["kind"], string> = {
  verification_failed: "Bot 検証に失敗しました。もう一度お試しください。",
  turnstile_unavailable: "一時的に検証サービスに接続できません。再試行してください。",
  invalid_parameters: "ファイル形式 / サイズが不正です。",
  upload_failed: "アップロードに失敗しました。再試行してください。",
  complete_failed: "アップロード完了処理に失敗しました。",
  validation_failed: "アップロード内容が検証で失敗しました。",
  network: "通信エラーが発生しました。再試行してください。",
};

function isUploadError(e: unknown): e is UploadError {
  return typeof e === "object" && e !== null && "kind" in e;
}

export function UploadClient({
  photobookId,
  turnstileSiteKey,
}: {
  photobookId: string;
  turnstileSiteKey: string;
}) {
  const [status, setStatus] = useState<UploadStatus>({ kind: "idle" });
  const [completed, setCompleted] = useState<CompletedImage[]>([]);
  const [turnstileToken, setTurnstileToken] = useState<string | null>(null);
  const [pendingFile, setPendingFile] = useState<File | null>(null);

  const handleFileSelect = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const f = e.target.files?.[0];
    if (!f) return;
    const v = validateFile(f);
    if (v) {
      setStatus({ kind: "error", message: v.kind === "too_large" ? "10MB 以下のファイルを選択してください。" : "対応していないファイル形式です。" });
      return;
    }
    setPendingFile(f);
    setTurnstileToken(null);
    setStatus({ kind: "selected", file: f });
  }, []);

  const handleTurnstileVerify = useCallback((token: string) => {
    setTurnstileToken(token);
  }, []);

  const handleTurnstileError = useCallback(() => {
    setStatus({ kind: "error", message: ERROR_MESSAGES.verification_failed });
  }, []);

  const startUpload = useCallback(async () => {
    if (!pendingFile || !turnstileToken) return;
    const file = pendingFile;
    let contentType = file.type;
    if (!contentType) {
      const lower = file.name.toLowerCase();
      if (lower.endsWith(".heic") || lower.endsWith(".heif")) contentType = "image/heic";
    }
    const sf = sourceFormatOf(contentType);
    if (!sf) {
      setStatus({ kind: "error", message: ERROR_MESSAGES.invalid_parameters });
      return;
    }
    try {
      setStatus({ kind: "verifying" });
      const uv = await issueUploadVerification(photobookId, turnstileToken);

      setStatus({ kind: "uploading" });
      const intent = await issueUploadIntent(
        photobookId,
        uv.uploadVerificationToken,
        contentType,
        file.size,
        sf,
      );

      await putToR2(intent.uploadUrl, contentType, file);

      setStatus({ kind: "completing" });
      const done = await completeUpload(photobookId, intent.imageId, intent.storageKey);

      setCompleted((prev) => [...prev, { imageId: done.imageId, status: done.status }]);
      setStatus({ kind: "processing", imageId: done.imageId });
      setPendingFile(null);
      setTurnstileToken(null);
    } catch (e) {
      const kind = isUploadError(e) ? e.kind : "network";
      setStatus({ kind: "error", message: ERROR_MESSAGES[kind] });
    }
  }, [pendingFile, turnstileToken, photobookId]);

  return (
    <section className="mx-auto max-w-3xl p-8">
      <h1 className="mb-4 text-xl font-semibold">Draft 編集ページ</h1>
      <p className="text-sm text-gray-700">
        photobook_id: <code className="font-mono">{photobookId}</code>
      </p>

      <div className="mt-6 rounded-lg border border-dashed border-gray-300 p-6">
        <h2 className="mb-3 text-base font-semibold">写真を追加</h2>
        <p className="mb-3 text-xs text-gray-500">
          JPG / PNG / WEBP / HEIC、最大 10MB。
        </p>
        <input
          type="file"
          accept="image/jpeg,image/png,image/webp,image/heic"
          onChange={handleFileSelect}
          disabled={status.kind === "verifying" || status.kind === "uploading" || status.kind === "completing"}
          className="block w-full text-sm"
        />

        {pendingFile && (
          <div className="mt-4">
            <p className="text-sm text-gray-700">
              選択中: {pendingFile.size.toLocaleString()} byte
            </p>
            <div className="mt-3">
              <TurnstileWidget
                sitekey={turnstileSiteKey}
                action="upload"
                onVerify={handleTurnstileVerify}
                onError={handleTurnstileError}
                onExpired={() => setTurnstileToken(null)}
              />
            </div>
            <button
              type="button"
              onClick={startUpload}
              disabled={!turnstileToken || status.kind === "verifying" || status.kind === "uploading" || status.kind === "completing"}
              className="mt-3 rounded-md bg-teal-600 px-4 py-2 text-sm font-semibold text-white disabled:opacity-50"
            >
              アップロード開始
            </button>
          </div>
        )}

        <div className="mt-4 text-sm">
          {status.kind === "verifying" && <p>Bot 検証中…</p>}
          {status.kind === "uploading" && <p>アップロード中…</p>}
          {status.kind === "completing" && <p>完了処理中…</p>}
          {status.kind === "processing" && (
            <p className="text-emerald-700">処理中（しばらく後に表示されます）</p>
          )}
          {status.kind === "error" && (
            <p className="text-red-600">{status.message}</p>
          )}
        </div>
      </div>

      {completed.length > 0 && (
        <div className="mt-6 rounded-lg border border-gray-200 p-4">
          <h2 className="mb-3 text-base font-semibold">アップロード済（処理中）</h2>
          <ul className="space-y-2 text-sm">
            {completed.map((c) => (
              <li key={c.imageId} className="font-mono">
                {c.imageId} — {c.status}
              </li>
            ))}
          </ul>
        </div>
      )}

      <p className="mt-6 text-xs text-gray-400">
        画像処理（HEIC 変換 / EXIF 除去 / variant 生成）は次の段階で実装されます。
      </p>
    </section>
  );
}
