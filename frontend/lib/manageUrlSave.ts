// manageUrlSave: 管理 URL の Provider 不要保存ヘルパ群。
//
// 設計参照:
//   - docs/plan/m2-email-provider-reselection-plan.md §7（Complete 画面 Provider 不要改善）
//   - docs/adr/0006-email-provider-and-manage-url-delivery.md（メール送信なし MVP）
//
// セキュリティ:
//   - 管理 URL は token 相当。本モジュールは値を console.log に出さない / Service Worker
//     に渡さない / localStorage / IndexedDB に書かない（pr-closeout.md / security-guard.md）。
//   - 呼び出し元（UI コンポーネント）も同じ原則を守る。
//   - .txt download は Blob URL を即 revoke して長期保持しない。
//   - mailto href は encodeURIComponent で改行 / 制御文字 / + 等を escape。

const TXT_DEFAULT_BASENAME = "vrc-photobook-manage-url";

// buildManageUrlTxtContent は .txt download に書き込む内容を作る。
// 余計な内部情報（photobook_id / token version / storage_key 等）は含めない。
export function buildManageUrlTxtContent(manageURL: string): string {
  return [
    "VRC PhotoBook 管理用 URL（自分用、再表示できません）",
    "",
    manageURL,
    "",
    "このリンクを失うと、編集や公開停止ができなくなります。",
    "パスワードマネージャや安全な場所に保管してください。",
    "",
  ].join("\n");
}

// sanitizeSlug はファイル名に安全な slug 部分のみを抽出する。
// 想定 slug は 12〜20 文字の小文字英数 + ハイフン（backend slug VO と一致）。
// 想定外の文字は除外し、空になる場合は呼び出し側で baseName のみを使う。
export function sanitizeSlug(raw: string): string {
  return raw.toLowerCase().replace(/[^a-z0-9-]/g, "").slice(0, 24);
}

// buildManageUrlTxtFileName はダウンロード時のファイル名を作る。
// slug が安全に使えれば付与し、そうでなければ baseName のみを返す。
export function buildManageUrlTxtFileName(rawSlug: string | undefined): string {
  if (!rawSlug) return `${TXT_DEFAULT_BASENAME}.txt`;
  const safe = sanitizeSlug(rawSlug);
  if (safe.length === 0) return `${TXT_DEFAULT_BASENAME}.txt`;
  return `${TXT_DEFAULT_BASENAME}-${safe}.txt`;
}

// buildMailtoHref は mailto: ハンドラ用 href を生成する。
//
// subject / body は固定 + URL のみで、photobook_id / token version 等の付加情報を含めない。
// 改行 / 特殊文字は encodeURIComponent でエスケープする。
export function buildMailtoHref(manageURL: string): string {
  const subject = "VRC PhotoBook 管理URL（自分用）";
  const body = [
    "VRC PhotoBook の管理用 URL です。",
    "このリンクを失うと、編集や公開停止ができなくなります。",
    "",
    manageURL,
    "",
  ].join("\n");
  return `mailto:?subject=${encodeURIComponent(subject)}&body=${encodeURIComponent(body)}`;
}

// triggerTxtDownload はブラウザの download 動作を起動する副作用関数。
//
// 実行順:
//  1. Blob を text/plain で組み立て
//  2. URL.createObjectURL で blob URL を作成
//  3. <a download> 要素を作って click
//  4. blob URL を即 revoke（長期保持しない）
//
// SSR 環境（document が無い）では何もしない（呼び出し側で `"use client"` 必須）。
export function triggerTxtDownload(filename: string, content: string): void {
  if (typeof document === "undefined" || typeof URL === "undefined") {
    return;
  }
  const blob = new Blob([content], { type: "text/plain;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  a.rel = "noopener";
  // body に attach せずに click するブラウザもあるので、念のため append する
  document.body.appendChild(a);
  try {
    a.click();
  } finally {
    document.body.removeChild(a);
    // blob URL を即 revoke して GC を待たずに開放する
    URL.revokeObjectURL(url);
  }
}
