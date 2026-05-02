# concurrency=2 で Turnstile upload-verification が race する

## 発生日

PR22 frontend-upload-ui Issue A hotfix 実装時（2026-04-27 前後）。
本 failure-log は 2026-05-03 STOP α harness 強化で正典化。

## 症状

`/prepare` で複数画像を選んで concurrency=2 で並列 upload 開始すると、

1. tile-1 の `runUpload` が `issueUploadVerification(tok)` を呼ぶ
2. tile-2 の `runUpload` が **同じ tok で** 並行に `issueUploadVerification(tok)` を呼ぶ
3. Cloudflare Turnstile siteverify は token を **single-use** で扱うため 2 回目が 403
4. tile-2 が `verification_failed` で fail tile になる

事故クラス: **single-use token を並列で消費する race condition**。

## 根本原因

`PrepareClient.runUpload` が tile ごとに独立して `issueUploadVerification` を呼んでおり、queue 全体で 1 回しか得られないはずの upload_verification_token を 2 件以上 issue しようとしていた。

## 修正

`frontend/lib/uploadVerificationCache.ts`（Issue A hotfix 実装時）:

```ts
export type UploadVerificationCache = {
  ensure(turnstileToken: string): Promise<UploadVerification>;
  reset(): void;
};
```

- `ensure()` 内に in-flight Promise を保持。並列 caller には同じ Promise を返す。
- 取得済 token がある間は再 issue しない。
- 失敗時は inflight + token を破棄して次回再試行可能。
- `reset()` で turnstile expired / error 時に明示クリア。

PrepareClient は `verificationCacheRef.current.ensure(tok)` 経由で取得することで、queue 全体で 1 回の issueUploadVerification で全 tile が同じ token を共有する。

## 追加した test

`frontend/lib/__tests__/uploadVerificationCache.test.ts`:
- 「正常_並列ensure_2件で issue は 1 回だけ呼ばれる_両方が同 token を得る」
- 「正常_3件並列 ensure でも issue は 1 回だけ」
- 「正常_取得済 token がある状態で ensure すると issue は呼ばれない」
- 「正常_失敗時は inflight / token が破棄され、次の ensure で再試行可能」
- 「正常_失敗中の並列 ensure はどちらも同じ rejection を受ける」
- 「正常_reset 後の ensure は issue を再度呼ぶ」
- 「正常_最初の ensure 呼び出しで渡された turnstileToken のみ issue に渡る」

## 今後の検知方法

- `createUploadVerificationCache` の実装変更で並列 issue 抑制が壊れたら 1 ケース目で即落ちる。
- `.agents/rules/turnstile-defensive-guard.md` L1〜L4 多層防御も併用。

## 残る follow-up

- 別 Turnstile-protected endpoint（report 通報、create 作成等）でも同種の single-use race が発生していないか横並び確認
- Backend 側で同 token の二重 siteverify を観測したらメトリクスで検知できるか検討

## 関連

- `.agents/rules/turnstile-defensive-guard.md` L0/L1/L2/L3/L4
- `harness/failure-log/2026-04-29_turnstile-widget-remount-loop.md`
- `harness/failure-log/2026-04-29_report-form-turnstile-bypass.md`
- `frontend/lib/uploadVerificationCache.ts`
