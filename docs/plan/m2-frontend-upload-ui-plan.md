# M2 Frontend upload UI 最小骨格 + Turnstile widget 実装計画（PR22 候補）

> 作成日: 2026-04-27
> 位置付け: PR21（R2 + presigned URL）完了後、Frontend に画像アップロード UI を最小限
> 実装し、Turnstile widget + Safari / iPhone Safari 実機確認まで行うフェーズの入口。
> 実装コードはまだ書かない。
>
> 上流参照（必読）:
> - [`docs/plan/m2-r2-presigned-url-plan.md`](./m2-r2-presigned-url-plan.md)
> - [`harness/work-logs/2026-04-27_r2-presigned-url-real-upload-result.md`](../../harness/work-logs/2026-04-27_r2-presigned-url-real-upload-result.md)
> - [`docs/plan/m2-upload-verification-plan.md`](./m2-upload-verification-plan.md)
> - [`docs/plan/m2-image-upload-plan.md`](./m2-image-upload-plan.md)
> - [`docs/plan/m2-photobook-image-connection-plan.md`](./m2-photobook-image-connection-plan.md)
> - [`docs/adr/0005-image-upload-flow.md`](../adr/0005-image-upload-flow.md)
> - [`design/mockups/prototype/screens-a.jsx`](../../design/mockups/prototype/screens-a.jsx) (Edit / Photo grid)
> - [`design/mockups/prototype/pc-screens-a.jsx`](../../design/mockups/prototype/pc-screens-a.jsx)
> - [`design/mockups/prototype/styles.css`](../../design/mockups/prototype/styles.css) (tokens)
> - [`frontend/app/(draft)/edit/[photobookId]/page.tsx`](../../frontend/app/\(draft\)/edit/\[photobookId\]/page.tsx) (現 PR10 placeholder)
> - [`frontend/lib/api.ts`](../../frontend/lib/api.ts)
> - [`.agents/rules/safari-verification.md`](../../.agents/rules/safari-verification.md)
> - [`.agents/rules/security-guard.md`](../../.agents/rules/security-guard.md)

---

## 0. 本計画書の使い方

- 設計の正典は `docs/adr/0005-image-upload-flow.md` + `design/mockups/prototype/`。
  本書はそれを **PR22 として**「最小骨格まで」**にどう絞るか**を整理する。
- 実装範囲を**意図的に狭く**保ち、Frontend と Backend を 1 サイクルで整合させる。
- §15 のユーザー判断事項に答えてもらってから PR22 実装に着手する。

---

## 1. 目的

- `/edit/<photobookId>` ページに画像アップロード UI の **最小骨格**を実装する。
- Turnstile widget を表示し、Frontend が token を取得できるようにする。
- Backend に `POST /api/photobooks/{id}/upload-verifications` を追加（Turnstile 検証 → upload verification token 発行）。
- Frontend が upload-intent → R2 PUT → complete を実行し、UI に **status: processing** を表示する。
- macOS Safari / iPhone Safari で実機確認し、`safari-verification.md` の必須項目を満たす。
- **available 化 / variant 生成 / 公開 Viewer 表示は PR23 / PR24** に切り分ける（processing 止まり）。

---

## 2. PR22 の対象範囲

### 対象（PR22 で実装する）

- **Backend**:
  - `POST /api/photobooks/{id}/upload-verifications` endpoint 追加（draft session middleware 経由）
  - `IssueUploadVerificationSession` UseCase + `Cloudflare Turnstile siteverify` 実 client（PR20 既実装、wireup を追加）
  - HTTP handler + router 統合
  - tests (handler + UseCase で fake / real verifier 両方)
- **Frontend**:
  - Turnstile widget React component（公式 script 動的読み込み）
  - upload UI 最小骨格（ファイル選択 + Turnstile + アップロード進捗 + processing 表示）
  - upload flow client (`/edit/[photobookId]/page.tsx` を Server → Client 構造に拡張)
  - API client wrapper: upload-verifications / upload-intent / R2 PUT / complete
  - error / progress 表示
