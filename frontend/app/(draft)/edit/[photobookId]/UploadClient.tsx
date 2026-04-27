// Edit page upload UI の Client Component。
//
// 設計参照:
//   - docs/plan/m2-frontend-upload-ui-plan.md §5 / §7
//
// セキュリティ:
//   - Turnstile token / upload_verification_token / presigned URL / Cookie 値を
//     UI / console / URL に出さない
//   - upload 完了後の表示は image_id と status のみ（image preview / file name は出さない)
//
// 多層 Bot ガード（PR22 Safari 確認時の指摘を受けて強化、2026-04-27）:
//   - L1: アップロード開始ボタンは turnstileToken が空文字以外でなければ disabled
//   - L2: startUpload 関数の冒頭で空 token チェック（再防御）
//   - L3: lib/upload.ts の issueUploadVerification も空 token を弾く
//   - L4: Backend handler が空 turnstile_token を 400 で拒否
//   - UI: Turnstile 検証完了状態を明示的にバッジ表示（Bot 検証成功 ✓）
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

/** Turnstile token が「実際に検証完了した」と扱える状態かを判定する。 */
function isTurnstileVerified(token: string | null): token is string {
  return typeof token === "string" && token.trim().length > 0;
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
      let message: string;
      switch (v.kind) {
        case "too_large":
          message = "10MB 以下のファイルを選択してください。";
          break;
        case "heic_unsupported":
          message = "HEIC / HEIF は現在未対応です。JPEG / PNG / WebP でアップロードしてください。";
          break;
        default:
          message = "対応していないファイル形式です（JPEG / PNG / WebP のみ）。";
      }
      setStatus({ kind: "error", message });
      // 入力をクリアして同じファイルを再選択しても change が発火するように
      e.target.value = "";
      return;
    }
    setPendingFile(f);
    setTurnstileToken(null);
    setStatus({ kind: "selected", file: f });
  }, []);

  const handleTurnstileVerify = useCallback((token: string) => {
    // 念のため空文字を弾く（widget の不正実装に対する defensive）
    if (typeof token !== "string" || token.trim() === "") return;
    setTurnstileToken(token);
  }, []);

  const handleTurnstileError = useCallback(() => {
    setTurnstileToken(null);
    setStatus({ kind: "error", message: ERROR_MESSAGES.verification_failed });
  }, []);

  const handleTurnstileExpired = useCallback(() => {
    setTurnstileToken(null);
  }, []);

  const startUpload = useCallback(async () => {
    // L2: defensive ガード（disabled UI を突破された / 状態 race のとき確実に弾く）
    if (!pendingFile) return;
    if (!isTurnstileVerified(turnstileToken)) return;
    const file = pendingFile;
    const contentType = file.type;
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

  const tokenReady = isTurnstileVerified(turnstileToken);
  const buttonDisabled =
    !tokenReady ||
    status.kind === "verifying" ||
    status.kind === "uploading" ||
    status.kind === "completing";

  return (
    <section className="mx-auto max-w-3xl p-8">
      <h1 className="mb-4 text-xl font-semibold">Draft 編集ページ</h1>
      <p className="text-sm text-gray-700">
        photobook_id: <code className="font-mono">{photobookId}</code>
      </p>

      <div className="mt-6 rounded-lg border border-dashed border-gray-300 p-6">
        <h2 className="mb-3 text-base font-semibold">写真を追加</h2>
        <p className="mb-3 text-xs text-gray-500">
          JPEG / PNG / WebP、最大 10MB。
        </p>
        <p className="mb-3 text-xs text-amber-700">
          ※ HEIC / HEIF は現在未対応です。iPhone でアップロードする場合は、設定で写真形式を
          「互換性優先（JPEG）」に切り替えるか、JPEG / PNG / WebP に変換してください。
        </p>
        <input
          type="file"
          accept="image/jpeg,image/png,image/webp"
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
              <p className="mb-2 text-xs text-gray-500">
                Bot 検証（Cloudflare Turnstile）を完了してください。
              </p>
              <TurnstileWidget
                sitekey={turnstileSiteKey}
                action="upload"
                onVerify={handleTurnstileVerify}
                onError={handleTurnstileError}
                onExpired={handleTurnstileExpired}
              />
              <p
                className={
                  "mt-2 text-xs " +
                  (tokenReady ? "text-emerald-700" : "text-gray-500")
                }
                aria-live="polite"
                data-testid="turnstile-state"
              >
                {tokenReady ? "Bot 検証成功 ✓ アップロード可能" : "Bot 検証 未完了（widget の challenge を完了してください）"}
              </p>
            </div>

            <button
              type="button"
              onClick={startUpload}
              disabled={buttonDisabled}
              data-testid="upload-button"
              aria-disabled={buttonDisabled}
              className="mt-3 rounded-md bg-teal-600 px-4 py-2 text-sm font-semibold text-white disabled:cursor-not-allowed disabled:opacity-50"
            >
              アップロード開始
            </button>
            {!tokenReady && (
              <p className="mt-1 text-xs text-gray-500">
                ※ Bot 検証が完了するまでアップロードできません。
              </p>
            )}
          </div>
        )}

        <div className="mt-4 text-sm">
          {status.kind === "verifying" && <p>サーバ側で Bot 検証中…</p>}
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
