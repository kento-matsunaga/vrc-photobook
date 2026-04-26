# 2026-04-27 R2 presigned URL 実 upload 確認 実施結果（PR21 Step E）

## 概要

`docs/plan/m2-r2-presigned-url-plan.md` PR21 Step A〜E 計画に基づき、
**実 R2 bucket への presigned URL 経由 PUT + complete API での processing 遷移**
を本番相当環境で確認した。

これにより M2 「画像アップロードの土台」が独自ドメイン上で成立した。

- 実施日時: 2026-04-27 04:36〜04:39 JST（約 3 分）
- Cloud Run revision: `vrcpb-api-00004-b8k`
- image: `asia-northeast1-docker.pkg.dev/.../vrcpb/vrcpb-api:579b027`（PR21 Step A HEAD）
- R2 bucket: `vrcpb-images`（PR21 Step B で作成、本実装専用）
- Cloud SQL: `vrcpb-api-verify`（migration v4 → v11 に更新）

## 前提

- PR12〜PR17: 独自ドメイン + token → HttpOnly Cookie session 済み
- PR18 Image aggregate / PR19 Photobook ↔ Image / PR20 Upload Verification: 完了
- PR21 Step A: Backend code with fake R2 を実装、commit `579b027`
- Step B: ユーザー手動で R2 bucket / API token / CORS / Turnstile widget 作成
- Step C: Secret Manager に R2_* v2 + TURNSTILE_SECRET_KEY 登録、spike service は `R2_*:1` で隔離
- Step D: Cloud Run vrcpb-api に env 注入（revision 00003-c2s）
- **Step D.5（Audit 追加）**: Backend image rebuild + push + revision 更新（00004-b8k）

## Step D.5 が必要になった理由

PR12 で deploy した `vrcpb-api:500f8cc` image は PR18〜PR21 Step A の Backend code を
**含まない**。Cloud Run の `--update-secrets` は env 注入だけで image は変わらないため、
古い main.go が R2 関連設定を読まず、imageupload endpoint が登録されない状態だった。

判明経路: dummy POST upload-intent で 404 が返り、access log にも upload-intent endpoint
の登録が無いことを確認 → image tag が `:500f8cc`（PR12 時点）であることを発見。

対処: Step D.5 として image rebuild + push + revision 更新を計画書に追記し、
HEAD commit `579b027` の image でデプロイ。起動 log に
`r2 configured; image upload endpoints enabled` を確認、dummy POST が 401 を返すように
変わったことで endpoint 登録を確認した。

PR21 計画書 §13.1 / §13.2 に Step D.5 として正式追記済（本 PR の commit に同梱）。

## Cloud SQL migration の更新

Cloud SQL `vrcpb-api-verify` は PR12 deploy 直後（migration v4）のままで、PR18 の images /
PR19 の photobook_pages 等 / PR20 の upload_verification_sessions が未適用だった。

Step E 着手時に tokengen 実行で `relation "upload_verification_sessions" does not exist
(SQLSTATE 42P01)` を検出 → goose up を Cloud SQL Auth Proxy 経由で実行し、
v4 → v11 に更新（00005〜00011 の 7 本適用、合計約 0.4 秒）。

## 実施手順

### Step E-1. 一時 tokengen 作成（コミット禁止、cleanup 済）

- 配置: `backend/internal/photobook/_tokengen/main.go`
- `backend/internal/photobook/_tokengen/cleanup/main.go`（R2 DeleteObject 用）
- 内容:
  1. `CreateDraftPhotobook` UseCase で draft photobook + draft raw token 発行
  2. `upload_verification_sessions` に直接 `Repository.Create` で session INSERT、raw token を返却
  3. cleanup tool は引数 `<storage_key>` で R2 DeleteObject
- stdout に `PHOTOBOOK_ID=<uuid> / DRAFT=<43chars> / UV=<43chars>` の 3 行
- 値はチャット / 作業ログに残さない、length のみ確認（draft 43 / uv 43）

### Step E-2. /draft/<token> 経由 Cookie 取得

