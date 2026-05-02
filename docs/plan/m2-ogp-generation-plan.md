# M2 OGP 自動生成 実装計画（PR33）

> **位置付け**: 本計画書は **計画書のみ**。本書段階では migration / R2 object 作成 /
> Cloud Build deploy / Workers redeploy / Cloud Run Jobs / Scheduler 作成は **行わない**。
>
> **正典関係**:
> - 上流: [`docs/design/cross-cutting/ogp-generation.md`](../design/cross-cutting/ogp-generation.md)
> - 業務知識 v4 §3.2（OGP 生成失敗でも公開は成功）/ §3.8（X 共有支援）/ §6.17（OGP の独立管理）
> - ADR-0001（Cloudflare R2 / Cloud Run / Cloudflare Workers）/ ADR-0005（storage_key 命名規則）
> - 関連: PR25 公開 Viewer / PR23 image-processor / PR30 Outbox / PR31 outbox-worker / PR32a email
>   provider（OGP は **Email Provider と独立**で進める）

---

## 0. 前提（再確認）

- 公開 Viewer `/p/[slug]` は noindex 固定（PR25、middleware + metadata 両方）
- 現状 og:image は **未出力**（`generateMetadata` で og:image を含まない、PR25 placeholder）
- R2 bucket `vrcpb-images` は **public OFF**（presigned URL 経由のみ）
- backend `image/domain/vo/storage_key` は `photobooks/{photobook_id}/ogp/{ogp_id}/{random}.png`
  prefix を既に予約（usage_kind='ogp'）
- PR30 outbox table は `photobook.published` / `image.became_available` / `image.failed`
  の 3 種のみ実体投入。CHECK 制約で他 event は弾かれる
- PR31 outbox-worker は no-op + log。Cloud Run Jobs / Scheduler は未作成（副作用が
  確定するまで稼働させない方針）
- Email Provider は ADR-0006 で再選定中。OGP は **Email と独立**

---

## 1. 目的

1. `/p/[slug]` の OGP 表示を改善し、SNS / 共有メッセージで photobook の存在感を出す
2. OGP 画像を **生成 → R2 保存 → public 配信**できる経路を確立する
3. cross-cutting/ogp-generation.md に従い `photobook_ogp_images` で状態管理を独立化
4. 失敗しても公開は成功（v4 §3.2、業務 P0-4）を維持
5. **Email Provider 確定の待ち時間を OGP 実装で埋める**

---

## 2. 範囲（PR33 全体の段階分割）

| 段階 | 内容 | 想定 PR |
|---|---|---|
| **PR33a**（本書） | 計画書（コード変更なし） | **本 PR** |
| PR33b | migration + OGP renderer + Repository + UseCase + CLI（手動 generate）+ unit test | 後続 PR、Cloud SQL migration STOP / Cloud Build deploy STOP あり |
| PR33c | OGP 配信経路（Workers proxy）+ Frontend metadata 更新 + SNS validator 確認 | 後続 PR、Workers redeploy STOP あり |
| PR33d | Outbox handler（PhotobookPublished consume）+ Cloud Run Jobs 作成（**副作用 handler を初めて稼働させるため STOP 必須**） | 後続 PR、Cloud Run Jobs 作成 STOP / Scheduler は別 STOP |
| PR33e（任意） | Reconcile（stale_ogp_enqueue / 手動 ogp_stale.sh） | 後続 PR |

**PR33a（本書）の対象**: 計画書 + 既存正典との整合確認 + 各 PR で停止ポイントを置く位置の確定。
**PR33a の対象外**: 実装 / migration / R2 / Workers / Cloud Run Jobs / Scheduler / 課金影響操作。

---

## 3. OGP 画像仕様

### 3.1 物理仕様

| 項目 | 値 | 根拠 |
|---|---|---|
| 解像度 | **1200 × 630 px**（X / Twitter 標準） | cross-cutting/ogp-generation.md §6.2 |
| 形式 | **PNG**（透過なし、フォトブックの cover が JPEG でも合成は PNG で安定） | レンダリング上の安定性、SNS は PNG/JPEG どちらも受ける |
| 圧縮 | PNG zlib level 6 程度 | バランス |
| 想定ファイルサイズ | 80〜250 KB | 1200x630 PNG の典型 |

### 3.2 表示要素（MVP）

