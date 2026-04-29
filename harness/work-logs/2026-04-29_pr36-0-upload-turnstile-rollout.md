# PR36-0 upload-verification への Turnstile 多層ガード横展開（2026-04-29、完了）

## 本書のスコープ

PR35b で確立した L0-L4 多層 Turnstile ガード（`.agents/rules/turnstile-defensive-guard.md`）を、
既存 upload-verification 経路に **コードレベルで横展開**した小 PR の進行記録。

本 PR は PR36（UsageLimit 集約）本体に入る前段の前駆 PR として位置付ける。
upload-verification の本番動作（draft session + 実画像 + R2 PUT + image processing まで含む）
の Safari 実機 smoke は本 PR に含めず、後続 PR / 運用フェーズで回収する方針。

## 概要

- Backend `internal/uploadverification/interface/http/handler.go` の L4 ガードを
  `req.TurnstileToken == ""` から `strings.TrimSpace(...) == ""` に強化、trim 後 token を
  UseCase へ渡す
- Backend `internal/uploadverification/internal/usecase/issue_upload_verification_session.go`
  の Execute 冒頭にも L4 ガード（`strings.TrimSpace(in.TurnstileToken) == ""` →
  `ErrUploadVerificationFailed` 早期 return）を追加し、Cloudflare siteverify に到達させない
- Frontend `app/(draft)/edit/[photobookId]/EditClient.tsx` を L0+L1+L2 強化:
  - L1: `isTurnstileReady = trim() !== ""` 判定追加、submit ボタン disable を `!isTurnstileReady`
  - L2: `startUpload` 冒頭で `!isTurnstileReady` early return
  - L0 二重 belt: `handleTurnstileVerify/Error/Expired/Timeout` を `useCallback` 安定化、
    `onTimeout` prop も追加（PR35b の TurnstileWidget 内部 useRef pattern との二重 belt）
- Frontend `lib/upload.ts` の L3 ガードは PR22 段階で実装済 → 本 PR で whitespace バリエーション
  4 種（tab / newline / CRLF / 全角空白）を `lib/__tests__/upload.test.ts` で固定化
- Backend test に whitespace ケース 3〜4 種を追加（handler / UseCase 双方）。
  `FakeTurnstile.VerifyFn` で `called` フラグを立て、Cloudflare siteverify が呼ばれない
  ことをテストレベルで保証

## 実施した STOP

| Phase | 結果 | 備考 |
|---|---|---|
| commit + push（`540cd1f fix(upload): apply turnstile defensive guards`）| OK | author: kento-matsunaga 単独、Co-Authored-By なし |
| **STOP A**（Backend Cloud Build deploy 前停止 → 承認）| OK | image `vrcpb-api:540cd1f` build SUCCESS（3m25s）、revision `vrcpb-api-00021-vl9` traffic 100% |
| Cloud Build smoke（runbook §1.4.1 / §1.4.2 準拠、deploy 後 9 分待機後）| OK | `/health` `/readyz` 200、`/api/public/photobooks/<bad-slug>` handler JSON 404、`<対象 slug>` 410 gone、OGP fallback、Workers regression なし |
| Cloud Run Job `vrcpb-outbox-worker` image 更新 | OK | image `vrcpb-api:540cd1f` 反映、command / args / cloudsql annotation / secretKeyRef 維持、`REPORT_IP_HASH_SALT_V1` Job 注入なし、Job 実行なし、Cloud Scheduler 0 件 |
| **STOP B**（Workers redeploy 前停止 → 承認）| OK | wrangler deploy SUCCESS、新 version `ce64f95a-d4ce-405b-821a-f71c22a992db` active 100%、OGP_BUCKET / ASSETS binding 維持 |
| ヘッドレス smoke | OK | `/` `/og/default.png` `/manage/<dummy>` `/p/<対象>/report`（hidden gone）すべて regression なし |
| EditClient chunk 反映確認 | OK | `.trim()` 3 hits / `useCallback` 14 hits / `verification_failed` 5 hits（識別子は minify で短縮、想定内）|
| Cloud Build / Cloud Run / Workers Secret 漏洩 grep | 0 件 | 用語 hit のみ、実値なし |

## 変更 commit

- `540cd1f fix(upload): apply turnstile defensive guards`（Backend / Frontend / failure-log / roadmap）
- `<本 PR closeout commit>`: `docs(work-log): finalize PR36-0 turnstile upload guard rollout`

## デプロイ後の本番状態（最終）

