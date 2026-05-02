// upload-verification session token を queue 全体で 1 度だけ取得し再利用するキャッシュ。
//
// 設計意図（Issue A hotfix の根幹）:
//   - PrepareClient で concurrency=2 並列 upload した際、両 tile が React state ベースで
//     `verificationToken === ""` を観測 → `issueUploadVerification` を 2 回呼んでしまい、
//     2 つ目が Cloudflare Turnstile token 単回使用制約で 403 verification_failed を返す
//     race condition を解消する。
//   - React state は async 更新で並列読み取り間の整合性が保証されない。代わりに
//     `useRef` で同期に「in-flight Promise」と「取得済 token」を共有する。
//   - 本モジュールは pure な factory として実装し、副作用（fetch / setState）を呼び出し側に
//     委ねる。これにより vitest=node 環境でも race / sequential / retry を直接 verify 可能。
//
// セキュリティ:
//   - upload_verification_token / Turnstile token を log / console / docs に出さない
//   - 本モジュールは値を内部で保持するのみ、外部公開 API は `current` の getter のみ

/** 1 トークンを取得する関数（lib/upload.issueUploadVerification 等を注入）。 */
export type IssueVerificationFn = (
  turnstileToken: string,
) => Promise<{ uploadVerificationToken: string }>;

export type UploadVerificationCache = {
  /** 取得済 token を返す。未取得なら空文字。 */
  readonly current: string;
  /**
   * upload_verification_token を保証する。
   *
   * - 取得済なら即座にその値を返す
   * - 取得中（in-flight Promise あり）ならその Promise を await（重複実行回避）
   * - 未取得 + 非 in-flight なら issueVerificationFn を呼んで取得
   * - 失敗時は Promise / token を破棄。次回呼び出しで再試行可能（ただし turnstileToken 自体は
   *   single-use のため呼び出し側で再取得が必要）
   */
  ensure(turnstileToken: string): Promise<string>;
  /** 明示破棄（例: verification_failed や rate_limited 後の cleanup）。 */
  reset(): void;
};

/** UploadVerificationCache を組み立てる factory。 */
export function createUploadVerificationCache(
  issue: IssueVerificationFn,
): UploadVerificationCache {
  let token = "";
  let inflight: Promise<string> | null = null;

  return {
    get current(): string {
      return token;
    },
    async ensure(turnstileToken: string): Promise<string> {
      // 既に取得済 → 共有
      if (token !== "") {
        return token;
      }
      // 既に in-flight → 同 Promise を await（重複 fetch 回避）
      if (inflight !== null) {
        return await inflight;
      }
      // 未取得 + 非 in-flight → 新規 Promise を作成し sync に inflight に格納
      // （以降の同期呼び出しは inflight ブランチに合流する）
      inflight = (async () => {
        try {
          const out = await issue(turnstileToken);
          token = out.uploadVerificationToken;
          return token;
        } catch (e) {
          // 失敗 → 後続呼び出しで再試行できるよう inflight / token を破棄
          inflight = null;
          token = "";
          throw e;
        }
      })();
      return await inflight;
    },
    reset(): void {
      token = "";
      inflight = null;
    },
  };
}
