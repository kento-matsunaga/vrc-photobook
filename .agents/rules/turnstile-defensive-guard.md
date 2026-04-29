# Turnstile 多層防御ガードルール

## 適用範囲

Cloudflare Turnstile を使用する **すべての public form / Backend endpoint** に適用する。
2026-04-29 時点の対象は以下:

- `frontend/components/Upload/*`（PR22）+ `frontend/lib/upload.ts` + Backend `internal/uploadverification/*`
- `frontend/components/Report/ReportForm.tsx`（PR35b）+ `frontend/lib/report.ts` + Backend `internal/report/*`
- 今後追加される **任意の public form**（問い合わせ / 再発行依頼 / 公開コメント等）に必ず適用

## 原則

> **Turnstile widget の verification 完了前に submit が成立する経路を、Frontend / Backend のいずれの層でも残さない。**

ADR-0005（Turnstile action 厳密一致 / Turnstile 必須）の前提条件は、**Frontend だけ**でも **Backend だけ**でもなく、L1〜L4 の 4 層すべてで重複ガードを噛ませて初めて担保される。1 層でも欠けると、widget bug / Browser 拡張 / 中断挙動 / 古い state 残存等で短時間 token を non-empty にできる経路ができ、submit を素通りさせるリスクが残る。

## 必須パターン（L0〜L4）

### L0: TurnstileWidget の安定 mount（Frontend Component 内部）

Turnstile widget は親 component の re-render で **`turnstile.remove()` → `turnstile.render()` の cycle が走らないよう** stable に保つ。これを怠ると、ユーザー視点では「ロボットではありません → ロード → 再度チェックボックス → ロード」の **無限ループ的挙動**になり、verification 完了の瞬間に意図せず submit が成立する事故が起きる（`harness/failure-log/2026-04-29_turnstile-widget-remount-loop.md`）。

```tsx
// ✅ 正しい: callback prop は useRef で保持し、useEffect 依存に入れない
const onVerifyRef = useRef(onVerify);
const onErrorRef = useRef(onError);
const onExpiredRef = useRef(onExpired);
const onTimeoutRef = useRef(onTimeout);
useEffect(() => {
  onVerifyRef.current = onVerify;
  onErrorRef.current = onError;
  onExpiredRef.current = onExpired;
  onTimeoutRef.current = onTimeout;
}, [onVerify, onError, onExpired, onTimeout]);

useEffect(() => {
  // ... turnstile.render({ callback: t => onVerifyRef.current(t), ... })
  // 依存配列は [scriptLoaded, sitekey, action] のみ
}, [scriptLoaded, sitekey, action]);

// ❌ 禁止: callback prop を useEffect 依存に入れる
}, [scriptLoaded, sitekey, action, onVerify, onError, onExpired]);
//                                  ^^^^^^^^^^^^^^^^^^^^^^^^^^^ 親 re-render で widget が remove → re-render
```

加えて、Form Component 側でも `useCallback` で callback を安定化する（防御的、widget 内部 useRef との二重 belt）。`error-callback` / `timeout-callback` / `expired-callback` を実装し、token は出さずに error code を `console.warn` で見える化する。

## 必須パターン（L1〜L4）

### L1: 送信ボタン disable 判定（Frontend Form Component）

送信ボタンの `disabled` / `aria-disabled` 判定で、Turnstile token を **空白 trim 後の非空** で判定する。

```tsx
// ✅ 正しい
const canSubmit =
  typeof turnstileToken === "string" &&
  turnstileToken.trim() !== "" &&
  formState !== "submitting";

// ❌ 禁止（PR35b STOP ε NG の原因）
const canSubmit = turnstileToken !== "" && formState !== "submitting";
//                                ^^^^ whitespace token / 一時的な non-empty 残存を素通り
```

加えて、Turnstile widget の `data-callback` / `onSuccess` で token をセットする際、widget 中断・error 系の `data-error-callback` / `onError` で **必ず token を空文字に戻す**。

### L2: onSubmit ハンドラ冒頭の再評価ガード（Frontend Form Component）

button disable があっても、JavaScript からの強制発火 / race condition / `<form>` の Enter 送信を考慮し、`onSubmit` の冒頭で **再度** trim 後 non-empty を確認する。

```tsx
// ✅ 正しい
const onSubmit = async (e: FormEvent) => {
  e.preventDefault();
  if (typeof turnstileToken !== "string" || turnstileToken.trim() === "") {
    setError("turnstile_failed");
    return;
  }
  // ... submit 処理
};
```

### L3: API client lib の defensive guard（Frontend lib）

Form Component を経由しない呼び出し（テスト / 別経路 / 将来の widget 取り換え）でも安全側に倒すため、API client 関数（`submitReport` / `issueUploadVerification` 等）の冒頭で同条件を確認し、**Backend へ送信する前に reject** する。

```ts
// ✅ 正しい（lib/report.ts / lib/upload.ts 共通パターン）
export async function submitReport(args: SubmitReportArgs): Promise<void> {
  if (typeof args.turnstileToken !== "string" || args.turnstileToken.trim() === "") {
    throw { kind: "turnstile_failed" } satisfies SubmitReportError;
  }
  // ... fetch
}
```

### L4: Backend endpoint の trim 後 empty 拒否（Backend Handler / UseCase）

HTTP handler の body decode 直後と UseCase の Execute 入口の **両方** で、`strings.TrimSpace(token) == ""` を確認し、**Cloudflare siteverify 呼び出し前** に 403（turnstile_failed 相当）で early return する。