```
┌─────────────────────────────────────────┐
│ ┌─────────┐                             │
│ │  cover  │  {photobook title}          │
│ │  image  │                             │
│ │  thumb  │  {photobook type badge}     │
│ │ 480x480 │                             │
│ │ 中央切抜│  by {creator_display_name}  │
│ └─────────┘                             │
│                                         │
│         VRC PhotoBook (logo)            │
└─────────────────────────────────────────┘
```

要素:

- **cover image**: `photobooks.cover_image_id` の `image_variants(thumbnail)` を使う。
  cover 未設定なら最初の available image。両方 nil なら fallback OGP（既定画像）に切替
- **photobook title**: 80 字制限内（domain で保証）。長い場合は 2 行で折り返し、3 行
  目以降は `…` で切る
- **type badge**: photobook_type（memory / event / world など）を日本語ラベルで表示
- **creator display name**: 50 字制限内。長い場合は `…` で切る
- **service logo / wordmark**: 「VRC PhotoBook」テキストロゴ（footer 配置）

### 3.3 fallback OGP（既定画像）

- `frontend/public/og/default.png` を 1 枚バンドル（type 別は MVP では作らず、共通 1 枚）
- 配信:
  - `pending` / `failed` / `fallback` 状態 → 既定 URL `https://app.vrc-photobook.com/og/default.png`
  - これは Workers static assets binding で配信（既存 OpenNext 経路）

### 3.4 日本語 / emoji / VRChat 名

- フォント: **Noto Sans JP**（OFL ライセンス、商用 OK）。Regular + Bold 2 ウェイトを backend image にバンドル
- emoji: フォントが対応する範囲のみレンダリング。色付き emoji（Apple Color Emoji）は
  サポートしない（grayscale / 文字化けを許容、SNS crawler の典型挙動と整合）
- VRChat 名の特殊文字（zero-width / 結合文字）: 表示できない範囲は `□`（tofu）に置換
- 折り返し: 文字単位で計算（ja の word break ルール、簡易実装で十分）

### 3.5 セキュリティ要素

- OGP 画像に **管理 URL / token / hash / storage_key 完全値を入れない**
- title / description / creator name は domain VO で長さ制限 + サニタイズ済 → そのまま埋め込み可
- XSS は OGP 画像（PNG）にはそもそも乗らないが、Frontend `<meta og:image content>` 属性は
  encodeURI と HTML escape を Next.js が行う

---

## 4. 生成方式の検討

| 案 | 概要 | 評価 |
|---|---|---|
| **A. Go image/draw + freetype** | pure Go、distroless OK、フォント静的バンドル | **採用候補（推奨）**。CGO 不要、Cloud Run 互換、cold start 速い |
| B. Headless browser / Playwright | Chromium ベースで HTML → PNG | distroless で動かない、image サイズ巨大、cold start 遅い、運用コスト高 |
| C. Satori + resvg | Node.js / Wasm で SVG → PNG | Frontend OG image API として優秀だが、backend Go との分離が増える、Wasm size + cold start |
| D. Frontend OG image endpoint（Next.js `ImageResponse`）| Next.js + Workers で生成、R2 保存は別経路 | Workers 単体ではフォント / バインディング制限、R2 PUT も追加実装 |
| E. 外部サービス（Cloudinary / Imgix 等）| URL parameter で動的生成 | 課金 + 個人 MVP では契約 / 価格不確定 |

### 推奨: **A（Go image/draw + freetype + 静的フォント）**

理由:
- Cloud Run 既存 image（distroless static + nonroot + CGO_ENABLED=0）にそのまま乗る
- フォントは Noto Sans JP の 2 ウェイトを `backend/internal/ogp/fonts/` に embed（go:embed）
- 既存 image-processor（PR23）の `disintegration/imaging` と同じレイヤーで動く
- pure Go なので test が書きやすい
- 失敗時は既定 OGP（fallback）にフォールバック → 公開は成功

### 補完: B / C を Phase 2 で再評価

- Phase 2（ローンチ後）に Frontend Workers での `ImageResponse` を再評価し、
  Cloud Run コスト削減を検討（Workers cost = ほぼ無料、Cloud Run cost = active 時のみ）
- ただし PR33 では **A で確定** し、Phase 2 で必要なら EmailSender 同様に
  **renderer ポート抽象化**で差し替える

---

## 5. R2 保存 + public 配信方針（**最重要決定**）

### 5.1 問題

SNS crawler（X / Discord / Slack / LINE）は OGP 画像 URL に対して以下を **使えない**:

- 認証 Cookie / Bearer header
- 短命 presigned URL（crawler 起動時に expire 済の可能性）
- JavaScript / SPA hydration