- `https://app.vrc-photobook.com/draft/<draft_token>` に curl
- 302 → /edit/<photobook_id> redirect が成立、HttpOnly Cookie が cookie jar に保存される
- Cookie name: `vrcpb_draft_<photobook_id>` / Domain=.vrc-photobook.com / HttpOnly / Secure / SameSite=Strict（PR16 / PR17 で確認済）

### Step E-3. POST upload-intent

- `POST https://api.vrc-photobook.com/api/photobooks/<pid>/images/upload-intent`
- Cookie: vrcpb_draft_<pid>
- Authorization: Bearer <upload_verification_token>
- Body: `{"content_type":"image/jpeg","declared_byte_size":1024,"source_format":"jpg"}`

結果:

| 項目 | 結果 |
|---|---|
| HTTP/2 | **201** ✅ |
| Cache-Control | `no-store` ✅ |
| Content-Type | `application/json` ✅ |
| Vary | `Origin`（CORS middleware 動作）✅ |
| response keys | `expires_at` / `image_id` / `required_headers` / `storage_key` / `upload_url` ✅ |
| image_id | length 36（UUID）✅ |
| upload_url | 520 chars / `r2.cloudflarestorage.com` ドメイン含む / **AKIA / Bearer / Secret 等の credential 文字列なし** ✅ |
| required_headers keys | `Content-Length` / `Content-Type` / `Host` ✅ |
| storage_key prefix | `photobooks/<pid>/images/<iid>/original/` 一致 ✅ |
| storage_key length | 121 chars（拡張子 .jpg 含む）✅ |
| expires_at | upload-intent 時刻 + 15 分 ✅ |

### Step E-4. presigned URL に curl PUT

- `curl -X PUT --upload-file dummy.bin -H "Content-Type: image/jpeg" <upload_url>`
- dummy.bin: JPEG magic header (`FF D8 FF E0`) + 0 padding 1020 byte = 1024 byte 計

結果:

| 項目 | 結果 |
|---|---|
| HTTP | **200** ✅ |
| body_size | 1024 byte（宣言と一致）✅ |
| ETag | `"bf46ace9508ca377ca61b8e7b4356c4a"` 取得 ✅ |
| Server | `cloudflare`（R2 経由）✅ |

aws-sdk-go-v2 の Content-Length 署名仕様（M1 PoC で実証済）が本実装でも成立。
`SignatureDoesNotMatch` 系のエラーなし。

### Step E-5. POST complete

- `POST https://api.vrc-photobook.com/api/photobooks/<pid>/images/<image_id>/complete`
- Cookie: vrcpb_draft_<pid>
- Body: `{"storage_key":"<storage_key>"}`

結果:

| 項目 | 結果 |
|---|---|
| HTTP/2 | **200** ✅ |
| response | `{"image_id":"<uuid>","status":"processing"}` ✅ |
| image_id | 一致 ✅ |
| status | `processing` ✅（image-processor は PR23、PR21 段階では processing どまりが正） |

Backend ログには:
- `POST /api/photobooks/.../images/upload-intent` 201
- `POST /api/photobooks/.../images/.../complete` 200

がそれぞれ記録された。

### Step E-6. R2 object 削除

- 一時 cleanup tool に Secret Manager 経由で R2 credentials を export → DeleteObject 実行
- env は実行直後に unset
- DELETED 確認

### Step E-7. Backend logs 漏洩 grep

```sh
gcloud run services logs read vrcpb-api --region=asia-northeast1 --limit=200 |
  grep -iE "(SECRET|API_KEY|PASSWORD|PRIVATE|sk_live|sk_test|draft_edit_token|
            manage_url_token|session_token|set-cookie|DATABASE_URL=|
            R2_SECRET_ACCESS_KEY|R2_ACCESS_KEY_ID|TURNSTILE_SECRET_KEY|AKIA|
            presigned|X-Amz-Signature|amz-signature)"
```

→ **NO_MATCH** ✅

## 実施しなかったこと

