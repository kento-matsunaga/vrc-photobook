# 公開 Viewer / 管理ページ 最小骨格 実装計画（PR24 計画書）

> 作成日: 2026-04-27
> 位置付け: 新正典 [`docs/plan/vrc-photobook-final-roadmap.md`](./vrc-photobook-final-roadmap.md) §3 PR24 の計画書本体。
> 本書では計画のみ確定し、実装は PR25 で行う。
>
> 上流参照（必読）:
> - [新正典ロードマップ](./vrc-photobook-final-roadmap.md)
> - [業務知識 v4](../spec/vrc_photobook_business_knowledge_v4.md) §2.3 公開 URL / §3.2 visibility / §6 manage URL
> - [Photobook ドメイン設計](../design/aggregates/photobook/ドメイン設計.md)
> - [Photobook データモデル設計](../design/aggregates/photobook/データモデル設計.md) §3 photobooks
> - [Image データモデル設計](../design/aggregates/image/データモデル設計.md) §4 image_variants
> - [ADR-0005 Image Upload Flow](../adr/0005-image-upload-flow.md) §display variant 配信
> - [PR23 image-processor 計画](./m2-image-processor-plan.md) §7 / §8 storage_key
> - [PR23 image-processor 結果](../../harness/work-logs/2026-04-27_image-processor-result.md)
> - [PR22 frontend upload UI 結果](../../harness/work-logs/2026-04-27_frontend-upload-ui-result.md)
> - [Safari 検証ルール](../../.agents/rules/safari-verification.md)
> - [Security Guard ルール](../../.agents/rules/security-guard.md)

---

## 1. 目的

- 公開 Viewer `/p/[slug]` の最小骨格を設計する
- 管理ページ `/manage/[photobookId]` の最小骨格を設計する
- display / thumbnail variant の表示方式（presigned GET / public access / Workers proxy）を決める
- public / manage の認可境界を確定する
- 公開 slug / 公開条件（status / visibility / hidden_by_operator）を確定する
- PR25 の実装範囲を 1〜2 PR に分割可能なところまで具体化する

---

## 2. PR24 対象範囲

### 対象（本計画書で確定する）

- Viewer / Manage の route 定義
- Viewer から見える項目と「見せない」項目の境界
- Backend public / manage read endpoint の I/F 案
- display / thumbnail variant の配信方式選定
- manage Cookie の認可方針（既存 session middleware の流用範囲）
- design prototype からの抽出方針
- Safari 確認チェック項目
- Test 方針
- Security 守るべき不変条件
- PR25 で「やる / やらない」の境界

### 対象外（本計画書では決めない）

- 実装本体（PR25）
- 編集 UI 本格化（PR26〜PR27）
- publish flow 完成（PR28）
- Outbox / SendGrid 連携（PR30〜PR32）
- OGP 自動生成（PR33）
- Moderation / Report / UsageLimit
- LP / terms / privacy / about（PR37）

---

## 3. 公開 Viewer 方針

### 3.1 route

- `/p/[slug]` を採用（mockup `design/mockups/prototype/screens-b.jsx` Line 42 で `https://vrc-photobook.com/p/abc123xyz` と一致）
- 配下に `frontend/app/(public)/p/[slug]/page.tsx` を新設
- `(public)` route group は新設（既存は `(draft)` / `(manage)`）

### 3.2 公開条件と HTTP ステータス

| photobook の状態 | Viewer の応答 | 理由 |
|---|---|---|
| `status='published'` AND `hidden_by_operator=false` | **200** + 表示 | 通常公開 |
| `status='published'` AND `hidden_by_operator=true` | **410 Gone** | 運営による一時非表示。slug は既知だが現在閲覧不可 |
| `status='deleted'` / `'purged'` | **404 Not Found** | 復元不可。slug 復元ルール（業務知識 v4）に従い、別 photobook が同 slug を再利用しないが、外部には 404 |
| `status='draft'` | **404 Not Found** | draft の存在を漏らさない |
| slug 不一致 | **404 Not Found** | 標準 |
| `visibility='private'` | **404 Not Found** | private は MVP では「配信しない」扱い（slug が漏れても表示しない） |
| `visibility='public'` / `'unlisted'` | 上記の status 判定に従う | 違いは SEO（§3.5） |