**public access 可能な URL** が必要。一方、R2 bucket `vrcpb-images` は **public OFF** で運用中。

### 5.2 配信候補比較

| 案 | 概要 | 評価 |
|---|---|---|
| **A. Cloudflare Workers proxy** | Frontend `vrcpb-frontend` Worker に R2 binding を追加し、`/ogp/<photobook_id>` route で R2 オブジェクトをそのまま return | **採用候補（推奨）**。R2 public OFF を維持、egress 無料、Cloudflare 同居で低レイテンシ、CDN cache に乗る |
| B. Backend Cloud Run endpoint | Cloud Run の `/api/public/photobooks/{slug}/og` で R2 GetObject → stream | Cloud Run 課金（リクエスト + CPU + egress R2→Cloud Run→User）、cold start、egress 二重 |
| C. R2 public bucket（prefix 限定） | `vrcpb-images` 全体を public 化 → `ogp/` prefix 以外は別 bucket に分離 | bucket 全体 public のリスク高、image / variant 等を public 化したくない |
| D. R2 public 第二 bucket（OGP 専用） | `vrcpb-ogp` を新設、public ON、`vrcpb-images` は OFF 維持 | bucket 設定追加、cross-bucket コピーが発生、運用が複雑 |
| E. 長命 presigned URL（1 年）| publish 時に 1 年有効な presigned を発行 | R2 access key 失効時に全 URL 無効、SNS 側 cache の鮮度問題、URL 形式に署名情報露出 |

### 5.3 推奨: **A（Cloudflare Workers proxy）**

#### 設計

```
SNS crawler
   │ GET https://app.vrc-photobook.com/ogp/<photobook_id>?v=<version>
   ▼
Cloudflare Workers (vrcpb-frontend)
   │ R2 binding（OGP_BUCKET）
   │ key = photobooks/<photobook_id>/ogp/<ogp_id>/<random>.png
   │ DB lookup は不要（key を Frontend route で組み立てる、
   │   または Backend に最新 key を問い合わせる）
   ▼
Cloudflare R2（public OFF 維持）
   │ オブジェクト返却（egress 無料）
   ▼
Workers レスポンス
   - Content-Type: image/png
   - Cache-Control: public, max-age=86400, s-maxage=86400
   - X-Robots-Tag: noindex, nofollow（OGP 画像自体）
```

#### key の解決

OGP key を Frontend Workers が知る必要がある。2 案:

- **案 A1**: Frontend Workers が Backend `/api/public/photobooks/<photobook_id>/ogp` を
  fetch して最新 key + version を取得 → R2 GetObject。Backend が DB lookup
- **案 A2**: storage_key は publish 時に確定するため、`{photobook_id}/<latest>` の
  semantic latest key を R2 上で「コピー」or「list 最新」で取得

**推奨: A1**（Backend 経由）。Frontend Workers が R2 と Backend を直接呼ぶ ことで
Cloudflare CDN cache に乗せやすく、key 更新時（OGP 再生成）の整合は Backend の
DB トランザクションで担保。Backend endpoint は **公開可能な情報のみ**（key + version、
内容は OGP 画像であり管理 URL を含まない）を返す。

#### Cache 戦略

- Frontend Workers が `Cache-Control: public, max-age=86400` を設定
- Photobook 更新 → `version++` → URL に `?v=<version>` クエリ → SNS 側で再 crawl
- Cloudflare CDN cache は version クエリで分離

#### 失敗時

- Backend lookup で `status != generated` → Workers は **default OGP**
  （`https://app.vrc-photobook.com/og/default.png` の static asset）に 302 redirect
- R2 GetObject 失敗 → 同上の fallback

### 5.4 R2 bucket 構成

| bucket | 状態 | 用途 |
|---|---|---|
| `vrcpb-images` | **public OFF 維持** | original / display / thumbnail / **ogp** すべてを格納 |

**OGP 専用 bucket は作らない**。`photobooks/<photobook_id>/ogp/...` prefix を Workers
proxy 経由で公開する形で、bucket 全体の公開設定は不要。

### 5.5 Workers binding 追加（PR33c で実施）

```jsonc
// frontend/wrangler.jsonc 追加
{
  "r2_buckets": [
    {
      "binding": "OGP_BUCKET",
      "bucket_name": "vrcpb-images"
    }
  ]
}
```

Workers Secret は不要（R2 binding は IAM で完結）。

---

## 6. DB 設計

