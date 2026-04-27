# 2026-04-27 Frontend upload UI 最小骨格 + Turnstile widget + Safari 実機確認 実施結果（PR22）

## 概要

`docs/plan/m2-frontend-upload-ui-plan.md` PR22 計画に基づき、Frontend に画像アップロード UI
の最小骨格 + Turnstile widget を実装し、Backend に upload-verifications endpoint を追加。
macOS Safari / iPhone Safari で実機確認し、**Bot 検証ゲート → upload-intent → R2 PUT →
complete → processing 表示**までを本番相当環境で完走させた。

これにより M2 「実ブラウザから画像アップロードできる MVP」まで到達。

- 実施日時: 2026-04-27 03:30〜04:35 JST（Safari 確認 1 回目 + 修正 + 2 回目）
- Backend revision: `vrcpb-api-00006-wdg` / image `vrcpb-api:8928be8`
- Frontend Workers Version: `6860b721-4ddb-456d-9f2d-be7f9d62bbe7`
- Cloud SQL: `vrcpb-api-verify`（残置継続）

## 前提

- PR12〜PR17: 独自ドメイン + HttpOnly Cookie session + Safari 確認 済
- PR18 Image aggregate / PR19 Photobook ↔ Image / PR20 Upload Verification foundation
- Audit: Security / Domain Integrity Audit
- PR21: upload-intent → R2 PUT → complete → processing（curl 経路で確認済）
- R2 bucket `vrcpb-images` / Turnstile widget は PR21 Step B で作成済
- Cloud Run env に R2_* / TURNSTILE_SECRET_KEY 注入済

## 実施した実装

### Backend

- `backend/internal/uploadverification/interface/http/handler.go`（新規）:
  POST `/api/photobooks/{id}/upload-verifications`
- `backend/internal/uploadverification/wireup/wireup.go`（新規）
- `backend/internal/http/router.go`（更新）: upload-verifications route 追加
- `backend/internal/config/config.go`（更新）: TURNSTILE_SECRET_KEY / TURNSTILE_HOSTNAME /
  TURNSTILE_ACTION env 追加
- `backend/cmd/api/main.go`（更新）: CloudflareVerifier 起動時組立 + uvHandlers 統合
- `handler_test.go`（新規）: 6 ケース（success / Turnstile failure / Cloudflare 障害 /
  draft session 不在 / photobook 不一致 / **空 turnstile_token は 400** / **欠落も 400**）

### Frontend

- `frontend/lib/upload.ts`（新規）: 4 つの API client wrapper（issueUploadVerification /
  issueUploadIntent / putToR2 / completeUpload）+ validateFile + sourceFormatOf
- `frontend/components/TurnstileWidget.tsx`（新規）: 自前 React component で Cloudflare
  公式 script を動的読み込み、`window.turnstile.render` を呼ぶ
- `frontend/app/(draft)/edit/[photobookId]/UploadClient.tsx`（新規）: Client Component
  で upload UI を実装
- `frontend/app/(draft)/edit/[photobookId]/page.tsx`（更新）: Server Component で
  photobook_id と sitekey を解決して UploadClient に渡す
- `frontend/.env.production.example`（更新）: NEXT_PUBLIC_TURNSTILE_SITE_KEY 追記
- `frontend/.env.production`（gitignore 維持、ユーザー対話で実値追加）
- `frontend/lib/__tests__/upload.test.ts`（新規）: 22 ケース

### Workers redeploy

- `npx opennextjs-cloudflare deploy` 経由（PR14 から wrangler が delegate するように変更）
- Custom Domain `app.vrc-photobook.com` 維持
- middleware (34.1 kB) 維持

## Safari 実機確認 1 回目（2026-04-27 03:30 JST 頃、修正前）

### 結果

- macOS / iPhone Safari で `/edit/<id>` 表示 ✅
- ファイル選択 ✅
- Turnstile widget 表示 ✅
- アップロード完走（upload-verifications 201 / upload-intent 201 / R2 PUT 200 / complete
  200 / processing 表示）✅
- **Turnstile gate が UI 上は「検証中」表示のまま、アップロードボタンを押せた** ❌

ユーザー指摘: 「Turnstile が検証状態のまま、その状態でもアップロード開始でき、アップロード
済みになりました」→ Bot ガードとして UI 上不十分。