> 410 と 404 を分ける理由: 通報や運営対応を伝える将来 UI に拡張余地を残す（PR35）。MVP UI は両方とも「閲覧できません」表示で問題ない。

### 3.3 SSR / レンダリング

- `export const dynamic = "force-dynamic"`（既存 edit / draft と統一）
- ISR / SSG は採用しない（hidden_by_operator のリアルタイム反映が必要）
- Cache-Control: `private, no-store`（個人作成 photobook、indexing は §3.5 で別制御）

### 3.4 Viewer で表示する項目（PR25 範囲）

- title / cover_title（ある場合）
- creator_display_name（任意）
- description（任意）
- type / layout / opening_style に応じた最小レイアウト（mockup 抽出）
- ページ並び（display_order ASC）
- 各 photo の display variant 画像
- thumbnail variant は grid サムネイル / OGP 用に保持
- 通報導線 placeholder（実装は PR35）

### 3.5 SEO / noindex

- MVP は **常に `noindex,nofollow`** とする（visibility=public でも MVP は noindex）
  - 理由: 業務知識 v4 のスマホファースト + ログイン不要設計上、検索エンジン経由よりも X 共有が主動線
  - 公開・拡散戦略確定後（PR37 LP 着手以降）に visibility=public のみ index 解禁を検討
- `<meta name="robots" content="noindex,nofollow">`
- `Referrer-Policy: strict-origin-when-cross-origin`（既存 middleware 維持）

### 3.6 OGP placeholder

- PR25 では og:title / og:description のみ配置
- og:image は **本実装まで placeholder（または未出力）**。PR33 で OGP 独立 table + 自動生成を導入し、その時点で og:image を生成 URL に切り替える
- twitter:card は `summary_large_image` を見越して `summary` にしておく（後続 PR で切替）

---

## 4. 管理ページ方針

### 4.1 route

- `/manage/[photobookId]` 既存（最小 placeholder）を本実装に置き換え
- token URL `/manage/token/[token]` → Cookie 化済 → redirect で本ページに着地（既存）

### 4.2 認可

- 既存 manage session middleware を流用
- manage Cookie が無効 / 期限切れ / photobookId 不一致 → `/?reason=invalid_manage_session` 等への redirect（既存パターン）
- manage Cookie の Domain / Path / SameSite は既存仕様維持（Safari ITP 確認済）

### 4.3 管理ページで表示する項目（PR25 範囲）

- 公開 URL（`https://app.vrc-photobook.com/p/{slug}`、コピー UI、mockup `UrlRow` 流用）
- manage URL（再ログイン用）— 表示は **発行直後のみ**、再表示は不可（業務知識 v4）。PR25 は再発行 UI placeholder
- 公開状態（published / hidden / deleted）
- 画像数（available 数）
- 公開停止 / 再公開 placeholder（実装は PR28 publish flow 完成 / PR34 Moderation）
- 公開設定 placeholder（visibility 切替）

### 4.4 PR25 で「やらない」もの

- manage URL 再発行の本実装（PR32 で SendGrid + ManageUrlDelivery 経由メール送信時に確定）
- 公開停止 / 再公開（PR28 / PR34）
- visibility 切替の本実装（PR27 編集 UI 本格化 / PR28 publish flow）

---

## 5. display variant 配信方式

### 5.1 候補比較

| 案 | 説明 | 利点 | 欠点 |
|---|---|---|---|
| **A: 短命 presigned GET URL を Backend が返す** | API レスポンスに `display_url` / `thumbnail_url`（5〜15 分有効）を埋める | R2 public 不要 / 既存 R2 client 流用 / Cookie auth と整合 | URL 失効で再取得が必要 / CDN キャッシュ最適化が弱い |
| B: Workers API route が Backend 経由で proxy | `/api/img/...` で Workers が Backend 経由で取得して中継 | URL 安定 / Cookie / Cloudflare CDN | Workers CPU/帯域コスト / 設計複雑化 |
| C: R2 bucket を public access ON | 直接配信 | 最速 / CDN 効果最大 | 全画像が誰でも GET 可能になる（unlisted / hidden の意味喪失） |
| D: Cloudflare Images / R2 custom domain（後続） | 専用画像配信ドメイン + token-signed URL | 本格運用向き | 初期コスト / 別ドメイン設計が必要 |