- **環境**:
  - `frontend/.env.production` に `NEXT_PUBLIC_TURNSTILE_SITE_KEY` 追加
  - Workers redeploy（`vrcpb-frontend`）
- **検証**:
  - macOS Safari / iPhone Safari 実機確認（Cookie / Turnstile / file picker / R2 PUT / processing）
  - Backend logs 漏洩 grep

### 対象外（PR22 では実装しない）

- image-processor 本体（HEIC 変換 / EXIF 除去 / variant 生成 / available 化）
- `display` / `thumbnail` variant の表示
- 公開 Viewer での画像表示
- OGP 生成
- moderation UI
- Outbox events
- SendGrid
- iPhone Safari Private Browsing の長期 ITP 確認（24h / 7d 観察は別途、運用フェーズ）
- design system component の正式抽出（PR22 では prototype を**参照のみ**、import しない）
- 編集 UI のフル機能（caption 編集 / page reorder / cover 設定 / publish ボタン 等は PR23 以降）
- Cloud SQL / spike / Cloudflare Dashboard 操作（既存設定のまま）

---

## 3. 重要な決定: upload-verifications endpoint を PR22 で追加

PR20 では public endpoint を作っていない。Step E では tokengen で DB 直接 INSERT したが、
Frontend は本物の HTTP 経路でしか token を取れない。

**選択肢 A（推奨）: PR22 で `POST /api/photobooks/{id}/upload-verifications` を追加**

```
POST /api/photobooks/{id}/upload-verifications
  Cookie: vrcpb_draft_<id>
  Body: { "turnstile_token": "<widget が返した response token>" }
  Response 201:
    {
      "upload_verification_token": "<base64url 43>",
      "expires_at": "...",
      "allowed_intent_count": 20
    }
```

**Backend**:
- 既存 PR20 `IssueUploadVerificationSession` UseCase をそのまま使う
- HTTP handler 1 本追加（`backend/internal/uploadverification/interface/http/handler.go`）
- wireup で `CloudflareVerifier`（PR20 既実装）を注入
- router に登録（`/api/photobooks/{id}/upload-verifications`、draft session middleware で認可）

**選択肢 B（非推奨）**: tokengen を Frontend から呼べるように public 化 → セキュリティ崩壊

→ **A 採用**。実装範囲は約 50 行（handler + wireup + router 1 行）。

### 3.1 注意: Turnstile siteverify への接続

PR21 Step D で `TURNSTILE_SECRET_KEY` を Cloud Run に注入済。Backend は
`cloudflare_verifier.NewCloudflareVerifier` を起動時に組み立てて wireup へ渡す。

---

## 4. Turnstile Frontend 方針

### 4.1 sitekey の配送

- `NEXT_PUBLIC_TURNSTILE_SITE_KEY`: 公開値、`frontend/.env.production` に inline
- OpenNext build 時に Frontend bundle に embed される（PR14 と同パターン）
- local dev: Cloudflare 公式 test sitekey `1x00000000000000000000AA`（常に success）を使う

```
NEXT_PUBLIC_TURNSTILE_SITE_KEY=<本番 widget の Site Key>
```

### 4.2 widget script 読み込み

```html
<script src="https://challenges.cloudflare.com/turnstile/v0/api.js" async defer></script>
```

Next.js の `<Script>` component で `strategy="afterInteractive"` で読み込む。
Server Component 配下で読むため、Edit page を Client Component 化（`"use client"`）するか、
Turnstile 部分のみ Client component で隔離する。

### 4.3 React component 化（推奨案）

```tsx
"use client";
function TurnstileWidget({ sitekey, action, onVerify }: {
  sitekey: string;
  action: string;
  onVerify: (token: string) => void;
}) { ... }
```

- `window.turnstile.render(...)` を `useEffect` で呼ぶ
- token は `onVerify` callback で親に渡す
- token 期限切れで再 challenge 起動
- failure 時は `onError` callback で UI 表示

### 4.4 Mobile Safari / iPhone Safari の表示

- Cloudflare Turnstile の "Managed" モードは iPhone Safari / Private Browsing 含めて
  動作（Cloudflare 公式 doc）。ただし Private Browsing では `localStorage` 制約で
  challenge が再表示されるケースあり
