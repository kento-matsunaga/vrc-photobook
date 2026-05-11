// CompleteView: publish 直後に表示する完了画面。
//
// セキュリティ:
//   - manage URL は raw token を含むため、URL bar / log / localStorage に出さない
//   - 本コンポーネントの prop で manage_url_path を受けるが、URL 遷移しない（display only）
//   - reload で props は失われる（intentional、再表示防止）
//
// Provider 不要 改善（PR32b、ADR-0006）:
//   - ManageUrlSavePanel: .txt download / mailto / 「保存しました」確認チェックを集約
//   - チェック前は閉じる導線（編集ページに戻る / 公開ページを開く）に注意文を出し、
//     ボタン自体は disable しない（誤誘導防止のため警告中心、選択肢は奪わない）
//
// M-2 STOP δ (ADR-0007):
//   - photobookId prop を追加し、Backend `/api/public/photobooks/{id}/ogp` を 2 s 間隔 /
//     最大 30 s polling して OGP readiness を確認する
//   - polling 中は「公開ページを開く」/ 共有 (UrlCopyPanel public copy) を disable + spinner
//   - generated 到達で全 enable、テキスト「OGP プレビュー画像の準備が完了しました」
//   - 30 s timeout で enable + info「OGP 画像の反映に少し時間がかかっています。SNS で
//     プレビューが表示されない場合は数十秒後に再度お試しください。」
//   - polling 失敗 (network / error / 例外) は **公開完了扱いを壊さない**。timeout 後
//     enable 経路で扱う
//   - photobookId / raw API URL は DOM / aria-label / log に残さない（fetch 内に閉じる）
//
// m2-design-refresh STOP β-4:
//   - design `wf-screens-b.jsx:197-246` (M) / `:247-293` (PC) `WFComplete` 視覚整合
//   - eyebrow「Status: PUBLISHED」+ h1「フォトブックを公開しました」(design 通り)
//   - PC は wf-grid-2 で公開 URL + 管理 URL 並列、Mobile は縦 stack
//   - PublicTopBar 統合 (showPrimaryCta=false、draft → published 完了画面で違和感回避)
//   - design footer FAQ link / wf-note 視覚整合
//   - 全 data-testid (complete-view / complete-actions / complete-open-viewer /
//     complete-back-to-edit / complete-save-reminder / complete-faq-link) **完全維持**
//   - savedConfirmed state / extractSlugFromPublicPath / onBackToEdit handler /
//     ManageUrlSavePanel / ManageUrlWarning / UrlCopyPanel logic は **触らない**
"use client";

import { useEffect, useRef, useState } from "react";

import { PublicTopBar } from "@/components/Public/PublicTopBar";
import { SectionEyebrow } from "@/components/Public/SectionEyebrow";
import { fetchOgpReadinessClient } from "@/lib/ogpReadiness";

import { ManageUrlSavePanel } from "./ManageUrlSavePanel";
import { ManageUrlWarning } from "./ManageUrlWarning";
import { UrlCopyPanel } from "./UrlCopyPanel";

type Props = {
  appBaseUrl: string;
  photobookId: string;
  publicUrlPath: string;
  manageUrlPath: string;
  onBackToEdit: () => void;
};

// M-2 (ADR-0007): polling 間隔 / timeout。renderer warm 10-13 ms + R2 PUT p95 200 ms +
// 同期 timeout 2.5 s + worker fallback 60 s 想定に対し、user 体感の上限として 30 s を採用。
// User-facing なので const 直書きで明示する。
const OGP_POLL_INTERVAL_MS = 2_000;
const OGP_POLL_TIMEOUT_MS = 30_000;

// OGP polling phase。共有 UI の disable / enable / 文言切替えで使う。
type OgpPhase = "checking" | "ready" | "timeout";