### 5.2 推奨: **案 A（短命 presigned GET URL）**

理由:
- R2 public access OFF を維持できる（hidden / private の意味を保てる）
- 既存 `r2.Client` に GetObject / PutObject / ListObjects があるので `PresignGetObject` 追加だけで対応可能
- Cookie / token と整合的
- Cloudflare CDN 利用は PR40+ の改善で（custom domain 化 / Cloudflare Images 評価）

### 5.3 PR25 で実装する変更

- `r2.Client` に `PresignGetObject(ctx, key, expiresIn)` を追加（Backend 側のみ、Frontend は触らない）
- API レスポンス例:
  ```
  {
    "photobook_id": "...",
    "title": "...",
    "type": "...",
    "layout": "...",
    "pages": [
      { "page_id": "...", "display_order": 0, "photos": [
        { "photo_id": "...", "image_id": "...", "display_url": "https://...", "thumbnail_url": "https://...", "expires_at": "..." }
      ]}
    ]
  }
  ```
- presigned URL の有効期限: **15 分**（既存 PUT presign と統一、Frontend での再 fetch 設計を後続 PR で）
- presigned URL は `image_id` ごとに **その時点で生成**（DB に保存しない、URL ローテーションを許容）

### 5.4 PR25 では実装しない

- presigned URL の自動 refresh（Frontend 側のロジックは PR41+ で）
- Cloudflare Images / custom domain
- CDN cache 戦略

---

## 6. Backend endpoint 候補

### 6.1 PR25 で実装する

- **`GET /api/public/photobooks/{slug}`**
  - Cookie 不要（public 経路）
  - response: photobook 公開メタ + pages + photos + variant URLs（短命 presigned GET）
  - 4xx 表現は §3.2 表に準拠
- **`GET /api/manage/photobooks/{id}`**
  - manage Cookie 必須
  - response: 公開 URL / manage URL（発行直後のみ表示用フラグ）/ 状態 / 画像数 + 編集 UI に渡す情報のサブセット

### 6.2 PR25 で実装しない

- `POST /api/manage/photobooks/{id}/reissue-manage-url` — 再発行は PR32 で SendGrid + Outbox 連携と一体実装
- `GET /api/images/{id}/display-url?variant=display` — variant URL は public/manage の応答に **埋め込み**で返す（独立 endpoint は不要）
- 編集系 endpoint（PR27）

### 6.3 router への追加

`backend/internal/http/router.go` 既存パターンを踏襲して以下を追加:

```
r.Get("/api/public/photobooks/{slug}", publicHandlers.GetBySlug)

r.Route("/api/manage/photobooks/{id}", func(sub chi.Router) {
    sub.Use(authmiddleware.RequireManageSession(...))  // 新設、既存draft middleware と対称
    sub.Get("/", manageHandlers.GetByID)
})
```

> 既存 draft middleware（`RequireDraftSession`）と同様、manage Cookie を見る `RequireManageSession` を auth/session/middleware に追加する。実装は PR25。

---

## 7. Frontend route 候補

### 7.1 PR25 で実装するファイル

| パス | 役割 |
|---|---|
| `frontend/app/(public)/p/[slug]/page.tsx` | Viewer SSR ページ（Server Component） |
| `frontend/app/(public)/layout.tsx`（必要なら） | public 用レイアウト |
| `frontend/app/(manage)/manage/[photobookId]/page.tsx`（既存を置き換え） | Manage SSR ページ |
| `frontend/lib/publicPhotobook.ts` | public lookup API client |
| `frontend/lib/managePhotobook.ts` | manage lookup API client |
| `frontend/components/Viewer/ViewerLayout.tsx` | Viewer レイアウト Server Component |
| `frontend/components/Viewer/Photo.tsx` | 画像表示 Server Component（display variant） |
| `frontend/components/Manage/ManagePanel.tsx` | Manage 主要パネル |
| `frontend/components/UrlRow.tsx` | mockup `UrlRow` の Tailwind 版 |