### 6.1 採用: 既存設計そのまま（cross-cutting/ogp-generation.md §3）

`photobook_ogp_images` 単独 table を新規 migration で追加。Photobook 集約には ogp 列は
追加しない（v4 §6.17 / cross-cutting §3.3）。

### 6.2 migration 草案（PR33b 範囲）

```sql
-- migrations/00013_create_photobook_ogp_images.sql
CREATE TABLE photobook_ogp_images (
    id              uuid        NOT NULL DEFAULT gen_random_uuid(),
    photobook_id    uuid        NOT NULL,
    status          text        NOT NULL DEFAULT 'pending',
    image_id        uuid        NULL,
    version         int         NOT NULL DEFAULT 1,
    generated_at    timestamptz NULL,
    failed_at       timestamptz NULL,
    failure_reason  text        NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT photobook_ogp_images_pk PRIMARY KEY (id),
    CONSTRAINT photobook_ogp_images_photobook_unique UNIQUE (photobook_id),
    CONSTRAINT photobook_ogp_images_status_check
        CHECK (status IN ('pending', 'generated', 'failed', 'fallback', 'stale')),
    CONSTRAINT photobook_ogp_images_version_check
        CHECK (version >= 1),
    CONSTRAINT photobook_ogp_images_failure_reason_len_check
        CHECK (failure_reason IS NULL OR char_length(failure_reason) <= 200),
    CONSTRAINT photobook_ogp_images_photobook_fk
        FOREIGN KEY (photobook_id) REFERENCES photobooks(id) ON DELETE CASCADE,
    CONSTRAINT photobook_ogp_images_image_fk
        FOREIGN KEY (image_id) REFERENCES images(id) ON DELETE SET NULL
);

CREATE INDEX photobook_ogp_images_status_updated_idx
    ON photobook_ogp_images (status, updated_at);
```

### 6.3 OGP 画像実体は Image 集約に格納

- `images.usage_kind = 'ogp'`（既存 VO）
- storage_key prefix `photobooks/{photobook_id}/ogp/{ogp_id}/{random}.png`（既存 VO）
- `image_variants` には OGP variant を 1 行（1200×630）作成。display / thumbnail は不要

### 6.4 状態と Photobook ライフサイクル

| Photobook 状態 | OGP 配信 |
|---|---|
| draft | OGP 生成しない（OGP row も作らない） |
| published | OGP row INSERT、生成キュー投入 |
| published + hidden_by_operator | **OGP 配信停止**（Workers proxy が hidden 状態を見て fallback or 410） |
| deleted（softDelete） | 同上、配信停止 |
| purged | photobook_ogp_images CASCADE で削除、Image 集約も孤児 GC |

---

## 7. 生成タイミング

### 7.1 トリガ候補

| トリガ | 必要性（MVP） | 実装 PR |
|---|---|---|
| Publish（draft → published）| **必須** | PR33d で `photobook.published` event consume |
| Cover 変更（PR27 SetCoverImage / ClearCoverImage） | 必須 | PR33d で別 event 追加 or stale 化 |
| Title / description 変更 | 推奨 | PR33d 以降 |
| Image became_available（cover 候補更新）| 任意 | PR33d 以降、必要なら |
| Manual regenerate（ops CLI / 運営判断）| 必須 | PR33b で CLI 提供 |
| Reconcile（stale 検出）| 後続 | PR33e（任意） |

### 7.2 PR33b では **CLI manual generate のみ**

- `cmd/ogp-generator --photobook-id <uuid>` で 1 件生成
- DB から photobook + cover image variant URL を fetch → 画像合成 → R2 PUT →
  `photobook_ogp_images` UPSERT（status='generated', image_id, version++）
- 失敗時は status='failed' / failure_reason 記録
- **Outbox event の consume はしない**（PR31 と同じく副作用 handler は未稼働）

### 7.3 PR33d で Outbox 連携 + Cloud Run Jobs

- `outbox/internal/usecase/handlers/photobook_published.go` を `no-op + log` から
  **OGP 生成 trigger** に変更（または新規 handler 追加）
- ただし PR31 で「副作用 handler が無いままに pending event を consume すると
  不整合状態になる」と判断したため、**PR33d で初めて Cloud Run Jobs を稼働させる**
  時点で停止ポイントを置く
- migration で event_type CHECK は **緩めない**（PR30 で投入済の `photobook.published`
  をそのまま OGP handler に渡す。`photobook.cover_changed` 等を増やすなら別 PR）

### 7.4 失敗時の挙動