// extractSlugFromManagePath は public_url_path から slug を取り出す（無ければ undefined）。
// 想定形式: "/p/<slug>" または "/manage/<...>"。manage_url_path は token 含むので使わない。
function extractSlugFromPublicPath(publicUrlPath: string): string | undefined {
  const m = publicUrlPath.match(/^\/p\/([^/?#]+)/);
  return m ? m[1] : undefined;
}

export function CompleteView({
  appBaseUrl,
  photobookId,
  publicUrlPath,
  manageUrlPath,
  onBackToEdit,
}: Props) {
  const base = appBaseUrl.replace(/\/$/, "");
  const publicURL = `${base}${publicUrlPath}`;
  const manageURL = `${base}${manageUrlPath}`;
  const slug = extractSlugFromPublicPath(publicUrlPath);

  const [savedConfirmed, setSavedConfirmed] = useState(false);

  // M-2 STOP δ: OGP readiness polling。public endpoint への fetch を 2 s 間隔で繰り返す。
  // ready / timeout のいずれかに至ったら interval を停止する。
  const [ogpPhase, setOgpPhase] = useState<OgpPhase>("checking");
  const ogpPhaseRef = useRef<OgpPhase>("checking");
  useEffect(() => {
    let cancelled = false;
    const abortController = new AbortController();
    const startedAt = Date.now();

    const tick = async () => {
      if (cancelled) return;
      const status = await fetchOgpReadinessClient(photobookId, abortController.signal);
      if (cancelled) return;
      if (status === "generated") {
        ogpPhaseRef.current = "ready";
        setOgpPhase("ready");
        return;
      }
      // pending / not_found / error はいずれも polling 継続 (ADR-0007 §3 (4))。
      // timeout 判定は次回 tick で interval の clearInterval により打ち切られる。
    };

    // 即時 1 回 + その後 2 s 間隔。
    void tick();
    const intervalId = window.setInterval(() => {
      if (cancelled) return;
      if (Date.now() - startedAt >= OGP_POLL_TIMEOUT_MS) {
        if (ogpPhaseRef.current !== "ready") {
          ogpPhaseRef.current = "timeout";
          setOgpPhase("timeout");
        }
        window.clearInterval(intervalId);
        return;
      }
      void tick();
    }, OGP_POLL_INTERVAL_MS);

    return () => {
      cancelled = true;
      abortController.abort();
      window.clearInterval(intervalId);
    };
  }, [photobookId]);

  // ready / timeout 後は共有 CTA を enable。checking 中のみ disable + spinner で誘導抑止。
  const shareEnabled = ogpPhase !== "checking";

  return (
    <>
      {/* draft → published 完了画面、PublicTopBar 統合 (showPrimaryCta=false) */}
      <PublicTopBar showPrimaryCta={false} />
      <main
        className="mx-auto min-h-screen w-full max-w-screen-md px-4 py-6 sm:px-9 sm:py-9"
        data-testid="complete-view"
      >
        <header className="space-y-2">
          <SectionEyebrow>Status: PUBLISHED</SectionEyebrow>
          <h1 className="text-h1 tracking-tight text-ink sm:text-h1-lg">
            フォトブックを公開しました
          </h1>
          <p className="text-sm leading-[1.7] text-ink-medium">
            公開 URL を SNS や友人にシェアできます。管理用 URL は再発行までの間、
            フォトブックを編集できる唯一の鍵です。
          </p>
        </header>

        {/* design `wf-screens-b.jsx:254-272` PC wf-grid-2 / Mobile 縦 stack で
            公開 URL + 管理 URL を並列表示 */}
        <div className="mt-7 grid grid-cols-1 gap-4 sm:grid-cols-2 sm:gap-5">
          <UrlCopyPanel
            kind="public"
            label="公開 URL"
            url={publicURL}
            helper="このページを VRChat やフレンドに共有できます。"
            testId="complete-public-url"
          />
          <UrlCopyPanel
            kind="manage"
            label="管理用 URL（再表示できません）"
            url={manageURL}
            helper="このリンクを失うと、編集や公開停止ができなくなります。安全に保管してください。"
            testId="complete-manage-url"
          />
        </div>

        <div className="mt-5">
          <ManageUrlWarning />
        </div>

        <div className="mt-5">
          <ManageUrlSavePanel
            manageURL={manageURL}
            slug={slug}
            saved={savedConfirmed}
            onSavedChange={setSavedConfirmed}
          />
        </div>

        {/* M-2 STOP δ: OGP readiness 状態表示。data-testid は phase ごとに分け、Workers
            production chunk の grep 検証に使う（raw photobook_id は出さない）。 */}
        <div className="mt-5" data-testid={`complete-ogp-${ogpPhase}`}>
          {ogpPhase === "checking" && (
            <div
              className="flex items-start gap-2.5 rounded-lg border border-divider-soft bg-surface-soft p-3.5 text-xs leading-[1.6] text-ink-medium"
              role="status"
              aria-live="polite"
            >
              <span
                aria-hidden="true"
                className="inline-block h-4 w-4 shrink-0 animate-spin rounded-full border-2 border-teal-400 border-t-transparent"
              />
              <p className="flex-1">
                OGP プレビュー画像の準備中です。SNS シェアの前に少しお待ちください。
              </p>
            </div>
          )}
          {ogpPhase === "ready" && (
            <div
              className="flex items-start gap-2.5 rounded-lg border-l-[3px] border-status-success bg-status-success-soft p-3.5 text-xs leading-[1.6] text-ink-strong"
              role="status"
              aria-live="polite"
            >
              <p className="flex-1">
                OGP プレビュー画像の準備が完了しました。公開 URL を SNS にシェアできます。
              </p>
            </div>
          )}
          {ogpPhase === "timeout" && (
            <div
              className="flex items-start gap-2.5 rounded-lg border-l-[3px] border-teal-300 bg-teal-50 p-3.5 text-xs leading-[1.6] text-ink-strong"
              role="status"
              aria-live="polite"
            >
              <p className="flex-1">
                OGP 画像の反映に少し時間がかかっています。SNS でプレビューが表示されない場合は
                数十秒後に再度お試しください。
              </p>
            </div>
          )}
        </div>

        <section
          className="mt-7 flex flex-col gap-3 border-t border-divider-soft pt-5 sm:flex-row sm:items-center sm:justify-between"
          data-testid="complete-actions"
        >
          {/* M-2 STOP δ: ready 前は disable + spinner で誘導抑止。
              ready / timeout 後は通常の <a> として enable する。 */}
          {shareEnabled ? (
            <a
              href={publicURL}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex h-12 items-center justify-center rounded-[10px] bg-brand-teal px-6 text-sm font-bold text-white shadow-sm transition-colors hover:bg-brand-teal-hover"
              data-testid="complete-open-viewer"
            >
              公開ページを開く
            </a>
          ) : (
            <button
              type="button"
              disabled
              aria-disabled="true"
              aria-label="公開ページを開く（OGP プレビュー準備中）"
              className="inline-flex h-12 cursor-not-allowed items-center justify-center gap-2 rounded-[10px] bg-brand-teal/60 px-6 text-sm font-bold text-white shadow-sm"
              data-testid="complete-open-viewer"
            >
              <span
                aria-hidden="true"
                className="inline-block h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent"
              />
              公開ページを開く
            </button>
          )}
          <button
            type="button"
            onClick={onBackToEdit}
            className="inline-flex h-12 items-center justify-center rounded-[10px] border border-divider bg-surface px-6 text-sm font-semibold text-ink-strong transition-colors hover:border-teal-300 hover:text-teal-700"
            data-testid="complete-back-to-edit"
          >
            編集ページに戻る
          </button>
        </section>

        {!savedConfirmed && (
          <div
            className="mt-5 flex items-start gap-2.5 rounded-lg border-l-[3px] border-status-warn bg-status-warn-soft p-3.5"
            role="status"
            data-testid="complete-save-reminder"
          >
            <span
              aria-hidden="true"
              className="grid h-[18px] w-[18px] shrink-0 place-items-center rounded-full bg-status-warn font-serif text-xs font-bold italic leading-none text-white"
            >
              !
            </span>
            <p className="flex-1 text-xs leading-[1.6] text-ink-strong">
              管理用 URL の保存をまだ確認していません。上の保存方法のいずれかで保管したら、
              チェックボックスをオンにしてください。
            </p>
          </div>
        )}

        <footer className="mt-10 border-t border-divider-soft pt-6 text-center text-xs text-ink-soft">
          <p>VRC PhotoBook（非公式ファンメイドサービス）</p>
          <p className="mt-1">
            管理 URL の保存方法・紛失時のご案内は{" "}
            <a
              href="/help/manage-url"
              className="text-teal-600 underline hover:text-teal-700"
              data-testid="complete-faq-link"
            >
              よくある質問
            </a>
            。
          </p>
        </footer>
      </main>
    </>
  );
}