### 7.2 PR25 を 1 PR or 2 PR に分割するか

- **PR25a**: Backend endpoint + R2 PresignGetObject + Backend test
- **PR25b**: Frontend Viewer + Manage + design-system 第一弾 + Safari 確認

実装容量で 1 PR が妥当か 2 PR に分けるかは PR25 着手時に再判定（計画書段階では「分割可能性を残す」のみ確定）。

---

## 8. design 参照方針

### 8.1 抽出ルール

- prototype は **値の抽出元**。直接 import / コピペしない（新正典 §0 ルール 4 を厳守）
- 抽出した値は `design/design-system/` に正典化し、`tailwind.config.ts` に反映する

### 8.2 PR25 で抽出する prototype 資産

| 用途 | prototype 参照点 |
|---|---|
| Viewer モバイル | `design/mockups/prototype/screens-b.jsx` `Viewer` (Line 128) |
| Viewer PC | `design/mockups/prototype/pc-screens-b.jsx` `PCViewer` |
| Manage モバイル | `design/mockups/prototype/screens-b.jsx` `Manage` (Line 242) |
| Manage PC | `design/mockups/prototype/pc-screens-b.jsx` `PCManage` |
| URL 表示 | `design/mockups/prototype/shared.jsx` `UrlRow` |
| color / typography / spacing / radius / shadow token | `design/mockups/prototype/styles.css` / `pc-styles.css` |
| Photo placeholder（実画像未到達時） | `design/mockups/prototype/shared.jsx` `Photo` (`v-a`〜`v-f`) |
| Steps（公開フロー進捗、PR28 で本格化） | `design/mockups/prototype/shared.jsx` `Steps` |

### 8.3 design-system 第一弾の範囲（PR25）

`design/design-system/` 配下に以下を作成（PR25 内で完結する量に絞る）:

- `colors.md` — teal / neutral / status のみ
- `typography.md` — font-size / font-weight / line-height
- `spacing.md` — gap / padding の段階値
- `radius-shadow.md` — radius / shadow の段階値
- `tailwind.config.ts` への反映

PR41+ で正式化（components.md / motion.md / Photobook type 別カラー等）。

---

## 9. Publish flow との境界

### 9.1 課題

- Viewer を作るには `status='published'` の photobook が必要
- 一方 PR28（publish flow 完成）はまだ着手していない

### 9.2 既存実装の現状

- Backend には **`PublishFromDraft` UseCase が既に存在**（`backend/internal/photobook/internal/usecase/publish_from_draft.go`）
- slug 生成 / manage token 発行 / status 遷移 / draft session revoke が同 TX 内で実行される
- HTTP endpoint は未公開（router 未接続）

### 9.3 推奨方針

PR25 では **Viewer / Manage の Read 経路のみ**を実装し、publish の UI / endpoint は触らない。

検証用に published 状態の photobook を準備するには次のどちらかを採る:

- **案 P1（推奨）**: 既存の `_tokengen`（PR17 で作成、cleanup 済）と同じパターンで、PR25 の Safari 確認用に **一時 CLI** を一度だけ実行して `PublishFromDraft` UseCase を呼ぶ。raw token / slug 値はチャットや work-log に出さず、URL のみ手元の一時ファイルに残す
- 案 P2: PR25 内で publish 用 HTTP endpoint も追加する → PR28 と境界が曖昧になるため避ける

PR28 で publish flow を完成（UI + endpoint + 完了画面）させる際に、PR25 で確認した Read 経路に publish 動線を被せる形になる。

---

## 10. Safari 確認方針

`.agents/rules/safari-verification.md` の発火条件に該当する。

### 10.1 必須確認項目（PR25 完了前）

- macOS Safari（最新）
- iPhone Safari（最新、可能なら 1 世代前も）

### 10.2 シナリオ

#### 公開 Viewer

- `https://app.vrc-photobook.com/p/{slug}` 直アクセス
  - 200 / レイアウト破綻なし / display 画像が表示される
  - URL に raw token / Cookie が出ない
  - meta tags（og / robots）が SSR HTML に出力される
