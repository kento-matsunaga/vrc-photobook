// /create — フォトブック作成入口（Server Component）。
//
// 設計参照:
//   - docs/plan/m2-create-entry-plan.md §3 / §8
//   - 業務知識 v4 §3.1（タイプ選択時に server draft Photobook を作成、final closeout 時 §3.1 改定予定）
//
// 制約:
//   - middleware で X-Robots-Tag: noindex, nofollow / Referrer-Policy が付与される
//   - Turnstile site key は build-time inject（NEXT_PUBLIC_TURNSTILE_SITE_KEY）
//   - raw token / Cookie / Secret 値は出さない（CreateClient 側でも出さない）

import type { Metadata } from "next";

import { ErrorState } from "@/components/ErrorState";
import { PublicPageFooter } from "@/components/Public/PublicPageFooter";
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
      <main className="mx-auto min-h-screen w-full max-w-screen-md bg-surface px-4 py-8 sm:px-6 sm:py-10">
        <ErrorState variant="server_error" />
        <PublicPageFooter />
      </main>
    );
  }

  return (
    <main
      className="mx-auto min-h-screen w-full max-w-screen-md bg-surface px-4 py-8 sm:px-6 sm:py-10"
      data-testid="create-page"
    >
      <header className="space-y-2">
        <SectionEyebrow>Create</SectionEyebrow>
        <h1 className="text-h1 text-ink">どんなフォトブックを作りますか？</h1>
        <p className="text-sm text-ink-medium">
          まずはタイプを選んでください。タイトルや作成者名は後でも変更できます。
        </p>
      </header>

      <CreateClient turnstileSiteKey={turnstileSiteKey} />

      <PublicPageFooter />
    </main>
  );
}
