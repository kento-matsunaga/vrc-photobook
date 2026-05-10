// ManageVisibilitySensitiveForm: 公開設定 (visibility unlisted/private) + sensitive 切替 (M-1a)。
//
// 設計参照:
//   - docs/plan/m-1-manage-mvp-safety-plan.md §3.2.2 / §3.2.3 / §3.3.1
//   - 業務知識 v4 §3.4 (公開範囲 / センシティブ設定の変更)
//
// 振る舞い:
//   - radio: unlisted / private（public は disabled + 注記）
//   - toggle: sensitive ON/OFF
//   - 「公開設定を保存」ボタン → 個別 PATCH 2 本（visibility が変わったら、sensitive が
//     変わったら、それぞれ 1 回ずつ）
//   - 成功時は内部 version を更新 + 保存済み notice を表示
//   - 失敗時はエラー文言（OCC は「再読込して再操作」を促す）
//
// 注意:
//   - public 化は publish flow 限定（業務知識 v4 §3.2 / Q6）
//   - destructive ではないため confirm modal なし（破壊系は M-1b の別 component）
"use client";

import { useState } from "react";

import {
  isManageMutationError,
  type ManageMutationError,
  updateSensitiveFromManage,
  updateVisibilityFromManage,
} from "@/lib/managePhotobook";

type Visibility = "public" | "unlisted" | "private";

type Props = {
  photobookId: string;
  initialVersion: number;
  initialVisibility: Visibility;
  initialSensitive: boolean;
};

const ERROR_LABEL: Record<ManageMutationError["kind"], string> = {
  unauthorized: "管理セッションが期限切れです。管理 URL から再度入場してください。",
  not_found: "対象のフォトブックが見つかりません。",
  version_conflict: "他の編集が反映されました。ページを再読込して再度お試しください。",
  public_change_not_allowed: "公開（誰でも閲覧可）への変更は管理ページからは行えません。",
  not_draft: "操作できません。",
  invalid_payload: "入力値に問題があります。",
  server_error: "一時的なエラーが発生しました。しばらくしてから再度お試しください。",
  network: "通信に失敗しました。ネットワークをご確認のうえ再度お試しください。",
};

export function ManageVisibilitySensitiveForm({
  photobookId,
  initialVersion,
  initialVisibility,
  initialSensitive,
}: Props) {
  const [visibility, setVisibility] = useState<Visibility>(initialVisibility);
  const [sensitive, setSensitive] = useState(initialSensitive);
  const [savedVisibility, setSavedVisibility] = useState<Visibility>(initialVisibility);
  const [savedSensitive, setSavedSensitive] = useState(initialSensitive);
  const [version, setVersion] = useState(initialVersion);
  const [busy, setBusy] = useState(false);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);
  const [savedNote, setSavedNote] = useState<string | null>(null);

  const dirty = visibility !== savedVisibility || sensitive !== savedSensitive;

  const onSave = async () => {
    if (busy || !dirty) return;
    setBusy(true);
    setErrorMsg(null);
    setSavedNote(null);
    let curVersion = version;
    try {
      if (visibility !== savedVisibility) {
        if (visibility === "public") {
          // UI 側で disabled だが防御的 reject
          throw { kind: "public_change_not_allowed" } satisfies ManageMutationError;
        }
        const out = await updateVisibilityFromManage(photobookId, visibility, curVersion);
        curVersion = out.version;
        setSavedVisibility(visibility);
      }
      if (sensitive !== savedSensitive) {
        const out = await updateSensitiveFromManage(photobookId, sensitive, curVersion);
        curVersion = out.version;
        setSavedSensitive(sensitive);
      }
      setVersion(curVersion);
      setSavedNote("公開設定を保存しました");
    } catch (e) {
      if (isManageMutationError(e)) {
        setErrorMsg(ERROR_LABEL[e.kind] ?? ERROR_LABEL.server_error);
      } else {
        setErrorMsg(ERROR_LABEL.server_error);
      }
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="space-y-4" data-testid="manage-visibility-sensitive-form">
      <div>
        <p className="text-sm font-bold text-ink-strong">公開範囲</p>
        <fieldset className="mt-2 space-y-1.5 text-sm text-ink-strong">
          <label className="flex items-start gap-2">
            <input
              type="radio"
              name="visibility"
              value="unlisted"
              checked={visibility === "unlisted"}
              onChange={() => setVisibility("unlisted")}
              disabled={busy}
              data-testid="manage-visibility-radio-unlisted"
              className="mt-0.5"
            />
            <span>
              <span className="font-bold">限定公開（unlisted）</span>
              <span className="ml-1 text-xs text-ink-medium">
                — URL を知っている人のみ閲覧可能
              </span>
            </span>
          </label>
          <label className="flex items-start gap-2">
            <input
              type="radio"
              name="visibility"
              value="private"
              checked={visibility === "private"}
              onChange={() => setVisibility("private")}
              disabled={busy}
              data-testid="manage-visibility-radio-private"
              className="mt-0.5"
            />
            <span>
              <span className="font-bold">非公開（private）</span>
              <span className="ml-1 text-xs text-ink-medium">
                — 公開 URL を知っていても閲覧できません
              </span>
            </span>
          </label>
          <label className="flex items-start gap-2 opacity-60" aria-disabled="true">
            <input
              type="radio"
              name="visibility"
              value="public"
              checked={false}
              disabled
              data-testid="manage-visibility-radio-public-disabled"
              className="mt-0.5"
            />
            <span>
              <span className="font-bold">公開（public）</span>
              <span className="ml-1 text-xs text-ink-medium">
                — 公開時にのみ設定可能（管理ページからは変更できません）
              </span>
            </span>
          </label>
        </fieldset>
      </div>

      <div>
        <p className="text-sm font-bold text-ink-strong">センシティブ設定</p>
        <label className="mt-2 inline-flex items-start gap-2 text-sm text-ink-strong">
          <input
            type="checkbox"
            checked={sensitive}
            onChange={() => setSensitive((v) => !v)}
            disabled={busy}
            data-testid="manage-sensitive-toggle"
            className="mt-0.5"
          />
          <span>
            <span className="font-bold">被写体配慮のためセンシティブ表示にする</span>
            <span className="ml-1 text-xs text-ink-medium">
              — ON にすると公開ページで閲覧前にワンクッション表示します
            </span>
          </span>
        </label>
      </div>

      <button
        type="button"
        data-testid="manage-visibility-sensitive-save"
        onClick={onSave}
        disabled={busy || !dirty}
        className="inline-flex h-10 items-center justify-center rounded-md bg-brand-teal px-5 text-sm font-bold text-white shadow-sm transition-colors hover:bg-brand-teal-hover disabled:cursor-not-allowed disabled:opacity-60"
      >
        {busy ? "保存中…" : "公開設定を保存"}
      </button>

      {errorMsg !== null && (
        <p
          role="alert"
          data-testid="manage-visibility-sensitive-error"
          className="text-xs leading-[1.6] text-status-warning"
        >
          {errorMsg}
        </p>
      )}
      {savedNote !== null && (
        <p
          role="status"
          data-testid="manage-visibility-sensitive-saved"
          className="text-xs leading-[1.6] text-teal-700"
        >
          {savedNote}
        </p>
      )}
    </div>
  );
}