- `slug` 不一致 → 404
- `status='draft'` の slug 推測 → 404
- ページ再読み込み・back/forward で破綻なし

#### 管理ページ

- token URL 経由で manage Cookie 取得 → `/manage/{id}` 着地 → 200
- 公開 URL のコピー（mobile / PC）
- Cookie 維持確認
- ページ再読み込みで session が維持される

### 10.3 監視（運用開始後）

- 24 時間後 / 7 日後の Cookie 残存確認（既存 ITP 観察に追加）

---

## 11. Test 方針

### 11.1 Backend

- `internal/photobook/infrastructure/repository/rdb/` に slug lookup query 追加 → 実 DB test
- `internal/photobook/internal/usecase/get_public_photobook.go`（新設）→ status / hidden / visibility の各分岐を table-driven test
- `internal/photobook/internal/usecase/get_manage_photobook.go`（新設）→ photobook_id / Cookie 整合の test
- `internal/imageupload/infrastructure/r2/aws_client.go` に `PresignGetObject` 追加 → unit test（fake R2 / 既存 fake_r2_client.go 拡張）
- HTTP handler test:
  - public: 200 / 404（draft）/ 404（deleted）/ 410（hidden）
  - manage: Cookie あり 200 / Cookie なし 401 / Cookie あるが photobook_id 不一致 401

### 11.2 Frontend

- `frontend/app/(public)/p/[slug]/page.test.ts`（新設）— SSR メタタグ / noindex / og:title が出る
- `frontend/lib/publicPhotobook.test.ts` — 4xx 系の error mapping
- `frontend/lib/managePhotobook.test.ts` — 同上
- 既存 `frontend/lib/__tests__/upload.test.ts` を壊さない確認

### 11.3 セキュリティ系 test

- API 応答に raw token / R2 credentials / storage_key 完全値 / DATABASE_URL が出ないことを grep
- presigned URL は **応答 body に出る**ものの、Cloud Run / Workers logs には出ないことを確認
- noindex / robots tags が SSR HTML に出ること

### 11.4 Manual

- Safari 確認は §10 を 1 回実施

---

## 12. Security

### 12.1 守るべき不変条件

- **unpublished / draft / deleted / purged は 404**（存在を漏らさない）
- **hidden_by_operator は 410 か 404**（PR25 は 410 採用、内部観察用）
- **private visibility は 404**
- **manage page は manage Cookie 必須**、Cookie 不在 / 期限切れ / photobook 不一致は **401 + redirect**（既存 draft 経路と対称）
- **display URL に R2 credentials を出さない**（presigned URL のクエリ署名のみ）
- **presigned URL を Cloud Run / Workers logs / 構造化 log の field に出さない**
- **storage_key 完全値を UI / log / API response に出さない**（API は presigned URL のみを返し、key 自体は内部にとどめる）
- **public page に編集用 URL / draft token / manage token を出さない**
- **manage URL 再発行時の URL は再表示しない**（業務知識 v4、PR32 で SendGrid 経由メール送信のみで通知）

### 12.2 grep 監査項目（PR25 commit 前）

```
grep -RInE "DATABASE_URL=|PASSWORD=|SECRET=|SECRET_KEY|API_KEY|sk_live|sk_test|draft_edit_token|manage_url_token|session_token|Set-Cookie|R2_SECRET_ACCESS_KEY=|TURNSTILE_SECRET_KEY=" \
  backend/internal/photobook/ backend/internal/imageupload/ frontend/app/ frontend/lib/ frontend/components/
```

実値ヒット 0 件。用語ヒットのみ可。

---

## 13. PR25 実装範囲（最終確認用 checklist）

### 13.1 Backend

- [ ] `PresignGetObject` を `r2.Client` interface / AWSClient / FakeR2Client に追加
- [ ] `GetPublicPhotobook` UseCase（slug → published photobook + variants の URL 一覧）
- [ ] `GetManagePhotobook` UseCase（id + manage session → manage view）
- [ ] `RequireManageSession` middleware（既存 `RequireDraftSession` と対称）
- [ ] `GET /api/public/photobooks/{slug}` handler
- [ ] `GET /api/manage/photobooks/{id}` handler
- [ ] router 接続 + main.go wiring
- [ ] sqlc query 追加（slug lookup、status / visibility 含む）
- [ ] handler / usecase / repository test
- [ ] Cloud Run revision 更新