- Backend image: `vrcpb-api:540cd1f` / Cloud Run revision `vrcpb-api-00021-vl9` / traffic 100%
- Cloud Run Job `vrcpb-outbox-worker` image: `vrcpb-api:540cd1f`
- Workers Frontend version: `ce64f95a-d4ce-405b-821a-f71c22a992db` / active 100%
  - rollback 候補: `6da0447b-...`（PR35b STOP δ3、Upload UI L0 二重 belt 未適用）
- Cloud SQL migration: v17（変更なし）
- Secret Manager: 既存 8 件（`REPORT_IP_HASH_SALT_V1` 含む、変更なし）
- target photobook: `hidden_by_operator=true` 維持（PR35b 完了状態を維持）
- reports: PR35b 監査チェーン記録 1 件（status=resolved_action_taken）維持
- outbox pending(available): 0

## Safari 実機 smoke を実施しなかった理由

- 本 PR の主目的は「PR35b で得た Turnstile 防御知見の横展開」
- Backend L4 / UseCase guard / Frontend L1-L3 / lib/upload.ts L3 ガードは **テストで固定済み**
- Upload 実機 smoke は draft session Cookie + 実画像 + R2 PUT + image processing まで絡み重い
- PR35b で **同じ TurnstileWidget は iPhone Safari 実機 smoke 成功済み**（widget 安定 mount 検証済）
- 本 PR で追加された差分は EditClient.tsx の L1+L2 + L0 二重 belt のみ。これらは
  ロジックの組み合わせが純粋（trim 判定 / useCallback 安定化）でテスト可能領域に収まる
- ユーザー判断「方針 A: 実機 smoke 不要で closeout」を採用

## 後続回収事項

- **iPhone Safari 実機 Upload smoke**（draft session 入場 → Turnstile → 実画像 upload →
  R2 PUT → image processing 確認）→ 後続 PR / 運用フェーズで回収
- 本項目は `docs/plan/vrc-photobook-final-roadmap.md` §1.3 に「実機 Safari smoke による
  Upload 画面 widget loop 再発確認 → 後続 PR / 運用フェーズで確認」として記録済

## PR closeout

- [x] コメント整合チェック実施（`scripts/check-stale-comments.sh` 実行、既存 hit はすべて C 区分の過去経緯）
- [x] 古いコメント修正（該当なし、新規追加コメントは状態ベース + ルール参照のみ）
- [x] 残した TODO とその理由: Upload 実機 Safari smoke（roadmap §1.3 + 本 work-log §後続回収）
- [x] 先送り事項記録先: `docs/plan/vrc-photobook-final-roadmap.md` §1.3 + `harness/failure-log/2026-04-29_report-form-turnstile-bypass.md` §「PR36-0 横展開完了記録」
- [x] generated file: 本 PR で sqlcgen / .open-next 等の差分なし（.open-next は git ignore）
- [x] Secret grep: 実値漏洩 0 件

## Secret 漏洩がないこと

- DATABASE_URL / R2_SECRET_ACCESS_KEY / TURNSTILE_SECRET_KEY / REPORT_IP_HASH_SALT_V1 / token /
  Cookie / manage URL / storage_key 完全値: いずれも本 PR の commit / work-log / chat 公開記録に
  未含有
- Cloud Build logs（229 行）/ Cloud Run service logs（16 行、新 revision）/ Workers chunk /
  Worker response いずれも実値 0 件
- 用語 hit（`.env.example` の placeholder / `--update-secrets=KEY=KEY:latest` Secret reference /
  grep pattern 文字列）は許容範囲

## 関連

- `.agents/rules/turnstile-defensive-guard.md`（L0〜L4 多層ガードの正典）
- `harness/failure-log/2026-04-29_report-form-turnstile-bypass.md` §「PR36-0 横展開完了記録」
- `harness/failure-log/2026-04-29_turnstile-widget-remount-loop.md`（PR35b L0 widget 再 mount 事案）
- `harness/work-logs/2026-04-29_report-result.md`（PR35b 完了 work-log）
- `docs/plan/vrc-photobook-final-roadmap.md` §1.3（後続 PR 横展開項目を完了マーク済）
- `docs/runbook/backend-deploy.md`（§1.4.1 / §1.4.2 で 5〜10 分待機 + handler JSON smoke を必須化済）

## 履歴

| 日付 | 変更 |
|------|------|
| 2026-04-29 | 初版（PR36-0 完了）。STOP A / STOP B 完了 + Backend / Frontend deploy + ヘッドレス smoke + closeout を集約 |