## 修正（commit `8928be8`）

### 多層 Bot ガード強化

- **L1 (UI disabled)**: アップロードボタン disabled 条件を `!tokenReady`
  (`turnstileToken.trim() === ""` を弾く) に強化
- **L2 (function guard)**: `startUpload` 関数の冒頭で `isTurnstileVerified(token)` を再評価
  して空文字列 / 空白を弾く defensive ガード
- **L3 (API client)**: `lib/upload.ts` の `issueUploadVerification` 冒頭で空 token を
  fetch せず即 `{ kind: "verification_failed" }` を throw
- **L4 (Backend)**: handler が空 / 欠落 turnstile_token を 400 で拒否（既存 + test 固定）

### UI 改善

- Turnstile widget 直下に **「Bot 検証 未完了（widget の challenge を完了してください）」**
  または **「Bot 検証成功 ✓ アップロード可能」** バッジ（緑色 / グレー切替）
- ボタン下に「※ Bot 検証が完了するまでアップロードできません。」のヒント文
- `data-testid` 付与で test 容易性向上
- `handleTurnstileError` で `setTurnstileToken(null)` を併走（widget エラー時に確実にクリア）

### 追加 test

- Backend: 空 / 欠落 turnstile_token 各 400
- Frontend: 空文字 / 空白だけの token は fetch されず reject

## Safari 実機確認 2 回目（2026-04-27 04:30 JST 頃、修正後）

### 結果（ユーザー報告）

- Turnstile Managed mode の検証完了まで少し時間がかかる（仕様）
- **Turnstile 未完了状態ではアップロードできないことを確認** ✅
- **Turnstile 完了後にアップロードボタンが有効化されることを確認** ✅
- アップロード実行後、processing 表示まで到達 ✅
- 動作上の問題なし ✅

### Backend access log（実機確認分）

| timestamp | method / path | status |
|---|---|---|
| 07:32:13 | OPTIONS upload-verifications | 200 (preflight) |
| 07:32:13 | POST upload-verifications | 201 |
| 07:32:14 | OPTIONS upload-intent | 200 |
| 07:32:14 | POST upload-intent | 201 |
| 07:32:15 | OPTIONS complete | 200 |
| 07:32:15 | POST complete | 200 |
| 07:32:37 | POST upload-verifications | 201 (2 枚目) |
| 07:32:38 | POST upload-intent | 201 |
| 07:32:39 | POST complete | 200 |

Turnstile 検証の Cloudflare siteverify が機能（fail-closed）し、preflight CORS / Backend
authn / R2 PUT / complete のフルサイクルが Safari で成立。

### `safari-verification.md` 必須項目チェック

- [x] /edit/<id> redirect 着地 + Cookie 発行 / Domain=.vrc-photobook.com / HttpOnly /
      Secure / SameSite=Strict（PR16 / PR17 と継続）
- [x] Turnstile widget 表示 + challenge 完了 + token 発行
- [x] file picker（macOS Safari / iPhone Safari）動作
- [x] HEIC accept（iPhone Safari の写真選択で確認可能）
- [x] upload flow 全段階で HTTP 2xx
- [x] processing 表示
- [x] reload 後も Cookie 維持
- Private Browsing は任意項目、未実施（次のサイクルで継続観察）

### 24 時間 / 7 日後の Cookie 残存（継続観察）

`.agents/rules/safari-verification.md` 「継続観察項目」として記録。本 PR 完了判定外。

## R2 cleanup

PR16 / PR17 / PR21 / PR22 の test photobook を一括対象に、
`backend/internal/photobook/_tokengen/cleanup --all-pr-test` を実行:

```
Found 8 test photobooks
  019dcdd7-...:  deleted 2 objects   ← PR22 Safari 2 回目
  019dcdc0-...:  deleted 2 objects   ← PR22 Safari 1 回目
  019dcb4c-...:  deleted 0 objects   ← PR21 Step E (cleanup 済)
  019dcb4b-...:  deleted 0 objects   ← PR16 (cleanup 済)
  019dcace-...:  deleted 0 objects   ← PR17 (cleanup 済)
  ...
```

PR22 で R2 に置かれた **計 4 個の dummy 画像**を削除完了。

## Backend logs 漏洩 grep