- WebKit の preflight / cookie behavior と相性が出ないかは PR22 実機確認で検証
- widget 高さは可変、layout 固定が必要なら `fixed-size` mode を後日検討

---

## 5. Upload flow（最小骨格）

```
1. User opens https://app.vrc-photobook.com/edit/<photobookId>
2. Browser has vrcpb_draft_<id> HttpOnly Cookie（PR15 で発行済）
3. User selects image via <input type="file">
4. File が 10MB 以下 / image/jpeg|png|webp|heic を Frontend で軽量検証（UX、Backend で再検証）
5. Turnstile widget が token を発行 (onVerify callback)
6. Frontend: POST /api/photobooks/{id}/upload-verifications
   Body: { turnstile_token }
   Response: { upload_verification_token, expires_at, allowed_intent_count }
7. Frontend: POST /api/photobooks/{id}/images/upload-intent
   Header: Authorization: Bearer <upload_verification_token>
   Body: { content_type, declared_byte_size, source_format }
   Response: { image_id, upload_url, required_headers, storage_key, expires_at }
8. Frontend: PUT to <upload_url>
   Headers: required_headers の Content-Type / Content-Length
   Body: File object
9. Frontend: POST /api/photobooks/{id}/images/{imageId}/complete
   Body: { storage_key }
   Response: { image_id, status: "processing" }
10. UI: ファイルカードに "アップロード完了（処理中）" 表示、状態 processing
```

---

## 6. R2 PUT from browser

### 6.1 fetch vs XMLHttpRequest

| 案 | 利点 | 欠点 |
|---|---|---|
| **案 A（推奨、初期）**: `fetch()` で PUT | 標準 API / Promise / 簡潔 | progress 取得不可（`ReadableStream` upload は Safari でサポート薄い） |
| 案 B: `XMLHttpRequest` で PUT | `xhr.upload.onprogress` で progress 取得可 | 古い API、Promise 化に wrapper 必要 |

→ **PR22 では案 A（fetch）で start**、progress は「アップロード中…」固定表示で足りる。
将来 progress bar が UX 的に必要なら PR23 以降で XHR に切替。

### 6.2 必須 header

upload-intent response の `required_headers` に従う:
- `Content-Type`: 申告値（例: `image/jpeg`）
- `Content-Length`: File.size（fetch では自動付与、明示不可だが OK）
- `Host`: ブラウザ自動

PR21 Step E の M1 PoC で確認済: aws-sdk-go-v2 presign は Content-Length / Content-Type が
SignedHeaders に含まれるため、宣言と実 PUT の整合が必須。

### 6.3 CORS preflight

R2 bucket CORS（PR21 Step B 設定済）:
- `AllowedOrigins`: `https://app.vrc-photobook.com`
- `AllowedMethods`: PUT / HEAD / GET
- `AllowedHeaders`: Content-Type / Content-Length / Authorization
- `MaxAgeSeconds`: 3600

PR22 で実機検証時に preflight OPTIONS が成功することを確認。

### 6.4 失敗時 UI

- network failure → 「アップロードに失敗しました。再試行してください」
- 4xx (R2 SignatureDoesNotMatch / 400) → 「アップロード設定に問題があります」+ retry button
- 5xx → 「サーバーエラー」 + retry button

---

## 7. UI 設計

### 7.1 design/mockups/prototype 参照（直接 import しない）

`design/mockups/prototype/screens-a.jsx` の Edit screen を **デザインの参考**として確認:
- 写真を追加 ボタン（破線 dashed 枠 / `card` style）
- 「JPG / PNG / WEBPに対応、最大200枚まで追加できます。」（PR22 では HEIC も追加）
- 「または、ここにドラッグ&ドロップ」（drag&drop は PR23 以降、PR22 ではボタンのみ）

prototype は **import しない**（PR18 計画書 §F の方針通り）。Tailwind CSS で再実装、
design tokens（`--teal #14B8A6` / `--radius` 12px / `.t-h1` 26px/800 等）は globals.css に
取り込む（既存 `frontend/app/globals.css` の確認 + 必要なら拡張）。

