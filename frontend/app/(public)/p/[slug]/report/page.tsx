// /p/[slug]/report 通報フォームページ。
//
// 設計参照:
//   - docs/plan/m2-report-plan.md §7
//
// 仕様:
//   - SSR で対象 photobook の最小情報（title）を表示
//   - photobook が公開対象でない（draft / private / hidden / deleted / purged）
//     場合は ErrorState（not_found / gone）を表示してフォームを出さない
//   - フォーム本体は ReportForm（client component）に委譲
//   - noindex / OGP は default
//
// セキュリティ:
//   - URL に raw token を出さない
//   - manage URL / draft URL / token を画面に出さない
//   - report_id は thanks view に出さない
//
// m2-design-refresh STOP β-5 (本 commit、visual のみ):
//   - design `wf-screens-c.jsx:79-129` (M) / `:130-176` (PC narrow) `WFReport` 視覚整合
//   - PublicTopBar 統合 (showPrimaryCta=false / 通報 context で primary CTA「無料で作る」は違和感のため非表示)
//   - eyebrow「Report」+ h1「{title} を通報」+ back link 既存維持
//   - main wrapper を他公開ページと統一 (max-w-screen-md / px-4 py-6 / sm:px-9 sm:py-9)
//   - data-testid="report-back-link" 維持

import type { Metadata } from "next";
import { notFound } from "next/navigation";
import Link from "next/link";

import { ErrorState } from "@/components/ErrorState";
import { ReportForm } from "@/components/Report/ReportForm";
import { PublicTopBar } from "@/components/Public/PublicTopBar";
import { SectionEyebrow } from "@/components/Public/SectionEyebrow";
import {
  fetchPublicPhotobook,
  isPublicLookupError,
  type PublicPhotobook,
} from "@/lib/publicPhotobook";

export const dynamic = "force-dynamic";

type Params = Promise<{ slug: string }>;

export const metadata: Metadata = {
  title: "通報 | VRC PhotoBook",
  robots: { index: false, follow: false },
};

export default async function ReportPage({ params }: { params: Params }) {
  const { slug } = await params;
  const turnstileSiteKey = process.env.NEXT_PUBLIC_TURNSTILE_SITE_KEY ?? "";
  if (turnstileSiteKey === "") {
    // env 未注入時は通報フォームを開かない（Backend は Turnstile 必須なので無意味）
    return (
      <>
        <PublicTopBar showPrimaryCta={false} />
        <ErrorState variant="server_error" />
      </>
    );
  }

  let photobook: PublicPhotobook;
  try {
    photobook = await fetchPublicPhotobook(slug);
  } catch (e) {
    if (isPublicLookupError(e)) {
      switch (e.kind) {
        case "not_found":
          notFound();
        case "gone":
          return (
            <>
              <PublicTopBar showPrimaryCta={false} />
              <ErrorState variant="gone" />
            </>
          );
        case "server_error":
        case "network":
          return (
            <>
              <PublicTopBar showPrimaryCta={false} />
              <ErrorState variant="server_error" />
            </>
          );
      }
    }
    return (
      <>
        <PublicTopBar showPrimaryCta={false} />
        <ErrorState variant="server_error" />
      </>
    );
  }

  return (
    <>
      <PublicTopBar showPrimaryCta={false} />
      <main className="mx-auto min-h-screen w-full max-w-screen-md px-4 py-6 sm:px-9 sm:py-9">
        <header className="space-y-2">
          <SectionEyebrow>Report</SectionEyebrow>
          <h1 className="text-h1 tracking-tight text-ink sm:text-h1-lg">
            「{photobook.title}」を通報
          </h1>
          <p className="text-sm leading-[1.7] text-ink-medium">
            このフォトブックに問題がある場合は、以下のフォームから運営に通報できます。
          </p>
          <p className="text-xs text-ink-soft">
            <Link
              href={`/p/${photobook.slug}`}
              className="text-teal-600 underline hover:text-teal-700"
              data-testid="report-back-link"
            >
              ← フォトブックに戻る
            </Link>
          </p>
        </header>

        <section className="mt-8">
          <ReportForm slug={photobook.slug} turnstileSiteKey={turnstileSiteKey} />
        </section>

        <footer className="mt-12 border-t border-divider-soft pt-6 text-center text-xs text-ink-soft">
          VRC PhotoBook（非公式ファンメイドサービス）
        </footer>
      </main>
    </>
  );
}
