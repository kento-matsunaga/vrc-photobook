// /create — フォトブック作成入口（Server Component）。
//
// m2-design-refresh STOP β-3:
//   - PublicTopBar 統合 (LP / About / Terms / Privacy / Help と一貫した shell)
//   - eyebrow を design 正典「Step 1 / 3」(動線進捗ステップ)
//   - main wrapper を他公開ページと統一 (max-w-screen-md / px-4 py-6 / sm:px-9 sm:py-9)
//   - PublicPageFooter showTrustStrip=false (default、policy / FAQ ページと一貫)
//
// 設計参照:
//   - design/source/project/wf-screens-a.jsx:206-308 (Create M / PC)
//   - design/source/project/wireframe-styles.css:351-369 (.wf-h1 / .wf-eyebrow)
//   - docs/plan/m2-design-refresh-stop-beta-3-plan.md §1
//   - docs/plan/m2-create-entry-plan.md §3 / §8
//   - 業務知識 v4 §3.1
//
// 制約:
//   - middleware で X-Robots-Tag: noindex, nofollow / Referrer-Policy が付与される
//   - Turnstile site key は build-time inject（NEXT_PUBLIC_TURNSTILE_SITE_KEY）
//   - raw token / Cookie / Secret 値は出さない（CreateClient 側でも出さない）

import type { Metadata } from "next";

import { ErrorState } from "@/components/ErrorState";
import { PublicPageFooter } from "@/components/Public/PublicPageFooter";
import { PublicTopBar } from "@/components/Public/PublicTopBar";
import { SectionEyebrow } from "@/components/Public/SectionEyebrow";

import { CreateClient } from "./CreateClient";

export const metadata: Metadata = {
  title: "フォトブックを作る｜VRC PhotoBook",
  description:
    "VRChat の写真をまとめて、ログイン不要でフォトブックを作成できます。タイプを選んで、すぐに編集を始められます。",
};

export default function CreatePage() {
  const turnstileSiteKey = process.env.NEXT_PUBLIC_TURNSTILE_SITE_KEY ?? "";
  if (turnstileSiteKey === "") {
    // env 未注入時は作成フォームを開かない（Backend は Turnstile 必須なので無意味）
    return (
      <>
        <PublicTopBar />
        <main className="mx-auto min-h-screen w-full max-w-screen-md px-4 py-6 sm:px-9 sm:py-9">
          <ErrorState variant="server_error" />
          <PublicPageFooter />
        </main>
      </>
    );
  }

  return (
    <>
      <PublicTopBar />
      <main
        className="mx-auto min-h-screen w-full max-w-screen-md px-4 py-6 sm:px-9 sm:py-9"
        data-testid="create-page"
      >
        <header className="space-y-2">
          {/* design `wf-screens-a.jsx:260` PC eyebrow「Step 1 / 3」/ Mobile :211「Step 1」 */}
          <SectionEyebrow>Step 1 / 3</SectionEyebrow>
          <h1 className="text-h1 tracking-tight text-ink sm:text-h1-lg">
            どんなフォトブックを作りますか？
          </h1>
          <p className="text-sm text-ink-medium">
            まずはタイプを選んでください。タイトルや作成者名は後でも変更できます。
          </p>
        </header>

        <CreateClient turnstileSiteKey={turnstileSiteKey} />

        <PublicPageFooter />
      </main>
    </>
  );
}
