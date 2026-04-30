// LP / Terms / Privacy / About + 共通 PublicPageFooter の SSR レンダリング検証。
//
// 方針:
//   - renderToStaticMarkup で初期 HTML を検証
//   - data-testid と主要見出し・主要リンクを確認
//   - 動的データ（token / Cookie / Secret / 任意 ID）が出ないことを確認

import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";

import HomePage from "@/app/page";
import TermsPage from "@/app/(public)/terms/page";
import PrivacyPage from "@/app/(public)/privacy/page";
import AboutPage from "@/app/(public)/about/page";
import { PublicPageFooter } from "@/components/Public/PublicPageFooter";

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

describe("PublicPageFooter", () => {
  it("正常_既定リンク_5件_Top_About_Terms_Privacy_Help_を含む", () => {
    const html = renderToStaticMarkup(<PublicPageFooter />);
    expect(html).toContain('data-testid="public-page-footer"');
    expect(html).toContain('href="/"');
    expect(html).toContain('href="/about"');
    expect(html).toContain('href="/terms"');
    expect(html).toContain('href="/privacy"');
    expect(html).toContain('href="/help/manage-url"');
    expect(html).toContain("VRC PhotoBook（非公式ファンメイドサービス）");
  });

  it("正常_links_を引数で上書きできる", () => {
    const html = renderToStaticMarkup(
      <PublicPageFooter
        links={[
          { href: "/about", label: "About" },
          { href: "/terms", label: "Terms" },
        ]}
      />,
    );
    expect(html).toContain('href="/about"');
    expect(html).toContain('href="/terms"');
    expect(html).not.toContain('href="/privacy"');
  });
});

describe("HomePage（LP, /）", () => {
  it("正常_主要セクション_features_cta_policy_と_footer_を含む", () => {
    const html = renderToStaticMarkup(<HomePage />);
    expect(html).toContain('data-testid="lp-header"');
    expect(html).toContain('data-testid="lp-features"');
    expect(html).toContain('data-testid="lp-cta"');
    expect(html).toContain('data-testid="lp-policy"');
    expect(html).toContain('data-testid="public-page-footer"');
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
  it("正常_第1条〜第9条_の見出しを含む", () => {
    const html = renderToStaticMarkup(<TermsPage />);
    expect(html).toContain("利用規約");
    expect(html).toContain("第 1 条 サービスの目的と性質");
    expect(html).toContain("第 2 条 投稿される画像に関する権利と責任");
    expect(html).toContain("第 3 条 禁止事項");
    expect(html).toContain("第 4 条 運営の権限と運用");
    expect(html).toContain("第 5 条 管理 URL の取り扱い");
    expect(html).toContain("第 6 条 サービスの変更・停止");
    expect(html).toContain("第 7 条 免責");
    expect(html).toContain("第 8 条 お問い合わせ・準拠法");
    expect(html).toContain("第 9 条 改訂");
    expect(html).toContain("非公式ファンメイド");
    expect(html).toContain("法律文書としての専門家レビュー");
    expect(html).toContain('data-testid="public-page-footer"');
    expectNoSecret(html);
  });
});

describe("PrivacyPage（/privacy）", () => {
  it("正常_第1条〜第10条_の見出しを含む", () => {
    const html = renderToStaticMarkup(<PrivacyPage />);
    expect(html).toContain("プライバシーポリシー");
    expect(html).toContain("第 1 条 取得する情報");
    expect(html).toContain("第 2 条 利用目的");
    expect(html).toContain("第 3 条 IP ハッシュ・scope ハッシュの取り扱い");
    expect(html).toContain("第 4 条 第三者提供");
    expect(html).toContain("第 5 条 利用する外部サービス");
    expect(html).toContain("第 6 条 保持期間");
    expect(html).toContain("第 7 条 削除請求・権利侵害申立て");
    expect(html).toContain("第 8 条 未成年保護");
    expect(html).toContain("第 9 条 SEO・検索エンジン");
    expect(html).toContain("第 10 条 改訂");
    expect(html).toContain("noindex, nofollow");
    expect(html).toContain("非公式ファンメイド");
    expect(html).toContain("Cloudflare");
    expect(html).toContain("Google Cloud");
    expect(html).toContain('data-testid="public-page-footer"');
    expectNoSecret(html);
  });

  it("正常_メール機能は現在提供していない_と明記", () => {
    const html = renderToStaticMarkup(<PrivacyPage />);
    expect(html).toContain("現在この機能は提供していません");
  });
});

describe("AboutPage（/about）", () => {
  it("正常_主要セクション_できること_できないこと_ポリシーと窓口_を含む", () => {
    const html = renderToStaticMarkup(<AboutPage />);
    expect(html).toContain("VRC PhotoBook について");
    expect(html).toContain("できること");
    expect(html).toContain("MVP ではできないこと");
    expect(html).toContain("ポリシーと窓口");
    expect(html).toContain("非公式ファンメイド");
    expect(html).toContain('href="/terms"');
    expect(html).toContain('href="/privacy"');
    expect(html).toContain('href="/help/manage-url"');
    expect(html).toContain('data-testid="public-page-footer"');
    expectNoSecret(html);
  });
});