```go
// ✅ 正しい
if strings.TrimSpace(req.TurnstileToken) == "" {
    writeError(w, http.StatusForbidden, "turnstile_failed")
    return
}

// ❌ 禁止（PR35b STOP ε NG の原因）
if req.TurnstileToken == "" {
    // whitespace は素通りして siteverify に丸投げになる
}
```

UseCase 側でも同条件を独立に確認すること（単体テスト容易化 + 将来的に handler 経由以外から呼ぶ場合の保険）。

## 必須テスト

各層に対して以下のテストを追加する（テーブル駆動 + Builder + description 必須、`.agents/rules/testing.md` 準拠）。

| 層 | テスト内容 | 場所 |
|---|---|---|
| L1 | disabled 判定が「空文字」「空白のみ」「null/undefined」で true（送信不可）になることを SSR HTML / DOM 上で確認 | `frontend/components/.../*.test.tsx` |
| L2 | `turnstileToken=" "` で onSubmit を呼んでも fetch が呼ばれないこと | 同上 |
| L3 | `submitReport({ turnstileToken: "" })` / `" \t\n"` が `turnstile_failed` で reject、`fetch` が呼ばれないこと | `frontend/lib/__tests__/*.test.ts` |
| L4 | `req.TurnstileToken=" "` を handler / UseCase に渡すと 403 / `ErrTurnstileFailed`、Cloudflare siteverify は呼ばれないこと | `backend/internal/.../*_test.go` |

## 禁止事項

1. **L1 のみ / L4 のみ** で完結させる（多層防御の原則違反）
2. Turnstile token を **空文字判定だけ** で済ませる（trim 必須）
3. Turnstile widget の error / timeout callback で token を維持する実装
4. 単体テスト無しで L1〜L4 を実装する
5. Turnstile を使う新規 form を、本ルールを参照せずに作る

## チェックリスト（PR レビューで使う）

新規に Turnstile を使う form / endpoint を追加した PR、もしくは既存 Turnstile 経路を改修した PR では、以下を完了報告に含める。

- [ ] L0: TurnstileWidget 内部で callback prop を `useRef` 保持し、useEffect 依存配列に入れていない（親 re-render で widget 再 mount しない）
- [ ] L0: error-callback / timeout-callback / expired-callback を実装し、token は出さず error code を `console.warn`
- [ ] L0: Form Component 側で `useCallback` を使い callback を安定化（防御的）
- [ ] L1: 送信ボタン disable 条件に `trim() !== ""` あり
- [ ] L1: widget `error` / `expired` callback で token を空文字に戻している
- [ ] L2: onSubmit 冒頭に再評価 early return あり
- [ ] L3: API client lib に defensive guard early reject あり
- [ ] L4: Backend handler / UseCase に `strings.TrimSpace(token) == ""` early return あり
- [ ] 各層に whitespace-only / 空文字 / 正常の **3 ケース最低**のユニットテストあり
- [ ] 既存 Turnstile 経路（upload / report / 他）も同等であることを横並び確認
- [ ] Safari 実機で「widget スピナー中に送信ボタンを押下しても submit されない」ことを確認（`.agents/rules/safari-verification.md` の Turnstile 項に従う）

## Why（なぜこのルールが必要か）

PR22（frontend-upload-ui）で L1〜L4 の多層防御は実装されていたが、**実装ノウハウが work-log にしか残らず、`.agents/rules/` に正典化されていなかった**。

その結果、PR35b（Report 公開通報窓口）で `ReportForm` を新設した際に L1 の trim ガードが落ち、L3 の defensive guard が抜け、L4 が空文字判定のみになり、本番 STOP ε で **Turnstile widget verification 未完了のまま submit が成立**する事故が発生した（`harness/failure-log/2026-04-29_report-form-turnstile-bypass.md`）。

ADR-0005 の「Turnstile 必須」を運用上担保するには、**個別 PR の実装者が PR22 の work-log を読みに行くことに依存するのではなく、ルール化して PR 完了処理（`.agents/rules/pr-closeout.md`）と PR レビューで強制チェック**する仕組みが必要である。

## 関連

- `docs/adr/0005-turnstile-action-binding.md` — Turnstile action 厳密一致 + 必須化の基盤
- `.agents/rules/safari-verification.md` — Safari 実機確認（widget 完了前 submit 不可の確認項目）
- `.agents/rules/security-guard.md` — Secret / token / 認可全般
- `.agents/rules/testing.md` — テーブル駆動 + Builder + description
- `.agents/rules/feedback-loop.md` — 失敗 → ルール化 → テスト の運用原則
- `.agents/rules/pr-closeout.md` — PR 完了処理（本ルールのチェックリストを必須化）
- `harness/failure-log/2026-04-29_report-form-turnstile-bypass.md` — 本ルールを起点にした失敗事例
- `harness/work-logs/2026-04-27_frontend-upload-ui-result.md` — PR22 で L1〜L4 が初回実装された記録

## 履歴

| 日付 | 変更 |
|------|------|
| 2026-04-29 | 初版作成。PR35b STOP ε NG（Turnstile bypass）を契機に、PR22 で実装済の L1〜L4 多層ガードを正典ルール化 |
| 2026-04-29 | PR35b STOP ε2 NG（widget remount loop）を契機に L0「TurnstileWidget 安定 mount」セクションを追加。`harness/failure-log/2026-04-29_turnstile-widget-remount-loop.md` |