### 13.2 Frontend

- [ ] `app/(public)/p/[slug]/page.tsx`
- [ ] `app/(public)/layout.tsx`（必要なら）
- [ ] `app/(manage)/manage/[photobookId]/page.tsx` 本実装
- [ ] `lib/publicPhotobook.ts` / `lib/managePhotobook.ts`
- [ ] `components/UrlRow.tsx` / `components/Viewer/ViewerLayout.tsx` / `components/Viewer/Photo.tsx` / `components/Manage/ManagePanel.tsx`
- [ ] design-system 第一弾（colors / typography / spacing / radius-shadow / tailwind 反映）
- [ ] route test
- [ ] Workers redeploy

### 13.3 検証

- [ ] Safari macOS / iPhone（§10）
- [ ] log / response の secret leak 監査
- [ ] noindex / OGP placeholder の SSR HTML 出力確認

### 13.4 PR25 で実装しない（再掲）

- publish flow 完成（PR28）
- 編集 UI 本格化（PR27）
- Outbox（PR30）
- SendGrid 連携（PR32）
- OGP 自動生成（PR33）
- Moderation（PR34）/ Report（PR35）/ UsageLimit（PR36）
- LP / terms / privacy / about（PR37）
- Cloudflare Images / custom domain 配信（PR41+）

---

## 14. 実リソース操作

| 操作 | 必要性 |
|---|---|
| Workers redeploy（vrcpb-frontend） | **必要**（公開 / 管理ページ追加） |
| Cloud Run revision 更新（vrcpb-api） | **必要**（新 endpoint 追加） |
| Cloud SQL migration | **不要**（schema 変更なし） |
| Secret 追加 | **不要**（既存 R2 / DATABASE_URL / TURNSTILE で完結） |
| Cloudflare Dashboard 操作 | **不要**（DNS / Custom Domain / Turnstile 設定は既存） |
| Safari 実機確認 | **必要**（§10） |
| 検証用 publish 実行（一時 CLI） | **必要**（§9.3 案 P1） |

---

## 15. ユーザー判断事項（PR25 着手前に確定）

| 判断項目 | 推奨 | 代替 |
|---|---|---|
| display variant 配信方式 | **案 A（短命 presigned GET URL）** | 案 B（Workers proxy）→ PR41+ で再評価 |
| Viewer route | `/p/[slug]` | 変更しない方が望ましい |
| Manage route | `/manage/[photobookId]`（既存） | 同上 |
| PR25 内で最小 publish endpoint を作るか | **作らない**（既存 `PublishFromDraft` を一時 CLI で呼ぶ） | PR28 で publish flow 完成と一体で endpoint 化 |
| design-system 第一弾の範囲 | colors / typography / spacing / radius-shadow + tailwind 反映 | components.md は PR41+ で正式化 |
| MVP は noindex 継続か | **継続**（visibility=public でも noindex） | PR37 LP 公開と同時に解禁検討 |
| Cloud SQL `vrcpb-api-verify` 残置 | **PR39 まで継続** | 早めに rename したいなら PR29 と同時に検討 |
| Public repo 化 | **PR38 まで保留** | 早期公開は § security final audit を前倒し |

---

## 16. 完了条件

- 本計画書が review 通過し、PR25 の実装単位が単独 PR 1〜2 本に分解可能
- §15 のユーザー判断事項が確定
- §13 の checklist が PR25 開始時にそのまま使える状態

## 17. 次 PR への引き継ぎ事項（PR25 開始時の前提）

- §3 の HTTP ステータス表
- §5.2 の display variant 配信方式（案 A）
- §6.1 の endpoint I/F
- §7 の Frontend route 構成
- §8 の design 抽出ルール
- §9.3 の publish 検証手段（一時 CLI）
- §10 の Safari チェックリスト
- §11 の test 観点
- §12 の secret 不変条件

---

## 18. 履歴

| 日付 | 変更 |
|------|------|
| 2026-04-27 | 初版作成。PR23 完了直後、新正典 §3 PR24 の本体 |