- Frontend Turnstile widget 表示
- image-processor 本体（HEIC 変換 / EXIF 除去 / variant 生成 / available 化）
- Outbox events
- SendGrid
- moderation UI
- Safari / iPhone Safari 実機検証（widget なし、PR22 で実施）
- Cloud SQL `vrcpb-api-verify` 削除
- 既存 spike Cloud Run / spike R2 bucket 削除
- raw token / Cookie 値 / presigned URL / R2 credentials の本書・チャット・コミットへの記録
- debug endpoint / dummy token 成功経路の追加

## 一時コード・一時ファイル削除確認

| 対象 | 結果 |
|---|---|
| `backend/internal/photobook/_tokengen/`（main.go + cleanup/）| 削除済 ✅ |
| `backend/_tokengen` / `backend/cleanup`（go build 副産物）| 削除済 ✅ |
| `/tmp/vrcpb-stepe-*`（cookie jar / response / dummy ファイル）| 削除済 ✅ |
| `/tmp/tmp.*`（DRAFT / UV / PID 一時ファイル）| 削除済 ✅ |
| Cloud SQL Auth Proxy プロセス | 停止済 ✅ |
| 環境変数 `R2_*` / `DB_PASSWORD` | unset 済 ✅ |
| git status | `M docs/plan/m2-r2-presigned-url-plan.md`（Step D.5 追記、commit 待ち）|

## 切戻し手順（参考、本書では実施しない）

### 旧 image への rollback

```sh
gcloud run services update vrcpb-api --region=asia-northeast1 \
  --project=project-1c310480-335c-4365-8a8 \
  --image=asia-northeast1-docker.pkg.dev/.../vrcpb/vrcpb-api:500f8cc
```

PR12 image (`:500f8cc`) は Artifact Registry に残存中。

### Secret rollback

R2_* v2 を disable して spike pin と同じ pattern で v1 に戻すことは **実用上不要**
（v1 は spike 値、本実装値 v2 が正）。万一 v2 が誤値だった場合のみ:

```sh
gcloud secrets versions disable 2 --secret=R2_ACCESS_KEY_ID --project=...
# その後 vrcpb-api を再 update して v3 を作る
```

## 費用

- Cloud Run revision 更新: 無料枠
- R2 PUT × 1 + DeleteObject × 1: 数 byte / Class A 操作 2 回（無料枠 1M/月 内）
- R2 storage: 1024 byte が約 30 秒間（無料枠 10GB 内）
- Cloud SQL 累計（PR17 完了から本書まで）: ~9 時間 / ~¥21

## 次のステップ

PR21 Step F: 計画書追記 + 本作業ログ commit / push

その後の roadmap:
- PR22: Frontend upload UI 最小骨格 + Turnstile widget 表示 + Safari / iPhone Safari 実機確認
- PR23: image-processor（HEIC 変換 / EXIF 除去 / variant 生成 / available 化）
- PR24: OGP / Moderation / 公開ページ整備

PR21 完了後の Cloud SQL 残置/削除判定:
- 推奨: **残置継続**（PR22 計画着手も連続予定）
- 次回判断タイミング: PR22 計画書完了時 or 2 日後

## 関連

- [PR21 計画書](../../docs/plan/m2-r2-presigned-url-plan.md)
- [PR20 Upload Verification 計画 / 実装結果](../../docs/plan/m2-upload-verification-plan.md)
- [PR18 Image aggregate 計画 / 実装結果](../../docs/plan/m2-image-upload-plan.md)
- [PR19 Photobook ↔ Image 連携](../../docs/plan/m2-photobook-image-connection-plan.md)
- [Security / Domain Integrity Audit](./2026-04-27_post-deploy-final-roadmap.md)
- [PR16 実 token 結合確認結果](./2026-04-27_frontend-backend-real-token-e2e-result.md)
- [PR12 Backend Domain Mapping 実施結果](./2026-04-27_backend-domain-mapping-result.md)
- [`docs/security/public-repo-checklist.md`](../../docs/security/public-repo-checklist.md)
- [`.agents/rules/security-guard.md`](../../.agents/rules/security-guard.md)
