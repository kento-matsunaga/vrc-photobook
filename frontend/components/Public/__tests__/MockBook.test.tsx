// MockBook / MockThumb の SSR レンダリング検証。
//
// 観点 (m2-design-refresh STOP β-2a):
//   - design 正典 (`design/source/project/wf-screens-a.jsx:4-43`) の構造を満たす
//     - 左 cover (width 58%) + 右 page (width 48%, 2x2 grid)
//     - 右 page の 3 cell は aria-hidden="true"
//   - mock-book は title / date / worldLabel を実テキストで表示
//   - 旧 PC 専用 rotate 小カード装飾は削除されている（design に無い）
//   - MockThumb 5 variants の data-testid を含む

import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";

import { MockBook, MockThumb } from "@/components/Public/MockBook";

describe("MockBook", () => {
  it("正常_title_date_worldLabel_を含む", () => {
    const html = renderToStaticMarkup(
      <MockBook
        title="テストタイトル"
        date="2026.04.24"
        worldLabel="Test World"
      />,
    );
    expect(html).toContain('data-testid="mock-book"');
    expect(html).toContain("テストタイトル");
    expect(html).toContain("2026.04.24");
    expect(html).toContain("Test World");
  });

  it("正常_左cover58_と_右page48_2x2grid_の design 正典構造を持つ", () => {
    const html = renderToStaticMarkup(<MockBook title="Title" />);
    // 左 cover: width 58% / 非対称 radius 4px-14px / teal-50 → surface gradient
    expect(html).toContain("w-[58%]");
    expect(html).toContain("rounded-l-[4px]");
    expect(html).toContain("rounded-r-[14px]");
    expect(html).toContain("from-teal-50");
    // 右 page: width 48% / 2x2 grid / 非対称 radius 14px-4px
    expect(html).toContain("w-[48%]");
    expect(html).toContain("grid-cols-2");
    expect(html).toContain("grid-rows-2");
    expect(html).toContain("rounded-l-[14px]");
    expect(html).toContain("rounded-r-[4px]");
    // 右 page の 3 cell すべて aria-hidden（装飾）+ top span 2
    expect(html).toContain("col-span-2");
    expect(html.match(/aria-hidden="true"/g)?.length ?? 0).toBeGreaterThanOrEqual(3);
  });

  it("正常_旧rotate装飾_は削除されている", () => {
    const html = renderToStaticMarkup(<MockBook title="Title" />);
    // design 正典には存在しない rotate 装飾は出さない
    expect(html).not.toContain("rotate-6");
    expect(html).not.toContain("-rotate-3");
  });

  it("正常_date_worldLabel_省略時は表示しない", () => {
    const html = renderToStaticMarkup(<MockBook title="Title" />);
    expect(html).toContain("Title");
    expect(html).not.toMatch(/2026\.\d{2}\.\d{2}/);
  });
});

describe("MockThumb", () => {
  it("正常_variants_a_b_c_d_e_全て_data-testid_付与", () => {
    const variants = ["a", "b", "c", "d", "e"] as const;
    for (const v of variants) {
      const html = renderToStaticMarkup(<MockThumb variant={v} />);
      expect(html).toContain(`data-testid="mock-thumb-${v}"`);
      // image 未指定時は装飾 (aria-hidden)
      expect(html).toContain('aria-hidden="true"');
    }
  });

  it("正常_aspect-square_default_と_sm:aspect-[4/3]_PC_を持つ", () => {
    const html = renderToStaticMarkup(<MockThumb variant="a" />);
    // mobile = 1:1 (`wf-screens-a.jsx:62-64`), PC = 4:3 (`:141-143`)
    expect(html).toContain("aspect-square");
    expect(html).toContain("sm:aspect-[4/3]");
    expect(html).toContain("rounded-sm");
  });

  it("正常_image指定時は_picture_webp_jpg_alt_を出し_aria-hiddenを付けない", () => {
    const html = renderToStaticMarkup(
      <MockThumb
        variant="a"
        image={{
          slug: "sample-01",
          alt: "テスト alt",
          width: 640,
          height: 1138,
        }}
      />,
    );
    // β-2c: image 指定時は <picture> + WebP source + JPEG fallback img
    expect(html).toContain("<picture>");
    expect(html).toContain('srcSet="/img/landing/sample-01.webp"');
    expect(html).toContain('src="/img/landing/sample-01.jpg"');
    expect(html).toContain('alt="テスト alt"');
    expect(html).toContain('width="640"');
    expect(html).toContain('height="1138"');
    expect(html).toContain('data-testid="mock-thumb-a"');
    // image 指定時は wrapper span に aria-hidden を付けない (写真は意味あり)
    expect(html).not.toMatch(/data-testid="mock-thumb-a"[^>]*aria-hidden/);
  });
});

