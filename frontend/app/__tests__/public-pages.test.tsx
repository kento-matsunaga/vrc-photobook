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
import HelpManageUrlPage from "@/app/(public)/help/manage-url/page";

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
  it("正常_design正典_主要セクション_hero_thumbs_features_use_cases_cta_block_footer_を含む", () => {
    const html = renderToStaticMarkup(<HomePage />);
    // β-2a: PublicTopBar を実利用開始 (`design/source/project/wf-shared.jsx:29-48` 相当)
    expect(html).toContain('data-testid="public-topbar"');
    expect(html).toContain('data-testid="public-topbar-cta"');
    // design 正典のセクション (`wf-screens-a.jsx:45-203`)
    expect(html).toContain('data-testid="lp-hero"');
    expect(html).toContain('data-testid="lp-thumbs"');
    expect(html).toContain('data-testid="lp-features"');
    expect(html).toContain('data-testid="lp-use-cases"');
    expect(html).toContain('data-testid="lp-cta-block"');
    expect(html).toContain('data-testid="public-page-footer"');
    // ε-fix: TrustStrip は LP では非表示（実機 smoke で「LP の集中導線を弱める」フィードバック）
    expect(html).not.toContain('data-testid="trust-strip"');
    expect(html).toContain('data-testid="mock-book"');
    // 5 thumbnails ある（最後の 1 つは sm:block で hidden）
    expect(html).toContain('data-testid="mock-thumb-a"');
    expect(html).toContain('data-testid="mock-thumb-b"');
    expect(html).toContain('data-testid="mock-thumb-c"');
    expect(html).toContain('data-testid="mock-thumb-d"');
    expect(html).toContain('data-testid="mock-thumb-e"');
    // 主要文言 (design 正典: `wf-screens-a.jsx:50` h1 / `:51` sub / features `:67-82`)
    expect(html).toContain("VRC写真を、");
    expect(html).toContain("Webフォトブックに。");
    expect(html).toContain("ログイン不要");
    expect(html).toContain("管理URLで編集");
    expect(html).toContain("非公式ファンメイド");
    expect(html).toContain("さあ、あなたの思い出をカタチにしよう");
    // Primary CTA は /create
    expect(html).toContain('data-testid="lp-hero-cta-create"');
    expect(html).toContain('href="/create"');
    expect(html).toContain('data-testid="lp-cta-block-create"');
    // Secondary CTA: 作例 (LP 内 anchor) → #examples / 他 link は PublicTopBar 経由
    expect(html).toContain('data-testid="lp-hero-cta-examples"');
    expect(html).toContain('href="#examples"');
    expect(html).toContain('href="/about"');
    expect(html).toContain('href="/help/manage-url"');
    // policy リンク (PublicPageFooter)
    expect(html).toContain('href="/terms"');
    expect(html).toContain('href="/privacy"');
    expectNoSecret(html);
  });

  it("正常_id_examples_anchor_が_lp-thumbs_に紐付く", () => {
    const html = renderToStaticMarkup(<HomePage />);
    // 「作例を見る」CTA → #examples → サンプル strip の id にスクロール
    expect(html).toMatch(/id="examples"[^>]*data-testid="lp-thumbs"/);
  });

  it("正常_ε-fix_モック表紙文言_おはツイまとめ_を含み_旧文言_は出ない", () => {
    const html = renderToStaticMarkup(<HomePage />);
    // ε-fix: 旧 hero title「ミッドナイト ソーシャルクラブ」/ heroWorld「Midnight Social Club」を撤去
    expect(html).toContain("おはツイ");
    expect(html).toContain("まとめ！");
    expect(html).not.toContain("ミッドナイト");
    expect(html).not.toContain("ソーシャルクラブ");
    expect(html).not.toContain("Midnight Social Club");
  });

  it("正常_ε-fix_CTA_band_文言_ログイン不要で作成できます_に更新_旧_完全無料_は出ない", () => {
    const html = renderToStaticMarkup(<HomePage />);
    expect(html).toContain("ログイン不要で作成できます");
    expect(html).not.toContain("ログイン不要・完全無料");
  });

  it("正常_ε-fix_LP_image_に_object-position_が付与される", () => {
    const html = renderToStaticMarkup(<HomePage />);
    // 縦長は "center 30%" / "center 32%" を持ち、object-cover の中央クロップを補正
    expect(html).toMatch(/object-position\s*:\s*center\s+30%/);
    expect(html).toMatch(/object-position\s*:\s*center\s+32%/);
  });

  it("正常_design正典外の_lp-policy_block_は出ない", () => {
    const html = renderToStaticMarkup(<HomePage />);
    // 旧版の lp-policy section は design に存在しないため削除済み
    expect(html).not.toContain('data-testid="lp-policy"');
  });

  it("正常_β-2c_実画像_hero_mock-cover_sample-01..05_の_webp_jpg_参照が出る", () => {
    const html = renderToStaticMarkup(<HomePage />);
    // hero (MockBook spread top span)
    expect(html).toContain('srcSet="/img/landing/hero.webp"');
    expect(html).toContain('src="/img/landing/hero.jpg"');
    // mock-cover (MockBook 左 cover)
    expect(html).toContain('srcSet="/img/landing/mock-cover.webp"');
    expect(html).toContain('src="/img/landing/mock-cover.jpg"');
    // sample-01..05 (sample strip + MockBook spread bottom 再利用)
    for (let i = 1; i <= 5; i++) {
      const slug = `sample-0${i}`;
      expect(html).toContain(`srcSet="/img/landing/${slug}.webp"`);
      expect(html).toContain(`src="/img/landing/${slug}.jpg"`);
    }
  });

  it("正常_raw_PNG_design_usephot_が_LP_HTML_に出ない", () => {
    const html = renderToStaticMarkup(<HomePage />);
    // β-2c: raw PNG / raw filename / design/usephot path が SSR HTML に漏れない
    expect(html).not.toContain(".png");
    expect(html).not.toContain("design/usephot");
    expect(html).not.toContain("usephot");
    expect(html).not.toMatch(/VRChat_\d{4}-\d{2}-\d{2}/);
    expect(html).not.toContain("82E37915");
  });
});