### 7.2 PR22 で実装する UI 要素（最小）

| 要素 | 実装内容 |
|---|---|
| Edit page | photobook_id 表示 + 以下の要素を含むレイアウト |
| ファイル選択ボタン | `<input type="file" accept="image/jpeg,image/png,image/webp,image/heic" />` |
| Turnstile widget | TurnstileWidget component（§4.3） |
| 進捗表示 | "Turnstile 確認中" / "アップロード中..." / "処理中（完了済み）" のテキスト |
| エラー表示 | 失敗時のメッセージ（固定文言、原因詳細を出さない） |
| photo card | アップロード済 image_id を表示（PR22 段階では画像表示はしない、ID + status のみ） |

### 7.3 mobile / PC layout

- mobile（iPhone Safari 想定）: 1 列縦並び
- PC: max-width 制限 + 中央寄せ（既存 globals.css の Tailwind utility で十分）
- 詳細レスポンシブは PR23 以降

---

## 8. Backend 追加範囲

### 8.1 PR22 で実装する Backend 変更

| ファイル | 内容 |
|---|---|
| `backend/internal/uploadverification/interface/http/handler.go` | `IssueUploadVerification` handler 1 本 |
| `backend/internal/uploadverification/wireup/wireup.go` | `BuildHandlers(repo, verifier, hostname, action)` |
| `backend/internal/http/router.go` | `/api/photobooks/{id}/upload-verifications` を draft session middleware 経由で登録 |
| `backend/cmd/api/main.go` | Cloudflare verifier を起動時に組み立てて wireup へ渡す |
| `backend/internal/uploadverification/interface/http/handler_test.go` | success / Turnstile failure / draft session 不正 / photobook 不一致 |

### 8.2 PR22 で追加しない Backend 変更

- image-processor 本体
- variant 生成 / available 化
- public image URL 配信
- Outbox events
- SendGrid

### 8.3 既存 endpoint との関係

- `POST /api/photobooks/{id}/images/upload-intent`: PR21 で実装済、変更なし
- `POST /api/photobooks/{id}/images/{imageId}/complete`: PR21 で実装済、変更なし
- `POST /api/auth/draft-session-exchange`: PR9c で実装済、変更なし

---

## 9. Frontend env / Workers redeploy

### 9.1 frontend/.env.production 拡張

PR14 で:
```
NEXT_PUBLIC_BASE_URL=https://app.vrc-photobook.com
NEXT_PUBLIC_API_BASE_URL=https://api.vrc-photobook.com
COOKIE_DOMAIN=.vrc-photobook.com
```

PR22 で追加:
```
NEXT_PUBLIC_TURNSTILE_SITE_KEY=<本番 widget の Site Key>
```

`.env.production` は引き続き `.gitignore` 除外（PR14 と同じ）。

### 9.2 Workers redeploy 手順（PR14 と同じ）

```sh
# typecheck / test / build
npm --prefix frontend run typecheck
npm --prefix frontend test
npm --prefix frontend run build
npm --prefix frontend run cf:build

# deploy（cwd を frontend に移すサブシェル経由、wsl-shell-rules.md §1）
( cd /home/erenoa6621/dev/vrc_photobook/frontend && wrangler deploy )
```

### 9.3 rollback

```sh
( cd frontend && wrangler deployments list )
( cd frontend && wrangler rollback <旧 deployment ID> )
```

PR14 で確立した手順、Custom Domain `app.vrc-photobook.com` は維持される。

---

## 10. Safari / iPhone Safari 確認方針

`.agents/rules/safari-verification.md` の必須項目を満たす。

### 10.1 macOS Safari 最新

- [ ] /edit/<photobookId> で draft session Cookie が DevTools で確認できる
- [ ] Turnstile widget が表示される
- [ ] file picker でローカル画像を選択できる
- [ ] upload-verifications POST → 201
- [ ] upload-intent POST → 201
- [ ] R2 PUT preflight OPTIONS / PUT 成功
- [ ] complete POST → 200 / status processing
- [ ] UI に「処理中」表示
- [ ] ページリロード後も Cookie 維持
- [ ] `referrer-policy: no-referrer` が `/edit/<id>` で確認できる
- [ ] 同 Cookie が `api.vrc-photobook.com` への fetch で送られている（Domain=.vrc-photobook.com）

