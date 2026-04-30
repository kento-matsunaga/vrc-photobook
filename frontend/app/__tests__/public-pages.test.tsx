// LP / Terms / Privacy / About の SSR レンダリング検証（design rebuild 版）。
//
// 方針:
//   - renderToStaticMarkup で初期 HTML を検証
//   - data-testid と主要見出し・主要リンク・新コンポーネントの存在を確認
//   - 動的データ（token / Cookie / Secret / 任意 ID）が出ないことを確認

import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";

import HomePage from "@/app/page";
import TermsPage from "@/app/(public)/terms/page";
import PrivacyPage from "@/app/(public)/privacy/page";
import AboutPage from "@/app/(public)/about/page";

const FORBIDDEN_PATTERNS = [
  /\bSecret\s*[:=]\s*[A-Za-z0-9_+/=-]{12,}/i,
  /\bDATABASE_URL\s*=\s*postgres/i,
  /\bsk_live_[A-Za-z0-9]+/,
  /\bsk_test_[A-Za-z0-9]+/,
  /\bBearer\s+[A-Za-z0-9._-]{20,}/,
  /\bSet-Cookie:/i,
  /\bmanage_url_token=/i,
  /\bdraft_edit_token=/i,
  /\bsession_token=/i,
];

function expectNoSecret(html: string) {
  for (const re of FORBIDDEN_PATTERNS) {
    expect(html).not.toMatch(re);
  }
}

describe("HomePage（LP, /）", () => {
  it("正常_主要セクション_hero_thumbs_features_policy_cta_block_を含む", () => {
    const html = renderToStaticMarkup(<HomePage />);
    expect(html).toContain('data-testid="lp-hero"');
    expect(html).toContain('data-testid="lp-thumbs"');
    expect(html).toContain('data-testid="lp-features"');
    expect(html).toContain('data-testid="lp-policy"');
    expect(html).toContain('data-testid="lp-cta-block"');
    expect(html).toContain('data-testid="public-page-footer"');
    expect(html).toContain('data-testid="trust-strip"');
    expect(html).toContain('data-testid="mock-book"');
    // 5 thumbnails ある（最後の 1 つは sm:block で hidden）
    expect(html).toContain('data-testid="mock-thumb-a"');
    expect(html).toContain('data-testid="mock-thumb-b"');
    expect(html).toContain('data-testid="mock-thumb-c"');
    expect(html).toContain('data-testid="mock-thumb-d"');
    expect(html).toContain('data-testid="mock-thumb-e"');
    // 主要文言
    expect(html).toContain("VRChat 写真を、");
    expect(html).toContain("ログイン不要");
    expect(html).toContain("管理 URL");
    expect(html).toContain("非公式ファンメイド");
    // CTA リンク先（about / help/manage-url）
    expect(html).toContain('href="/about"');
    expect(html).toContain('href="/help/manage-url"');
    // policy リンク
    expect(html).toContain('href="/terms"');
    expect(html).toContain('href="/privacy"');
    expectNoSecret(html);
  });
});

describe("TermsPage（/terms）", () => {
  it("正常_TOC_第1条〜第9条_PolicyArticle_を含む", () => {
    const html = renderToStaticMarkup(<TermsPage />);
    expect(html).toContain("利用規約");
    expect(html).toContain('data-testid="policy-toc"');
    expect(html).toContain('data-testid="policy-notice"');
    // 各 article (id="terms-1" 〜 "terms-9")
    for (let i = 1; i <= 9; i++) {
      expect(html).toContain(`data-testid="policy-article-terms-${i}"`);
    }
    expect(html).toContain("第 1 条");
    expect(html).toContain("第 9 条");
    expect(html).toContain("非公式ファンメイド");
    expect(html).toContain("法律文書としての専門家レビュー");
    // Anchor scroll 用 href
    expect(html).toContain('href="#terms-1"');
    expect(html).toContain('href="#terms-9"');
    expect(html).toContain('data-testid="public-page-footer"');
    // Terms には trust-strip 不要
    expect(html).not.toContain('data-testid="trust-strip"');
    expectNoSecret(html);
  });
});

describe("PrivacyPage（/privacy）", () => {
  it("正常_TOC_第1条〜第10条_外部サービスchip_を含む", () => {
    const html = renderToStaticMarkup(<PrivacyPage />);
    expect(html).toContain("プライバシーポリシー");
    expect(html).toContain('data-testid="policy-toc"');
    expect(html).toContain('data-testid="policy-notice"');
    expect(html).toContain('data-testid="privacy-external-services"');
    for (let i = 1; i <= 10; i++) {
      expect(html).toContain(`data-testid="policy-article-privacy-${i}"`);
    }
    expect(html).toContain("第 1 条");
    expect(html).toContain("第 10 条");
    expect(html).toContain("Cloudflare Workers");
    expect(html).toContain("Google Cloud Run");
    expect(html).toContain("noindex, nofollow");
    expect(html).toContain('data-testid="public-page-footer"');
    expect(html).not.toContain('data-testid="trust-strip"');
    expectNoSecret(html);
  });

  it("正常_メール機能は現在提供していない_と明記", () => {
    const html = renderToStaticMarkup(<PrivacyPage />);
    expect(html).toContain("現在この機能は提供していません");
  });
});

describe("AboutPage（/about）", () => {
  it("正常_位置づけ_できる6_できない4_ポリシー_trust_を含む", () => {
    const html = renderToStaticMarkup(<AboutPage />);
    expect(html).toContain("VRC PhotoBook について");
    expect(html).toContain("サービスの位置づけ");
    expect(html).toContain("できること");
    expect(html).toContain("MVP ではできないこと");
    expect(html).toContain("ポリシーと窓口");
    expect(html).toContain("非公式ファンメイド");
    expect(html).toContain("ERENOA");
    expect(html).toContain("@Noa_Fortevita");
    expect(html).toContain('href="/terms"');
    expect(html).toContain('href="/privacy"');
    expect(html).toContain('href="/help/manage-url"');
    expect(html).toContain('data-testid="public-page-footer"');
    expect(html).toContain('data-testid="trust-strip"');
    expectNoSecret(html);
  });
});
