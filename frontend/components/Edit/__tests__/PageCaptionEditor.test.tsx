// PageCaptionEditor SSR / structural test。
//
// vitest + react-dom/server で markup を検証する (DOM testing library 非導入の方針)。
// blur 保存 / 状態遷移は behavior level では確認できないため、SSR で属性 / 値を確認する。

import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";

import { PageCaptionEditor } from "@/components/Edit/PageCaptionEditor";

const noop = async () => undefined;

describe("PageCaptionEditor SSR markup", () => {
  it("正常_initialValue を value に持つ input を描画", () => {
    const html = renderToStaticMarkup(
      <PageCaptionEditor initialValue="hello" onSave={noop} />,
    );
    // input + data-testid + value
    expect(html).toContain('data-testid="page-caption-editor"');
    expect(html).toContain('value="hello"');
    // aria-label
    expect(html).toContain('aria-label="page caption"');
  });

  it("正常_disabled で input が disabled になる", () => {
    const html = renderToStaticMarkup(
      <PageCaptionEditor initialValue="" disabled onSave={noop} />,
    );
    const inputMatch = html.match(/<input[^>]*data-testid="page-caption-editor"[^>]*>/);
    expect(inputMatch?.[0] ?? "").toMatch(/disabled=""/);
  });

  it("正常_placeholder と max 文字数 hint", () => {
    const html = renderToStaticMarkup(
      <PageCaptionEditor initialValue="" onSave={noop} />,
    );
    expect(html).toContain("ページのタイトル");
    expect(html).toContain("/ 200");
  });

  it("正常_runeCount を表示_2byte 文字も 1 文字としてカウント", () => {
    // 「あいう」= 3 rune
    const html = renderToStaticMarkup(
      <PageCaptionEditor initialValue="あいう" onSave={noop} />,
    );
    expect(html).toContain("3 / 200");
  });

  it("正常_idle 状態は「変更なし」を表示", () => {
    const html = renderToStaticMarkup(
      <PageCaptionEditor initialValue="x" onSave={noop} />,
    );
    expect(html).toContain("変更なし");
  });
});