cross-cutting/ogp-generation.md §8 と整合:

1. 生成失敗 → `status=failed`, `failure_reason` 記録（200 char 上限、Secret redact）
2. 既定 OGP 配信（Workers proxy が status を見て default に redirect）
3. 公開自体は成功（v4 §3.2）
4. 30 日経過した failed → `fallback` 確定（Reconcile、PR33e）

---

## 8. Outbox との関係

### 8.1 既存 outbox の扱い

PR30 で投入した 3 種 event:

| event_type | OGP との関係 |
|---|---|
| `photobook.published` | **OGP 生成トリガ**（PR33d で handler 化） |
| `image.became_available` | cover 候補が更新されうるが、PR33 では **無視**（cover_image_id 未設定の場合のみ別フロー、後続検討） |
| `image.failed` | OGP には影響なし（fallback で対応） |

### 8.2 PR31 で止めた判断との整合

PR31 で「副作用 handler が無い状態で Cloud Run Jobs を稼働させない」と決めた。
PR33d で初めて副作用 handler（OGP 生成）が入るため:

- **PR33d 着手前に PR30 deploy 以降に積み上がった pending event の扱いを判断**
- 候補:
  - 全件「実害なし」と判断して通常 consume させる（OGP 生成は全件実施、過去 photobook も新 OGP に切り替わる）
  - 過去 event を一括で processed に進める SQL を流す（OGP 生成は cover 変更時のみ）
  - 過去 event を pending のまま放置し、新規 event のみ consume する（要 SQL で条件 fix）
- **採用候補**: 1 番（過去分も含めて OGP 生成）。理由は OGP 未生成のまま公開されている
  photobook 群を一括バックフィルできるため

### 8.3 PR31 worker への影響（PR33d で更新）

PR31 worker の handler dispatch は registry pattern なので、`handlers/photobook_published.go`
を no-op から OGP 生成 handler に置き換えるだけで済む。`image.became_available` / `image.failed`
は引き続き no-op（または OGP 関連 trigger を追加）。

### 8.4 新規 event は **PR33 では追加しない**

- `photobook.cover_changed` / `photobook.updated` 等の追加は migration で event_type
  CHECK 緩和が必要。PR33 では行わず、後続で必要に応じて
- 代わりに **Photobook UseCase（SetCoverImage 等）が同 TX で OGP `status=stale, version++`
  を実行** → CLI / Reconcile で再生成する設計を取る（cross-cutting/ogp-generation.md §5.2）

---

## 9. API / CLI

### 9.1 PR33b 範囲

- `cmd/ogp-generator/main.go`（CLI）
  - `--photobook-id <uuid>` 1 件生成
  - `--all-pending`（status='pending' / 'stale' を順次処理）
  - `--max-events` / `--timeout` / `--dry-run`
  - PR31 outbox-worker と同じ flag 体系を踏襲
- `internal/ogp/` パッケージ
  - `domain/`（OGP entity / VO / status）
  - `infrastructure/repository/rdb/`（photobook_ogp_images）
  - `infrastructure/renderer/`（image/draw + freetype + go:embed フォント）
  - `internal/usecase/`（GenerateOgp / FindByPhotobookID / RecordFailure）
- 新規 sqlc set: `internal/ogp/infrastructure/repository/rdb/queries/ogp.sql`
- フォント: `backend/internal/ogp/fonts/NotoSansJP-Regular.ttf` / `Bold.ttf` を `go:embed`
- 画像合成 lib: `disintegration/imaging`（既存 image-processor で利用中）+ Go 標準 `image/draw`

### 9.2 admin-only HTTP endpoint は **作らない**

- 内部から CLI で叩く運用（PR23 image-processor と同方針）
- 必要なら ops CLI として `cmd/ops/photobook_regenerate_ogp` を後続で追加

### 9.3 Backend public endpoint（Workers proxy 用）

- `GET /api/public/photobooks/<photobook_id>/ogp` を新設（PR33c）
- レスポンス: `{ "status": "generated|fallback|...", "image_url_path": "/ogp/<photobook_id>?v=<n>", "version": <n> }`
- 管理 URL / token は **含めない**
- noindex / public read 可

---

## 10. Frontend metadata 連携

### 10.1 PR33c で `generateMetadata` を更新