### 10.2 iPhone Safari 最新

- [ ] /edit/<photobookId> redirect 着地後にページが表示される
- [ ] Turnstile widget が表示・操作可能
- [ ] file picker（カメラロール / 写真選択）から画像を取得できる
- [ ] HEIC ファイルも選択できる（Backend で受領、PR23 image-processor で変換）
- [ ] upload flow が完走する
- [ ] processing 表示が出る
- [ ] ページリロード後もページが開ける（Cookie 維持）

### 10.3 Private Browsing（任意）

- 可能なら macOS Safari Private Browsing でも 1 回確認
- iPhone Safari Private Browsing は実機制約で省略可

### 10.4 24 時間 / 7 日後の Cookie 残存（継続観察、本 PR 完了判定外）

`.agents/rules/safari-verification.md` の「継続観察項目」として記録するのみ。
PR22 完了判定には含めない。

---

## 11. Test 方針

### 11.1 PR22 で書くテスト

**Backend**:
- handler test: upload-verifications 201 / Turnstile failure 403 / draft session 401 / photobook 不一致 401
- wireup test（Cloudflare verifier の cfg 注入）

**Frontend**:
- TurnstileWidget unit test（Vitest + React Testing Library、`window.turnstile` を mock）
- upload flow unit test（fetch を mock、3 endpoint の連続呼び出し）
- File validation unit test（10MB / content_type / source_format 軽量検証）

**Manual browser test**（実機）:
- macOS Safari / iPhone Safari の §10 checklist
- 実 R2 への 1 image upload 完走（cleanup tool で削除）

### 11.2 PR22 で書かないテスト

- E2E (Playwright 等)
- 並行 upload race
- HEIC 表示（image-processor 不在のため）
- Safari Private Browsing 自動 test

---

## 12. Security / プライバシー

- Turnstile token を logs / 画面 / URL に出さない
- upload verification token を logs / 画面 / URL に出さない
- presigned URL を logs に出さない（response body には乗る、UI には絶対出さない）
- Cookie 値を logs / 画面 / コンソールに出さない
- storage_key は UI 表示しない、必要最小限の記録のみ
- file name は DB 保存しない（Frontend 側で File.name を upload-intent body に乗せない）
- SVG / HTML 拒否（Frontend 軽量検証 + Backend 再検証）
- 10MB 制限（File.size > 10MB は Frontend で拒否、Backend でも CHECK）
- CORS は `app.vrc-photobook.com` 厳格一致
- Workers Custom Domain (PR15) / Cloud Run Domain Mapping (PR12) を維持

---

## 13. PR22 実装範囲の明確化

### PR22 で実装する

- Backend: `POST /api/photobooks/{id}/upload-verifications` endpoint + handler + wireup + tests
- Frontend:
  - TurnstileWidget React component
  - Edit page Client component 化 + upload UI 最小骨格
  - API client wrapper（upload-verifications / upload-intent / R2 PUT / complete）
  - error / progress 表示
- Frontend env: `NEXT_PUBLIC_TURNSTILE_SITE_KEY` 追加
- Workers redeploy
- Safari / iPhone Safari 実機確認 + 作業ログ
- `frontend/.env.production` の見本値追加（実値は引き続き gitignore）

### PR22 で実装しない

- image-processor 本体（HEIC 変換 / EXIF 除去 / variant 生成 / available 化）
- 公開 Viewer での画像表示
- OGP 生成
- moderation UI
- Outbox events
- SendGrid
- 編集 UI のフル機能（caption 編集 / page reorder / cover 設定 / publish ボタン）
- design system の正式抽出
- Cloud SQL / spike 削除
- Public repo 化

---

## 14. Cloud SQL 残置/一時削除判断

### 14.1 PR22 計画書完了時点での判断材料

- PR22 実装にすぐ進むなら残置（実 R2 upload + Backend test に DB 必要）
- 数日空くなら一時削除
- 累計（PR17 完了から本書まで）: ~9 時間 / ~¥21
- R2 実 object が今後増える（dummy 上書きしながら検証）のでクリーンアップ意識

