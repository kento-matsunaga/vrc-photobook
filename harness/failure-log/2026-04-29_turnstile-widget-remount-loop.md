# 2026-04-29 PR35b STOP ε2 で Turnstile widget が iPhone Safari 上で remount loop し、視認上は無限ループしつつ実は 1 回 submit が成立した

## 発生状況

- **何をしようとしていたか**: PR35b STOP ε2（Backend revision 00020 + Workers `5d09172b` 反映後の Safari 実機 Report 送信 smoke）として、対象 photobook を一時 unhide し iPhone Safari から `/p/<対象>/report` で 1 件だけ submit する正常系 smoke。
- **どのファイル/モジュールで発生したか**:
  - `frontend/components/TurnstileWidget.tsx`（useEffect 依存配列）
  - `frontend/components/Report/ReportForm.tsx`（inline arrow callback の渡し方）

## 失敗内容

- ユーザーが iPhone Safari で実機 smoke 中、Turnstile widget の verification 完了前段階で **「ロボットではありません」チェック → ロード → 再度チェックボックス → ロード**を繰り返す **無限ループ的挙動**を観測。
- ユーザーは「submit はまだ成功させていない」と認識して中断。
- しかし DB 確認で **reports row 1 件が submitted 状態で作成されていた**（prefix `019dd8a5...`、reason=harassment_or_doxxing、status=submitted、has_detail/contact/ip_hash all true、ip_hash_byte_len=32）。
- Cloud Run access logs では POST `/api/public/photobooks/<slug>/reports` が **2026-04-29T09:49:45.859Z に 1 回だけ** → 201 Created（重複 submit / 4xx 無し、Backend 側は完全正常）。
- ユーザー視点の「無限ループ」と Backend 視点の「1 回 submit 成立」が両立。

## 根本原因

`frontend/components/TurnstileWidget.tsx` の useEffect 依存配列に **inline arrow function（`onError` / `onExpired`）が含まれていた**。

```tsx
}, [scriptLoaded, sitekey, action, onVerify, onError, onExpired]);
```

ReportForm 側は:

```tsx
onError={() => setTurnstileToken("")}      // 毎 render で新参照
onExpired={() => setTurnstileToken("")}    // 毎 render で新参照
```

メカニズム:

1. ユーザーが detail / contact / reason / formState 変化で ReportForm が re-render
2. `onError` / `onExpired` の inline arrow 参照が変わる → useEffect が依存変化と判定
3. useEffect cleanup で `turnstile.remove(widgetId)` → 再 render で `turnstile.render(...)` 再実行
4. widget は「ロード → チェックボックス → ロード」を再開（ユーザー視点では無限ループ）
5. ユーザーが入力を止めた瞬間に verification が完了する瞬間がある → `callback(token)` 発火 → `setTurnstileToken(token)` → submit ボタン enable
6. ユーザーが押下 → submit 成立 → 201

これは **L1-L4 多層 Turnstile ガードの欠陥ではなく、widget 安定性の問題**。L4 trim ガードは正常に動作（whitespace token なし、submit は valid token で 201）。

PR22 frontend-upload-ui の TurnstileWidget も同じ inline arrow パターンだが、Upload UI は state 変化頻度が低い（caption / reorder は子コンポーネント化されている）ため、目立った再 mount loop は発生していなかった。**横展開対象**だが、本対策（TurnstileWidget 内部の useRef 化）で Upload にも自動で恩恵が及ぶため、追加修正は不要。

## 影響範囲

- **本番データへの影響**: 想定外の reports row 1 件 + outbox `report.submitted` pending 1 件作成。raw 値は work-log / commit / chat（公開記録）に出さず、本セッション内 cleanup TX で signature 一致 1 件のみを安全に DELETE 済（target reports count: 1 → 0、outbox `report.submitted` 全体: 1 → 0）。photobook hidden=true は維持、unhide / hide で発生した outbox `photobook.unhidden` + `photobook.hidden` の pending 2 件は worker `--once --max-events 1` を 2 回実行で no-op processed 化済（pending(available)=0）。
- **設計への影響**: 「Turnstile L1-L4 多層ガード」は機能していたが、**widget 自体が安定 mount されない場合のユーザー体験は loop に見える**ため、ガードの前提が崩れる。今後 Turnstile を使う form は **TurnstileWidget の内部 useRef pattern** + **親側 useCallback での callback 安定化**を必須化する。
- **harness 上の影響**: 「親 re-render が widget 寿命を直撃する」典型的な React hooks 落とし穴を捕捉できなかった。実機 smoke（特に iPhone Safari の入力イベント頻度の高い環境）でしか露呈しない。

## 対策種別

- [x] ルール化（`.agents/rules/turnstile-defensive-guard.md` に「widget 安定 mount」セクション追記候補。本失敗を契機に検討）
- [ ] スキル化
- [x] テスト追加（既存 SSR テスト全通過確認、jsdom + RTL は本 PR で導入せず後続）
- [ ] フック追加

## 取った対策

### 1. データ後始末

- cleanup TX で signature 一致 1 件の reports + outbox を DELETE（前回 STOP ε NG cleanup と同じ TX 構造）
- worker `--once --max-events 1` 2 回で photobook.unhidden + photobook.hidden pending を no-op processed 化
- photobook は最初から最後まで `hidden_by_operator=true` を維持

### 2. Frontend 修正

- `frontend/components/TurnstileWidget.tsx`: callback prop を `useRef` で保持し、useEffect 依存配列を `[scriptLoaded, sitekey, action]` のみに変更。`error-callback` / `timeout-callback` / `expired-callback` 実装、token は出さず error code を `console.warn`。
- `frontend/components/Report/ReportForm.tsx`: `useCallback` で `handleTurnstileVerify` / `handleTurnstileError` / `handleTurnstileExpired` / `handleTurnstileTimeout` を安定化（防御的、TurnstileWidget 内部の useRef で吸収済みだが二重 belt）。

### 3. テスト

- 既存 vitest 112 tests 全通過確認
- TurnstileWidget の jsdom + window.turnstile mock テストは React Testing Library 未導入のため本 PR では追加せず、後続候補（PR35c / PR40 ローンチ前チェック等）

### 4. 横展開

- Upload UI（`frontend/app/(draft)/edit/[photobookId]/EditClient.tsx`）も同じ TurnstileWidget を使うため、本修正で自動的に恩恵を受ける（追加コード変更不要）
- `.agents/rules/turnstile-defensive-guard.md` に「widget 安定 mount ルール」セクションを追記（L1-L4 + L0=widget mount stability の構造化を検討、後続）

## 関連

- `.agents/rules/turnstile-defensive-guard.md`（L1-L4 多層ガードルール、本失敗で「widget 安定 mount」観点を追加検討）
- `harness/failure-log/2026-04-29_report-form-turnstile-bypass.md`（PR35b 内で先に検出した別の Turnstile 系失敗）
- `harness/failure-log/2026-04-29_public-photobook-route-unregistered-after-report-guard-deploy.md`（同日に検出した Cloud Run deploy 直後 transient）
- `docs/adr/0005-turnstile-action-binding.md`（Turnstile action 厳密一致 + 必須化の基盤）

## 履歴

| 日付 | 変更 |
|------|------|
| 2026-04-29 | 初版作成。PR35b STOP ε2 NG（Turnstile widget remount loop）の経緯を記録。cleanup + Frontend 修正 + ルール化検討 |