```ts
// frontend/app/(public)/p/[slug]/page.tsx
export async function generateMetadata({ params }: { params: Params }): Promise<Metadata> {
  const { slug } = await params;
  let pb: PublicPhotobook | null = null;
  try {
    pb = await fetchPublicPhotobook(slug);
  } catch { pb = null; }

  const title = pb?.title ?? "VRC PhotoBook";
  const description = pb?.description ?? "VRC PhotoBook（非公式ファンメイドサービス）";
  const ogImageUrl = pb?.ogp ? `${BASE}${pb.ogp.imageUrlPath}` : `${BASE}/og/default.png`;
  const publicUrl = pb ? `${BASE}/p/${pb.slug}` : `${BASE}/`;

  return {
    title,
    description,
    robots: { index: false, follow: false }, // MVP は noindex 継続
    openGraph: {
      title, description, type: "website",
      url: publicUrl,
      images: [{ url: ogImageUrl, width: 1200, height: 630 }],
    },
    twitter: {
      card: "summary_large_image",
      title, description,
      images: [ogImageUrl],
    },
  };
}
```

### 10.2 noindex と OGP の両立

- `<meta name="robots" content="noindex,nofollow">` は **検索エンジン向け**
- OGP は **SNS preview 向け**（X / Discord / Slack / LINE）
- 両者は独立で、**両立可能**（noindex でも OGP は出る）
- middleware の `x-robots-tag: noindex, nofollow` も同じく検索向け

### 10.3 OGP 画像 URL の絶対 URL

- `https://app.vrc-photobook.com/ogp/<photobook_id>?v=<n>`
- Workers proxy 経由（§5.3）

### 10.4 stale URL の扱い

- Photobook 更新 → `version++` → URL に新 query → SNS 側で再 crawl
- Cloudflare CDN cache は version クエリ単位で別キー

---

## 11. Security 方針

### 11.1 OGP 画像に出さない

- 管理 URL / draft URL / token / hash / Cookie / R2 credentials / DATABASE_URL
- storage_key 完全値（OGP 画像内には出ないが、metadata の URL 形式にも含めない）

### 11.2 Photobook 状態に応じた配信制御

| Photobook 状態 | OGP 配信 |
|---|---|
| draft | **配信しない**（OGP row 自体作らない、Backend lookup で 404） |
| published（visibility=public）| 配信 |
| published + hidden_by_operator | **配信停止**（Backend lookup で fallback or 404）|
| deleted | **配信停止** |
| purged | row 削除済、404 |

### 11.3 Backend lookup endpoint の権限

- `/api/public/photobooks/<photobook_id>/ogp` は public read（no Cookie）
- 管理 URL を含まない情報のみ返す
- `photobook_id` は public 識別子（業務知識 v4 §3.5）

### 11.4 OGP renderer ログ

- title / description / creator name はログに出してよい（既に画面表示される情報）
- storage_key は出さない、cover image の **SHA-256 prefix だけ** debug log に出す（任意）
- failure_reason は sanitize（200 char、Secret パターン redact、PR31 worker と同方式）

### 11.5 Workers proxy ログ

- crawler UA / request path / response status のみ
- R2 binding の credentials はログに残らない（binding なので key 値が無い）

### 11.6 XSS

- `og:title` / `og:description` の `<meta content>` 属性は Next.js が HTML escape 済
- title / description の本文サニタイズは Photobook domain VO で完了

---

## 12. Safari / SNS 確認方針

### 12.1 必須確認（PR33c / PR33d 完了時）

| 観点 | 方法 |
|---|---|
| `<head>` に og:title / og:description / og:image / og:url / twitter:card が出る | curl + grep |
| og:image URL が public で 200 OK + image/png Content-Type を返す | curl `https://app.vrc-photobook.com/ogp/<id>?v=1` |
| Cache-Control が `public, max-age=86400` | curl -I |
| version クエリで CDN cache が分離する | curl `?v=1` / `?v=2` で異なる Cf-Cache-Status |
| draft / hidden / deleted は OGP を返さない（fallback or 404） | curl 各状態 |

### 12.2 SNS validator（PR33c / PR33d 完了時）

| サービス | 確認方法 |
|---|---|
| **X (Twitter) Card Validator** | <https://cards-dev.twitter.com/validator> または `curl -A "Twitterbot/1.0"` |
| **Discord** | 投稿してプレビュー確認 |
| **Slack** | 投稿してプレビュー確認 |
| **LINE** | LINE 共有でプレビュー確認 |
| **OGP デバッガ（Open Graph Object Debugger 系）** | 第三者 validator |

### 12.3 Safari 実機確認

`.agents/rules/safari-verification.md` に従い:

- macOS Safari でメッセージ App / メールに URL を貼り付けて preview を確認
- iPhone Safari で同様 + iMessage / LINE / X アプリでの preview
- **OGP 変更は SSR HTML の `<meta>` 出力に影響するため Safari 確認必須**

### 12.4 PR33a（本書）の確認範囲

- 計画書のみ。実機確認は PR33b 以降の各 PR で実施
- 本書では確認項目のリストアップのみ

---

## 13. Test 方針

### 13.1 PR33b（renderer + UseCase + CLI）

| 対象 | テスト |
|---|---|
| renderer | title 80 文字 / 50 文字 creator / 日本語 / 折り返し / cover あり / cover なし fallback / image_variants thumbnail URL fetch fail / フォント load 成功 |
| Repository | UPSERT / status 遷移 / version++ / failure_reason 200 char / FK ON DELETE CASCADE |
| UseCase | publish 後 OGP 生成 / 既存 row が stale なら version++ / 失敗時 status=failed |
| CLI | --photobook-id / --all-pending / --dry-run / unknown id |
| Secret grep | rendered PNG / failure_reason / log に管理 URL / token / storage_key 完全値が出ない |

### 13.2 PR33c（配信経路）

| 対象 | テスト |
|---|---|
| Backend `/api/public/photobooks/<id>/ogp` | published / hidden / deleted / draft / unknown |
| Frontend `generateMetadata` | og:image URL 生成 / fallback URL / version クエリ |
| Workers proxy | R2 hit / miss / status != generated → fallback redirect |

### 13.3 PR33d（outbox handler）

| 対象 | テスト |
|---|---|
| handler dispatch | photobook.published consume → OGP 生成呼び出し |
| failure | OGP 生成失敗 → outbox MarkFailedRetry / MarkDead 経路 |
| pending event バックフィル | 過去 event の consume で OGP が生成される |

---

## 14. 実リソース操作（PR ごとの停止ポイント）

| PR | 停止ポイント | 内容 |
|---|---|---|
| PR33a（本書）| **なし** | 計画書のみ |
| PR33b | **STOP α**: Cloud SQL に migration 00013 を適用 | goose up（PR30 と同手順） |
| PR33b | **STOP β**: Cloud Build manual submit deploy（cmd/ogp-generator binary 同梱、cloudbuild.yaml の `traffic-to-latest` 通過確認） | image 更新、Cloud Run service 動作不変 |
| PR33b | （ローカル CLI で 1 件 generate して R2 に PUT する PoC は **STOP γ**）| R2 object 作成、課金影響あり（egress 0、storage 微小） |
| PR33c | **STOP δ**: Workers redeploy（R2 binding 追加、wrangler.jsonc 変更） | binding 追加 → 既存 frontend 動作確認 |
| PR33c | **STOP ε**: Backend Cloud Build deploy（`/api/public/photobooks/<id>/ogp` endpoint 追加） | 同 |
| PR33d | **STOP ζ**: Cloud Run Jobs 作成（outbox-worker）+ 過去 pending event の consume 戦略確定 | **副作用 handler の初回稼働、最重要 STOP** |
| PR33d | **STOP η**: Cloud Scheduler 作成（cadence は image-processor 1 min と整合させるか別 interval にするかは運用観測後判断） | 自動定期実行、別 STOP。outbox 用の Scheduler はまだ未作成（手動 Job execute 運用中、STOP α P0 v2 時点） |
| PR33e | **STOP θ**: Reconcile cron（stale_ogp_enqueue） | 自動 reconciler |

各 STOP はユーザー判断項目（§15）と組で承認する想定。

---

## 15. ユーザー判断事項

PR33a 完了時点でユーザーに判断を仰ぐ項目:

| 番号 | 判断項目 | 推奨 | 理由 |
|---|---|---|---|
| Q1 | OGP 画像生成方式 | **A: Go image/draw + freetype + 静的フォント** | distroless / Cloud Run / pure Go の整合 |
| Q2 | OGP 画像 public 配信経路 | **A: Cloudflare Workers proxy** | R2 public OFF 維持、egress 無料、CDN cache |
| Q3 | OGP 専用 R2 bucket を作るか | **作らない**（既存 vrcpb-images の prefix を使う） | bucket 数を増やさない、ADR-0005 命名規則と整合 |
| Q4 | DB 設計 | `photobook_ogp_images` 単独 table | cross-cutting/ogp-generation.md 既存設計どおり |
| Q5 | PR33b で何を作るか | renderer + Repository + UseCase + CLI（手動 generate） | 外部副作用なしで PoC 可能 |
| Q6 | PR33c で Workers binding を追加するか | **追加する**（R2 binding） | OGP 配信に必須 |
| Q7 | PR33d で Cloud Run Jobs を作るか | **STOP 後に判断**（副作用 handler 初回稼働、過去 pending event の処理戦略確定が前提） | 不整合リスクの最小化 |
| Q8 | PR33d で Scheduler を作るか | **PR33d で作らず、別 STOP**（手動 invoke で動作確認後） | 段階導入 |
| Q9 | 過去 pending event をどう扱うか | **全件 consume**（過去 photobook も OGP 生成） | バックフィル効果 |
| Q10 | noindex を OGP 反映時に解除するか | **MVP は継続 noindex**（PR37 LP 公開と同時に検討） | 公開状態の安定確認後 |
| Q11 | fallback OGP の type 別バリエーション | **MVP は共通 1 枚**（type 別は Phase 2） | 工数最小化 |
| Q12 | 日本語フォント | **Noto Sans JP**（OFL、商用 OK、Regular + Bold 2 ウェイト） | ライセンス安全 / 視認性 |
| Q13 | OGP 画像 cache TTL | **86400s（24h）+ version クエリ** | crawler 更新と CDN cache の両立 |
| Q14 | Safari / SNS validator 確認範囲 | macOS Safari / iPhone Safari / X Card Validator / Discord / Slack（必須）+ LINE（任意） | 主要流入経路 |
| Q15 | OGP 関連 migration の分割 | 1 migration（00013）で table + index を一括 | 単純化 |

---

## 16. 関連計画 / 設計への影響

### 16.1 更新が必要な docs（PR33a で実施）

- `docs/plan/vrc-photobook-final-roadmap.md` §3 PR33: 段階分割（PR33a/b/c/d/e）と本書参照を反映
- `docs/plan/m2-outbox-plan.md`: Cloud Run Jobs / Scheduler の作成タイミングを「PR33d で副作用 handler と組」に明記

### 16.2 後続 PR で更新が必要な docs

- PR33b: `docs/runbook/backend-deploy.md`（cmd/ogp-generator の実行手順追記）
- PR33c: `docs/runbook/` に Workers proxy / R2 binding の運用 note
- PR33d: 同 + Cloud Run Jobs / Scheduler の作成手順

---

## 17. PR closeout 適用（pr-closeout.md §6）

PR33a 完了報告に以下のチェックリストを含める。

- [ ] コメント整合チェック実施: `bash scripts/check-stale-comments.sh --extra "OGP|og:image|twitter:card|SNS|crawler|R2 public|public URL|Cloud Run Jobs|Scheduler"`
- [ ] 古いコメントを修正したか
- [ ] 残した TODO とその理由（4 区分）
- [ ] 先送り事項がロードマップに記録済み（PR33b/c/d/e）
- [ ] generated file 未反映コメント: 該当なし（コード変更なし）
- [ ] Secret 漏洩 grep（実値 0 件、用語記述のみ）

---

## 18. 履歴

| 日付 | 変更 |
|------|------|
| 2026-04-28 | 初版（PR33a）。OGP 自動生成の段階分割（renderer / 配信 / Outbox / Reconcile）と公開配信経路（Cloudflare Workers proxy 推奨）を確定。R2 public OFF 維持、Cloud Run Jobs 作成は副作用 handler 初回稼働と組で停止ポイント |
| 2026-04-28 | PR33b 完了。renderer + Repository + UseCase + CLI + Dockerfile 同梱 + Cloud SQL migration 13 適用 + Cloud Build deploy（vrcpb-api-00014-9sk）+ ローカル CLI で R2 PUT PoC + cleanup |
| 2026-04-28 | PR33c 完了（一部）。Backend `/api/public/photobooks/<id>/ogp` endpoint + GenerateOgp 完了化（images/image_variants + MarkGenerated）+ Frontend metadata + Workers R2 binding + `/ogp/<id>` proxy route + default OGP placeholder 実装 + STOP δ (Workers redeploy) / ε (Backend deploy vrcpb-api-00015-j8t) 完了。**STOP ζ はスキップ**（published+visibility=public な photobook が本番 DB に 0 件、unlisted 強制公開は不採用、テスト photobook 新規作成は PR33c 範囲外と判断）。**generated OGP の public 配信実機確認 / SNS validator / Safari 実機確認は PR33d 持ち越し** |