describe("MockBook image props (β-2c)", () => {
  it("正常_cover指定時_背景picture_と_dark_overlay_と_white_text_が出る", () => {
    const html = renderToStaticMarkup(
      <MockBook
        title="タイトル"
        cover={{ slug: "mock-cover", alt: "cover alt", width: 720, height: 1280 }}
      />,
    );
    expect(html).toContain('srcSet="/img/landing/mock-cover.webp"');
    expect(html).toContain('src="/img/landing/mock-cover.jpg"');
    expect(html).toContain('alt="cover alt"');
    // dark gradient overlay (title 可読性)
    expect(html).toContain("from-black/70");
    // title が white text に切替
    expect(html).toContain("text-white");
  });

  it("正常_spreadTop_spreadBottomLeft_spreadBottomRight_3画像が出る", () => {
    const html = renderToStaticMarkup(
      <MockBook
        title="t"
        spreadTop={{ slug: "hero", alt: "hero alt", width: 1600, height: 900 }}
        spreadBottomLeft={{ slug: "sample-04", alt: "", width: 640, height: 1138 }}
        spreadBottomRight={{ slug: "sample-01", alt: "", width: 640, height: 1138 }}
      />,
    );
    expect(html).toContain('srcSet="/img/landing/hero.webp"');
    expect(html).toContain('src="/img/landing/hero.jpg"');
    expect(html).toContain('alt="hero alt"');
    expect(html).toContain('srcSet="/img/landing/sample-04.webp"');
    expect(html).toContain('srcSet="/img/landing/sample-01.webp"');
    // top span 2 cell
    expect(html).toContain("col-span-2");
  });

  it("正常_image_未指定時は_β-2a_と同等の_gradient_placeholder_に_fallback", () => {
    const html = renderToStaticMarkup(<MockBook title="t" />);
    // 写真 picture が出ない
    expect(html).not.toContain("<picture>");
    expect(html).not.toContain("/img/landing/");
    // 既存 placeholder gradient が維持される
    expect(html).toContain("from-teal-50");
    // 装飾 aria-hidden cell が右 page にある
    expect(html.match(/aria-hidden="true"/g)?.length ?? 0).toBeGreaterThanOrEqual(3);
  });

  it("正常_ε-fix_LandingImage_の_objectPosition_が_img_style_に反映される", () => {
    const html = renderToStaticMarkup(
      <MockBook
        title="t"
        cover={{
          slug: "mock-cover",
          alt: "cover alt",
          width: 720,
          height: 1280,
          objectPosition: "center 30%",
        }}
        spreadTop={{
          slug: "hero",
          alt: "hero alt",
          width: 1600,
          height: 900,
          objectPosition: "center 40%",
        }}
        spreadBottomLeft={{
          slug: "sample-04",
          alt: "",
          width: 640,
          height: 1138,
          objectPosition: "center 32%",
        }}
        spreadBottomRight={{
          slug: "sample-01",
          alt: "",
          width: 640,
          height: 1138,
        }}
      />,
    );
    // cover / spreadTop / spreadBottomLeft の各 image に object-position style が反映
    expect(html).toMatch(/object-position\s*:\s*center\s+30%/);
    expect(html).toMatch(/object-position\s*:\s*center\s+40%/);
    expect(html).toMatch(/object-position\s*:\s*center\s+32%/);
  });
});

describe("MockThumb image props (ε-fix)", () => {
  it("正常_objectPosition_が_img_style_に反映される", () => {
    const html = renderToStaticMarkup(
      <MockThumb
        variant="a"
        image={{
          slug: "sample-01",
          alt: "alt",
          width: 640,
          height: 1138,
          objectPosition: "center 28%",
        }}
      />,
    );
    expect(html).toMatch(/object-position\s*:\s*center\s+28%/);
  });

  it("正常_objectPosition_未指定時は_object-position_style_を出さない", () => {
    const html = renderToStaticMarkup(
      <MockThumb
        variant="a"
        image={{
          slug: "sample-01",
          alt: "alt",
          width: 640,
          height: 1138,
        }}
      />,
    );
    expect(html).not.toMatch(/object-position\s*:/);
  });
});
