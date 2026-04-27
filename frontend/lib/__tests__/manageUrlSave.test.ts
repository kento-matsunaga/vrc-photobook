// manageUrlSave のユニットテスト（Node 環境、DOM 不要部分のみ）。
//
// セキュリティ:
//   - テスト内で実 raw token / 実管理 URL は使わない（固定 dummy URL）。
//   - filename / mailto href の文字列に余計な内部情報が含まれないことを検証する。
import { describe, expect, it } from "vitest";

import {
  buildMailtoHref,
  buildManageUrlTxtContent,
  buildManageUrlTxtFileName,
  sanitizeSlug,
} from "../manageUrlSave";

const dummyManageURL = "https://app.vrc-photobook.com/manage/token/aaaaaaaaaaaaaaaa";

describe("buildManageUrlTxtContent", () => {
  const tests = [
    {
      name: "正常_管理URLが本文に含まれる",
      description: "Given: dummy URL, When: build text, Then: URL が含まれる",
      url: dummyManageURL,
      wantContains: dummyManageURL,
      wantNotContains: ["photobook_id", "storage_key", "token_version"],
    },
    {
      name: "正常_余計な内部識別子は含めない",
      description: "Given: 管理URL, When: build, Then: photobook_id 等の用語が出ない",
      url: dummyManageURL,
      wantContains: "管理用 URL",
      wantNotContains: ["photobook_id", "draft_edit_token", "session_token", "Set-Cookie"],
    },
  ];
  for (const tt of tests) {
    it(tt.name, () => {
      const got = buildManageUrlTxtContent(tt.url);
      expect(got).toContain(tt.wantContains);
      for (const banned of tt.wantNotContains) {
        expect(got).not.toContain(banned);
      }
    });
  }
});

describe("sanitizeSlug", () => {
  const tests: { name: string; description: string; input: string; want: string }[] = [
    {
      name: "正常_a-z0-9-のみ通過",
      description: "Given: 安全文字のみ, When: sanitize, Then: そのまま返る",
      input: "ab12cd34ef56",
      want: "ab12cd34ef56",
    },
    {
      name: "正常_大文字は小文字化",
      description: "Given: 大文字混在, When: sanitize, Then: 小文字化",
      input: "AbCdEf",
      want: "abcdef",
    },
    {
      name: "異常_path_traversal_文字は除去",
      description: "Given: '../../etc/passwd', When: sanitize, Then: 危険文字除去",
      input: "../../etc/passwd",
      want: "etcpasswd",
    },
    {
      name: "異常_空白_/_:_+_/_等は除去",
      description: "Given: スペース・コロン等, When: sanitize, Then: 全部消える",
      input: " a b:c+d/e?f#g",
      want: "abcdefg",
    },
    {
      name: "異常_長すぎる_24文字でtruncate",
      description: "Given: 50文字, When: sanitize, Then: 24文字で打ち切り",
      input: "a".repeat(50),
      want: "a".repeat(24),
    },
    {
      name: "異常_全部弾かれる場合は空文字",
      description: "Given: 日本語のみ（ASCII含まず）, When: sanitize, Then: 空文字",
      input: "管理",
      want: "",
    },
    {
      name: "正常_日本語+ASCIIはASCII部のみ通過",
      description: "Given: '管理URL', When: sanitize, Then: 'url'（大文字小文字化＋日本語除去）",
      input: "管理URL",
      want: "url",
    },
  ];
  for (const tt of tests) {
    it(tt.name, () => {
      expect(sanitizeSlug(tt.input)).toBe(tt.want);
    });
  }
});

describe("buildManageUrlTxtFileName", () => {
  const tests: {
    name: string;
    description: string;
    input: string | undefined;
    want: string;
  }[] = [
    {
      name: "正常_slug付きファイル名",
      description: "Given: 安全な slug, When: build filename, Then: vrc-photobook-manage-url-<slug>.txt",
      input: "ab12cd34ef56",
      want: "vrc-photobook-manage-url-ab12cd34ef56.txt",
    },
    {
      name: "正常_slug_undefinedはdefaultに",
      description: "Given: undefined, When: build filename, Then: default",
      input: undefined,
      want: "vrc-photobook-manage-url.txt",
    },
    {
      name: "正常_危険文字含むslugはsanitize済が使われる",
      description: "Given: '../etc/x', When: build, Then: 危険文字除去後の名前",
      input: "../etc/x",
      want: "vrc-photobook-manage-url-etcx.txt",
    },
    {
      name: "異常_ASCII_を含まない日本語のみslugはdefault",
      description: "Given: '管理'（ASCIIなし）, When: build, Then: default",
      input: "管理",
      want: "vrc-photobook-manage-url.txt",
    },
    {
      name: "正常_空文字はdefault",
      description: "Given: '', When: build, Then: default",
      input: "",
      want: "vrc-photobook-manage-url.txt",
    },
  ];
  for (const tt of tests) {
    it(tt.name, () => {
      expect(buildManageUrlTxtFileName(tt.input)).toBe(tt.want);
    });
  }
});

describe("buildMailtoHref", () => {
  it("正常_subject_body_が encodeURIComponent され、URL がそのまま含まれる", () => {
    const got = buildMailtoHref(dummyManageURL);
    // mailto: prefix
    expect(got.startsWith("mailto:?")).toBe(true);
    // subject / body が encode 済
    expect(got).toContain("subject=");
    expect(got).toContain("body=");
    // URL は body の中に含まれる
    const decodedBody = decodeURIComponent(got.split("body=")[1] ?? "");
    expect(decodedBody).toContain(dummyManageURL);
    // 余計な内部情報が出ない
    expect(decodedBody).not.toContain("photobook_id");
    expect(decodedBody).not.toContain("draft_edit_token");
    expect(decodedBody).not.toContain("session_token");
  });

  it("異常_改行_+_等の特殊文字を含むURLでも安全に encode される", () => {
    // ハイポセティカル: 想定外文字を含む URL でもクラッシュしない
    const tricky = "https://app.example.com/manage/token/abc 123+456&x=y\n";
    const got = buildMailtoHref(tricky);
    expect(got.startsWith("mailto:?")).toBe(true);
    const decodedBody = decodeURIComponent(got.split("body=")[1] ?? "");
    expect(decodedBody).toContain(tricky);
  });
});