describe("TermsPage（/terms）", () => {
  it("正常_TOC_第1条〜第9条_PolicyArticle_PublicTopBar_を含む", () => {
    const html = renderToStaticMarkup(<TermsPage />);
    // β-2b-1: PublicTopBar 統合 (`design/source/project/wf-shared.jsx:29-48` 相当)
    expect(html).toContain('data-testid="public-topbar"');
    // β-2b-1: eyebrow inline 「Terms · 最終更新 2026-05-01」(date は eyebrow に統合、ハイフン形式)
    expect(html).toContain("Terms · 最終更新 2026-05-01");
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
    // Terms には trust-strip 不要 (showTrustStrip=false / 既定)
    expect(html).not.toContain('data-testid="trust-strip"');
    expectNoSecret(html);
  });

  it("正常_重要法務文言_権利_免責_準拠法_未成年_通報_管轄_改訂_誹謗中傷_管理URL_が維持されている", () => {
    const html = renderToStaticMarkup(<TermsPage />);
    // 既存 9 article から法務文言が消えていないことを SSR HTML で確認
    expect(html).toContain("権利");
    expect(html).toContain("免責");
    expect(html).toContain("準拠法");
    expect(html).toContain("未成年");
    expect(html).toContain("通報");
    expect(html).toContain("管轄");
    expect(html).toContain("改訂");
    expect(html).toContain("誹謗中傷");
    expect(html).toContain("管理 URL");
    expect(html).toContain("一時非表示");
  });

  it("正常_article数は9件で固定_data-testid_policy-article-terms_カウント", () => {
    // Unit 2 polish: article 数の invariant を data-testid 一致数で固定。
    // article を増減させる変更は本 test を意図的に更新する必要がある。
    const html = renderToStaticMarkup(<TermsPage />);
    const articleMatches = html.match(/data-testid="policy-article-terms-/g) ?? [];
    expect(articleMatches.length).toBe(9);
  });
});

describe("PrivacyPage（/privacy）", () => {
  it("正常_TOC_第1条〜第10条_外部サービスchip5件_PublicTopBar_を含む", () => {
    const html = renderToStaticMarkup(<PrivacyPage />);
    // β-2b-1: PublicTopBar 統合
    expect(html).toContain('data-testid="public-topbar"');
    // β-2b-1: eyebrow inline 「Privacy · 最終更新 2026-05-01」
    expect(html).toContain("Privacy · 最終更新 2026-05-01");
    expect(html).toContain("プライバシーポリシー");
    expect(html).toContain('data-testid="policy-toc"');
    expect(html).toContain('data-testid="policy-notice"');
    expect(html).toContain('data-testid="privacy-external-services"');
    for (let i = 1; i <= 10; i++) {
      expect(html).toContain(`data-testid="policy-article-privacy-${i}"`);
    }
    expect(html).toContain("第 1 条");
    expect(html).toContain("第 10 条");
    // β-2b-1: chip 5 件確定 (Cloudflare Workers / Turnstile / R2 / Cloud Run / Cloud SQL)
    expect(html).toContain("Cloudflare Workers");
    expect(html).toContain("Cloud Run");
    expect(html).toContain("Cloud SQL");
    expect(html).toContain('data-testid="privacy-chip-cloudflare-workers"');
    expect(html).toContain('data-testid="privacy-chip-turnstile"');
    expect(html).toContain('data-testid="privacy-chip-r2"');
    expect(html).toContain('data-testid="privacy-chip-cloud-run"');
    expect(html).toContain('data-testid="privacy-chip-cloud-sql"');
    expect(html).toContain("noindex, nofollow");
    expect(html).toContain('data-testid="public-page-footer"');
    expect(html).not.toContain('data-testid="trust-strip"');
    expectNoSecret(html);
  });

  it("正常_chip数は5件で固定_data-testid_privacy-chip_カウント", () => {
    const html = renderToStaticMarkup(<PrivacyPage />);
    const chipMatches = html.match(/data-testid="privacy-chip-/g) ?? [];
    expect(chipMatches.length).toBe(5);
  });

  it("正常_未採用service_PostHog_Sentry_SecretManager_は不存在", () => {
    const html = renderToStaticMarkup(<PrivacyPage />);
    // β-2b-1: design 正典の chip ('Cloudflare','Turnstile','R2','Sentry','PostHog') のうち
    // production 未採用の Sentry / PostHog は本文・chip いずれにも出さない。
    // Google Secret Manager は infra-only のため privacy chip から削除 (Q-2b1-1 確定)。
    // 本文に補足文も追加しない (Q-2b1-6 確定: 誤読リスク回避)。
    expect(html).not.toContain("PostHog");
    expect(html).not.toContain("Sentry");
    expect(html).not.toContain("Google Secret Manager");
    expect(html).not.toContain("Secret Manager");
  });

  it("正常_メール機能は現在提供していない_と明記", () => {
    const html = renderToStaticMarkup(<PrivacyPage />);
    expect(html).toContain("現在この機能は提供していません");
  });

  it("正常_重要法務文言_個人情報_第三者提供_削除請求_保持期間_ハッシュ_ソルト_EXIF_位置情報_未成年_が維持されている", () => {
    const html = renderToStaticMarkup(<PrivacyPage />);
    expect(html).toContain("第三者");
    expect(html).toContain("削除");
    expect(html).toContain("保持");
    expect(html).toContain("ハッシュ");
    expect(html).toContain("ソルト");
    expect(html).toContain("EXIF");
    expect(html).toContain("位置情報");
    expect(html).toContain("未成年");
    expect(html).toContain("HttpOnly Cookie");
  });

  it("正常_article数は10件で固定_data-testid_policy-article-privacy_カウント", () => {
    // Unit 2 polish: article 数の invariant を data-testid 一致数で固定。
    // article を増減させる変更は本 test を意図的に更新する必要がある。
    const html = renderToStaticMarkup(<PrivacyPage />);
    const articleMatches = html.match(/data-testid="policy-article-privacy-/g) ?? [];
    expect(articleMatches.length).toBe(10);
  });
});

describe("AboutPage（/about）", () => {
  it("正常_PublicTopBar_位置づけ_できる6_できない4_3button_通報補足_trust_を含む", () => {
    const html = renderToStaticMarkup(<AboutPage />);
    // β-2b-2: PublicTopBar 統合
    expect(html).toContain('data-testid="public-topbar"');
    // h1 は Mobile で <br/> 改行するため substring で個別検証
    expect(html).toContain("VRC PhotoBook");
    expect(html).toContain("について");
    expect(html).toContain("サービスの位置づけ");
    expect(html).toContain("できること");
    expect(html).toContain("MVP ではできないこと");
    expect(html).toContain("ポリシーと窓口");
    // dl meta (production truth) は維持
    expect(html).toContain('data-testid="about-positioning-meta"');
    expect(html).toContain("個人運営の非公式ファンメイド");
    expect(html).toContain("ERENOA");
    expect(html).toContain("@Noa_Fortevita");
    // β-2b-2: 3 button block の data-testid と href
    expect(html).toContain('data-testid="about-policy-list"');
    expect(html).toContain('data-testid="about-policy-link-terms"');
    expect(html).toContain('data-testid="about-policy-link-privacy"');
    expect(html).toContain('data-testid="about-policy-link-help-manage-url"');
    expect(html).toContain('href="/terms"');
    expect(html).toContain('href="/privacy"');
    expect(html).toContain('href="/help/manage-url"');
    // β-2b-2: 通報窓口補足は別段落
    expect(html).toContain('data-testid="about-report-note"');
    expect(html).toContain("このフォトブックを通報");
    // footer は維持。ε-fix: trust strip は About でも非表示
    expect(html).toContain('data-testid="public-page-footer"');
    expect(html).not.toContain('data-testid="trust-strip"');
    expectNoSecret(html);
  });

  it("正常_canDo_6件_cannotDo_4件_の数が固定されている", () => {
    const html = renderToStaticMarkup(<AboutPage />);
    const canMatches = html.match(/data-testid="about-can-item-/g) ?? [];
    expect(canMatches.length).toBe(6);
    const cantMatches = html.match(/data-testid="about-cannot-item-/g) ?? [];
    expect(cantMatches.length).toBe(4);
  });

  it("正常_重要文言_個人運営_非公式_noindex_管理URL_通報_プライバシー_権利_Turnstile_IPハッシュ_が維持されている", () => {
    const html = renderToStaticMarkup(<AboutPage />);
    expect(html).toContain("個人運営");
    expect(html).toContain("非公式");
    expect(html).toContain("noindex");
    expect(html).toContain("MVP");
    expect(html).toContain("管理 URL");
    expect(html).toContain("通報");
    expect(html).toContain("プライバシー");
    expect(html).toContain("権利");
    expect(html).toContain("Turnstile");
    expect(html).toContain("IP ハッシュ");
  });
});

describe("HelpManageUrlPage（/help/manage-url）", () => {
  it("正常_PublicTopBar_h1_Q1〜Q6_id_data-testid_を含む", () => {
    const html = renderToStaticMarkup(<HelpManageUrlPage />);
    // β-2b-3: PublicTopBar 統合
    expect(html).toContain('data-testid="public-topbar"');
    // β-2b-3: h1 design 正典「管理 URL の使い方」 (Mobile <br/> 改行 → substring で検証)
    expect(html).toContain("管理 URL の");
    expect(html).toContain("使い方");
    // Q1〜Q6 の id + data-testid (将来 TOC 追加用 anchor)
    for (let i = 1; i <= 6; i++) {
      expect(html).toContain(`id="help-q${i}"`);
      expect(html).toContain(`data-testid="help-q${i}"`);
    }
    // Q プレフィックス
    expect(html).toContain("Q1.");
    expect(html).toContain("Q6.");
    // footer は維持、trust-strip は出さない (showTrustStrip=false / 既定)
    expect(html).toContain('data-testid="public-page-footer"');
    expect(html).not.toContain('data-testid="trust-strip"');
    // β-2b-3: TOC は出さない (Q-2b3-3 確定)
    expect(html).not.toContain('data-testid="policy-toc"');
    expectNoSecret(html);
  });

  it("正常_Q数は6件で固定_data-testid_help-q_カウント", () => {
    const html = renderToStaticMarkup(<HelpManageUrlPage />);
    const matches = html.match(/data-testid="help-q\d+"/g) ?? [];
    expect(matches.length).toBe(6);
  });

  it("正常_重要文言_管理URL_紛失_再表示_メール送信_再選定_編集_削除_公開停止_共有_提供していません_が維持されている", () => {
    const html = renderToStaticMarkup(<HelpManageUrlPage />);
    expect(html).toContain("管理用 URL");
    expect(html).toContain("紛失");
    expect(html).toContain("再表示");
    expect(html).toContain("メール送信");
    expect(html).toContain("再選定");
    expect(html).toContain("編集");
    expect(html).toContain("削除");
    expect(html).toContain("公開停止");
    expect(html).toContain("共有");
    expect(html).toContain("提供していません");
  });
});
