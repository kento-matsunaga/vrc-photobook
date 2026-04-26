// PR10.5 Route Handler テスト用ヘルパ。
//
// セキュリティ:
//   - 固定 43 文字 token を repo に書かない。テスト内で動的に生成する
//   - 生成した raw token は test ログに出さない（assert に raw を直接埋め込まない）

/**
 * 43 文字の base64url 風文字列をテスト用に生成する。
 *
 * ただし「実トークンとして DB に通る」必要はないため、文字種だけが正規（base64url）の
 * ダミー文字列で十分。fetch mock を介して扱うので Backend に届かない。
 */
export function fakeToken43(seed: string): string {
  // base64url 文字集合: A-Z a-z 0-9 - _
  const alphabet =
    "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_";
  let h = 0;
  for (let i = 0; i < seed.length; i++) {
    h = (h * 31 + seed.charCodeAt(i)) | 0;
  }
  let out = "";
  let state = h >>> 0;
  for (let i = 0; i < 43; i++) {
    state = (state * 1103515245 + 12345) >>> 0;
    out += alphabet[state % alphabet.length];
  }
  return out;
}