### 14.2 推奨

**残置継続**（PR22 実装に連続着手予定 + Safari 実機確認の手戻り回避）。
次回判断タイミング: 「PR22 実装 PR の完了時 or 2 日後」の早い方。

---

## 15. ユーザー判断事項（PR22 着手前に確認）

| # | 判断対象 | 推奨案 | 代替案 | 影響 |
|---|---|---|---|---|
| Q1 | upload-verifications endpoint を PR22 で追加 | **追加する**（選択肢 A） | tokengen で代用（不可） | 推奨に従って良いか |
| Q2 | Turnstile widget 表示位置 | **アップロードボタン直上 + ファイル選択後に表示**（UX 最小フリクション） | 常時表示 | UX 軽微差 |
| Q3 | upload progress | **fetch + 「アップロード中…」固定表示**（推奨） | XHR で progress bar | 実装複雑度 |
| Q4 | processing 止まり UI | **「処理中（しばらく後に表示されます）」テキストのみ**（PR23 で variant / 表示画像） | spinner / animation | UX 妥協 |
| Q5 | Safari 実機確認の範囲 | **macOS Safari + iPhone Safari、Private Browsing は任意** | Private Browsing も必須 | 推奨で safari-verification.md 必須項目満たす |
| Q6 | TurnstileWidget の React 実装 | **自前 component（公式 script 動的読み込み）** | `react-turnstile` 等 npm package | ライブラリ依存削減 |
| Q7 | local dev での Turnstile | **公式 test sitekey `1x00...AA` を使う** | 本番 sitekey 流用 | dev / prod 分離 |
| Q8 | edit page の Client / Server 構造 | **Edit page を Client 化、Turnstile / upload UI 全部 Client**（推奨） | Turnstile 部分のみ Client、他は Server | shopify ベース構造維持 |
| Q9 | HEIC accept | **`image/heic` を `<input accept>` に含める** | jpg/png/webp のみ | iPhone Safari ユーザー UX |
| Q10 | drag & drop | **PR22 では実装しない**（ボタン選択のみ） | 実装する | スコープ拡大回避 |
| Q11 | UI design system 抽出 | **PR22 ではしない**（prototype 参照、再実装で対応） | design system PR を先に挟む | スコープ拡大回避 |
| Q12 | file size を Frontend で軽量検証 | **する（10MB 上限 + content_type）** | Backend 任せ | UX 改善 |
| Q13 | upload 完了後の photo grid 表示 | **image_id + status のみ**（画像表示なし） | 画像表示も入れる | image-processor 待ち |
| Q14 | 並行 upload | **直列**（1 ファイルずつ） | 並行 | UX / 上限管理 |
| Q15 | Workers redeploy のタイミング | **Backend 完了 + Frontend 完了 + ローカル動作確認後** | Backend だけで先行 deploy | rollback 可能 |
| Q16 | Cloud SQL 残置 | **残置継続** | 一時削除 | PR21 判断と整合 |
| Q17 | Public repo 化 | **PR22 完了後でもまだ保留** | PR22 完了で公開化 | secret scan 必要 |

Q1〜Q17 への回答後、PR22 実装に進む。

---

## 16. 関連

- [PR21 R2 + presigned URL 計画 / 実装結果](./m2-r2-presigned-url-plan.md)
- [PR20 Upload Verification 計画](./m2-upload-verification-plan.md)
- [PR18 Image aggregate 計画](./m2-image-upload-plan.md)
- [PR19 Photobook ↔ Image 連携 計画](./m2-photobook-image-connection-plan.md)
- [ADR-0005 画像アップロード方式](../adr/0005-image-upload-flow.md)
- [Post-deploy Final Roadmap](../../harness/work-logs/2026-04-27_post-deploy-final-roadmap.md)
- [`.agents/rules/safari-verification.md`](../../.agents/rules/safari-verification.md)
- [`.agents/rules/security-guard.md`](../../.agents/rules/security-guard.md)
- [`docs/security/public-repo-checklist.md`](../security/public-repo-checklist.md)