```
gcloud run services logs read vrcpb-api --region=asia-northeast1 --limit=300 |
  grep -iE "(SECRET|API_KEY|PASSWORD|PRIVATE|sk_live|sk_test|draft_edit_token|
            manage_url_token|session_token|set-cookie|DATABASE_URL=|
            R2_SECRET_ACCESS_KEY|R2_ACCESS_KEY_ID|TURNSTILE_SECRET_KEY|AKIA|
            presigned|X-Amz-Signature|amz-signature|turnstile_token)"
```

→ ヒット: `GET /private/.env` 404（攻撃 bot による典型 scan、Backend は 404 で拒否、漏洩なし）

実 Secret / token / Cookie / presigned URL / R2 credentials の漏洩は **0 件** ✅

## 一時コード・一時ファイル削除確認

| 対象 | 結果 |
|---|---|
| `backend/internal/photobook/_tokengen/`（main.go + cleanup/）| 削除済 ✅ |
| `backend/_tokengen` / `backend/cleanup` バイナリ | 削除済 ✅ |
| `/tmp/vrcpb-safari-urls.txt`（token URL）| 削除済 ✅ |
| `/tmp/vrcpb-pr22-start.epoch` | 削除済 ✅ |
| `/tmp/vrcpb-tokengen.err` | 削除済 ✅ |
| Cloud SQL Auth Proxy プロセス | 停止済 ✅ |
| 環境変数 `R2_*` / `DB_PASSWORD` | unset 済 ✅ |
| git status | `?? .wrangler/`（wrangler 一時ディレクトリ、別 PR で gitignore 化）|

## Step D.5 の慣例化

PR21 で発見した「Backend code 変更時は image rebuild + push + revision update が必要」を
PR22 でも同パターン適用:
- 1 回目 deploy: commit `0ad32a4` → vrcpb-api-00005-698
- 2 回目 deploy（修正後）: commit `8928be8` → vrcpb-api-00006-wdg

これで「`gcloud run services update --update-secrets=...` だけ + image rebuild なし」を
回避できた（PR21 Step E で発見した古い image 問題）。

## 実施しなかったこと

- image-processor 本体（HEIC 変換 / EXIF 除去 / variant 生成 / available 化）
- 公開 Viewer での画像表示
- OGP 生成 / moderation UI / Outbox / SendGrid
- 編集 UI のフル機能（caption 編集 / page reorder / cover 設定 / publish）
- design system の正式抽出
- drag & drop
- progress bar (XHR upload)
- Safari Private Browsing 自動 test
- 24 時間 / 7 日後 Cookie 残存の自動観察（手動継続観察項目）
- `.wrangler/` の gitignore 化（別 PR）
- Cloud SQL 削除
- 既存 spike Cloud Run / R2 削除
- Public repo 化

## 次のステップ

PR23: image-processor（HEIC → JPG/WebP 変換、EXIF 除去、display / thumbnail variant 生成、
processing → available 遷移）

PR23 着手時:
- DB に `processing` のままの image レコードが 4 件残存（PR22 Safari 2 回 × 2 枚 = 4 件）
- これらは image-processor から見ると「実 R2 object なし」なので、`MarkFailed(object_not_found)`
  になる想定（PR21 と同じロジック）
- PR23 計画書で test 用 photobook の cleanup 戦略を整理

PR22 完了後の Cloud SQL 残置/削除判断:
- 推奨: **残置継続**（PR23 着手も連続予定）
- 次回判断タイミング: PR23 計画書完了時 or 2 日後

## 関連

- [PR22 計画書](../../docs/plan/m2-frontend-upload-ui-plan.md)
- [PR21 R2 + presigned URL 結果](./2026-04-27_r2-presigned-url-real-upload-result.md)
- [PR20 Upload Verification 計画](../../docs/plan/m2-upload-verification-plan.md)
- [Security / Domain Integrity Audit](../../docs/security/public-repo-checklist.md)
- [`.agents/rules/safari-verification.md`](../../.agents/rules/safari-verification.md)
- [`.agents/rules/security-guard.md`](../../.agents/rules/security-guard.md)

## 観察メモ

> Turnstile Managed mode の検証完了まで少し時間がかかるが、未完了状態ではアップロード不可。
> 検証完了後のみアップロード可能となり、R2 PUT → complete → processing まで成立。
