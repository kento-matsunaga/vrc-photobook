# 作成導線追加 PR 計画書（m2-create-entry）

> 作成: 2026-05-01
> 状態: **STOP α（設計判断資料）**、ユーザー承認待ち
> 起点: PR37 design rebuild final closeout `c906030` 完了。LP に作成導線が無いことが本番運用上の致命的ギャップとして user 指摘
> 関連 docs: 業務知識 v4 §3.1 / §3.7 / §6.13、[`docs/plan/vrc-photobook-final-roadmap.md`](./vrc-photobook-final-roadmap.md)、[`docs/plan/m2-usage-limit-plan.md`](./m2-usage-limit-plan.md)、[`docs/runbook/usage-limit.md`](../runbook/usage-limit.md)、[`docs/runbook/backend-deploy.md`](../runbook/backend-deploy.md)
> 関連 rules: [`.agents/rules/security-guard.md`](../../.agents/rules/security-guard.md) / [`.agents/rules/safari-verification.md`](../../.agents/rules/safari-verification.md) / [`.agents/rules/turnstile-defensive-guard.md`](../../.agents/rules/turnstile-defensive-guard.md)
> 関連 failure-log: [`harness/failure-log/2026-05-01_pr37-public-pages-design-mismatch.md`](../../harness/failure-log/2026-05-01_pr37-public-pages-design-mismatch.md)

## 1. 現状の実装棚卸し

### 1.1 揃っている資産

| 工程 | 場所 | 状態 |
|---|---|---|
| `CreateDraftPhotobook` UseCase | `backend/internal/photobook/internal/usecase/create_draft_photobook.go` | **完成**（type / layout / opening_style / visibility / creator_display_name / rights_agreed / draft TTL を引数、`Output{Photobook, RawDraftToken}` 返却）|
| `PhotobookRepository.CreateDraft` | `backend/internal/photobook/infrastructure/repository/rdb/photobook_repository.go:52` | **完成**（sqlc `CreateDraftPhotobook`）|
| `draft_edit_token` 発行 + SHA-256 hash | UseCase 内、`vo/draft_edit_token` / `vo/draft_edit_token_hash` | **完成** |
| `POST /api/auth/draft-session-exchange` | `backend/internal/photobook/interface/http/handler.go:97` | **完成**（raw token → session_token + Set-Cookie 用情報）|
| `/draft/[token]` Route Handler | `frontend/app/(draft)/draft/[token]/route.ts` | **完成**（token → session 交換 → `/edit/<id>` redirect、Set-Cookie + Cache-Control: no-store）|
| `/edit/[photobookId]` 編集 UI | `frontend/app/(draft)/edit/[photobookId]/page.tsx` + `EditClient.tsx` | **完成** |
| publish / complete / `/manage/[photobookId]` | 既存 | **完成** |
| Turnstile L0-L4 ガード（Backend / Frontend）| 既存 ReportForm / Upload で確立 | **完成** |
| UsageLimit（report.submit / upload_verification.issue / publish.from_draft）| 既存 PR36 | **完成** |

### 1.2 抜けている部分（本 PR で埋める対象）

| 抜け | 影響 |
|---|---|
| **作成 endpoint（HTTP）**: `POST /api/photobooks` 等で `CreateDraftPhotobook` UseCase を公開する handler | LP からの作成導線が成立しない |
| **作成 Frontend route**: `/create`（or 等価） + 作成ページ UI | 同上 |
| **Frontend API client**: `lib/createPhotobook.ts`（POST → response.draft_edit_token を URL 化して redirect） | 同上 |
| **LP CTA 差替**: 「今すぐ作る → /create」を Primary に | LP の主目的が不在 |
| **UsageLimit 連動の判断**: `photobook.create` action を新設するか、`publish.from_draft` で間接抑止のみとするか | 採用方針による |
| **Turnstile 配線判断**: `/create` 段階で Turnstile を要求するか、後段（upload / publish）に任せるか | bot 経由の draft 行スパム / abuse 抑止 |

### 1.3 業務知識 v4 §3.1 との重要な不整合（要 user 判断、§4.1 参照）

業務知識 v4 §3.1（フォトブック作成機能）に明記:
- 「**初回画像アップロード時に server draft Photobook を作成し、`draft_edit_token` を発行する**」
- 「**draft 編集 URL（`/draft/{draft_edit_token}`）を作成者に提示する**」

つまり業務知識上は「**先に画像アップロードがあって、その時点で初めて draft 行が DB に出来る**」という設計フロー。しかし現実装は:

- `CreateDraftPhotobook` UseCase は **画像なしで draft 行を作る前提**（UseCase 引数に画像が無い）
- 既存 upload-verification 経路は **draft 既存前提**（`/api/photobooks/{id}/upload-verifications` のように `{id}` をパスに含む）

→ 「LP → /create で type 選択 → POST /api/photobooks で draft 作成 → /draft/<token> → /edit」という素直な経路は、**既存実装が既にサポートしているのに、業務知識 §3.1 の文言とはやや不整合**。

候補:
- **§3.1 の「初回画像アップロード時」を「type 選択時」に改定する**（業務知識 v5 候補、§4.1 案 W）
- **業務知識通りに upload-intent ハンドラを改造して「draft 作成 + 画像アップロード」を atomic 化する**（§4.1 案 X、規模大）

本書では §4.1 で 4 案を比較し、推奨を提示するが、**最終判断はユーザー**。

## 2. 用語と redact 規則

- raw `draft_edit_token` / raw `manage_url_token` / raw `session_token` / Cookie 値 / Secret / `source_ip_hash` 完全値 / `scope_hash` 完全値 は **chat / commit / docs / work-log / failure-log に書かない**
- raw photobook_id（UUIDv7 全長 36 文字）も同様。記録時は先頭 8 文字 prefix + `<redacted>` の形式（PR36 / PR37 と同流儀）
- `<user-local VRChat photo folder>` のような redact ラベルで参照
- API 設計時は「response body に raw token を含むが、Frontend 経由で 1 度だけ使い捨て、以降 DB の SHA-256 hash のみで認可」を厳守

## 3. 導線案 A / B / C / D 比較

すべて「LP の Primary CTA から `CreateDraftPhotobook` UseCase 経路に到達する」を共通とし、**入口 UX** の差を比較する。

### 案 A — 専用ページ `/create`

- LP「今すぐ作る」→ `/create`（type 選択 + title 任意 + Turnstile + 「編集を始める」）→ POST → /draft/<token>
- prototype `screens-a.jsx CreateStart` 直系

| 観点 | 評価 |
|---|---|
| UX | 良。type 選択を 1 画面で完結、ユーザーの意図確認に余裕 |
| 実装規模 | 中（page + API client + route handler または server action）|
| token / URL / Cookie リスク | 低。raw token は POST response → 即座 redirect → window.location 制御 |
| Safari リスク | 低。LP / About / Terms / Privacy と同じ静的層 |
| Turnstile との相性 | 良（後述 §5）|
| rollback のしやすさ | 容易（page + API endpoint の 1 commit revert）|
| prototype との整合 | 高（CreateStart 直接踏襲）|

### 案 B — LP インライン作成パネル

- LP の hero 内に inline で type 選択 + Turnstile を出し、submit 後 /draft/<token>

| 観点 | 評価 |
|---|---|
| UX | 良。クリック 1 回少ない |
| 実装規模 | 中（LP に動的 state + Turnstile を埋め込む、SSR と client component 境界が複雑化）|
| token / URL / Cookie リスク | 低 |
| Safari リスク | 中（LP がより重くなる、Turnstile widget が hero 領域に常設）|
| Turnstile との相性 | 中（LP 訪問者全員に Turnstile 渡す、UX ノイズ増）|
| rollback のしやすさ | 中（LP 改修 + 新 endpoint の 2 commit に切れる）|
| prototype との整合 | 低（prototype は専用画面）|

### 案 C — type 選択省略 + LP 直 POST

- LP「今すぐ作る」を Server Action にして、default type=memory / layout=simple / opening_style=light で即作成 → /draft/<token>

| 観点 | 評価 |
|---|---|
| UX | 速い反面、type 選択ができず「自由に始める」前提になる（業務知識 §3.1「タイプに応じたテンプレートを提示し選択を受け付ける」と矛盾、要改定）|
| 実装規模 | 小（LP + Server Action + 新 endpoint のみ）|
| token / URL / Cookie リスク | 低 |
| Safari リスク | 低 |
| Turnstile との相性 | 低（Turnstile を出すと LP に widget を埋める必要、案 B と同じ問題）|
| rollback のしやすさ | 容易 |
| prototype との整合 | 低（type 選択画面がスキップされる）|

### 案 D — 別 route 名（`/start` / `/new`）+ 案 A の構成

- 案 A と内容は同じだが、route 名を `/create` ではなく `/start`（始める）/`/new`（新規）にする

| 観点 | 評価 |
|---|---|
| UX | 案 A と同等 |
| 実装規模 | 案 A と同等 |
| その他 | 案 A と同等。`/start` は SaaS 系で慣用、`/new` は GitHub 系で慣用、`/create` は最も明示的 |

→ **route 名は §13 ユーザー判断 #2 で確定**。

### 比較サマリ

| 案 | 推奨度 |
|---|---|
| **A `/create` 専用ページ** | **★★★ 推奨**（prototype 直系、Turnstile を入口に置きやすい、rollback 容易、業務知識 §3.1 の type 選択方針と整合）|
| B LP inline | ★★（Turnstile を LP に出す UX ノイズ）|
| C type 選択省略 | ★（業務知識との矛盾、UX 多様性が消える）|
| D 別 route 名 | 案 A と同等（route 名のみ user 判断）|

## 4. 業務知識 §3.1 との整合（最重要、要 user 判断）

### 4.1 4 案の比較

#### 案 W（推奨候補）— 業務知識を改定し、`/create` で先 draft 作成

- 業務知識 v4 §3.1 の「初回画像アップロード時に server draft Photobook を作成し」を「**type 選択時に server draft Photobook を作成し**」に改定
- 改定の根拠: 現実装が既に `CreateDraftPhotobook` UseCase + `/api/photobooks/{id}/upload-verifications` の 2 段階を採用しており、初回 upload-intent で draft 作成という設計は **未実装 + 既存パターンとの統合が複雑**
- 業務知識 §3.1 の他項目（draft session 交換 / draft 編集 URL 提示 / draft 延長ルール / draft expiry）は全て不変

#### 案 X — 業務知識通り、upload-intent で draft 作成

- 既存 `/api/photobooks/{id}/upload-verifications` を `/api/photobooks/upload-verifications`（`{id}` 不要）に分岐改造、未確定 photobook 用の特殊モードを実装
- Turnstile / UsageLimit / draft 作成 / 画像アップロード を atomic に処理する大規模改造
- 既存 upload UseCase / handler / wireup / FE `lib/upload.ts` 全てに影響、規模大

#### 案 Y — 折衷: `/create` は client-only 状態で type 選択し、初回画像 upload-intent で draft 作成

- Frontend `/create` は React state で type を保持し、ユーザーが「最初の写真を選ぶ」UI に遷移
- 画像選択時に POST /api/upload-intent（draft 無し用）→ Backend が draft 作成 + image upload → response で `draft_edit_token` 返却
- 業務知識 §3.1 文言改定は不要だが、案 X と同等の Backend 改造規模

#### 案 Z — `/create` を削除し、LP / About にも作成導線を出さない

- 現状維持（業務知識上の "初回画像アップロード" は将来実装、MVP では作成不可）
- ただし **LP として失格** という user 指摘の根本問題は解消されない

### 4.2 比較表

| 案 | 業務知識改定 | Backend 改造規模 | Frontend 改造規模 | UsageLimit migration | 推奨度 |
|---|---|---|---|---|---|
| **W** | 1 行改定（§3.1）| 小（新 endpoint 1 個 + UseCase wireup）| 小（/create page）| 後述 §6 で判断 | **★★★ 推奨** |
| X | 不要 | 大（upload-intent 改造）| 中（画像選択 UI も含む）| 同上 | ★ |
| Y | 不要 | 大（同上）| 中 | 同上 | ★ |
| Z | 不要 | なし | なし | なし | ✗（LP 失格問題未解決）|

→ **案 W を推奨**。業務知識 §3.1 の「初回画像アップロード時に server draft Photobook を作成」は MVP 設計時の理想形だが、現実装では `CreateDraftPhotobook` UseCase が **画像なしで draft を作る** 設計になっており、文言を実装に合わせる方が clean。

## 5. Turnstile 方針比較

### 5.1 案 T1（推奨）— `/create` submit に Turnstile 必須

- `/create` の type 選択 + 「編集を始める」ボタンに Turnstile widget を埋める
- 既存 ReportForm の L0-L4 ガード パターンを踏襲（`.agents/rules/turnstile-defensive-guard.md`）
- Backend `POST /api/photobooks` で `strings.TrimSpace(token) == ""` 早期 return + Cloudflare siteverify
- Turnstile action を新規追加（例: `photobook-create`）。`TURNSTILE_ACTION` env で固定値、cmd/api `main.go` で wireup
- bot 経由の draft 行スパム抑止に最も効果的

### 5.2 案 T2 — Turnstile を `/create` には置かず、後段（upload / publish）のみで守る

- `/create` は素通り、初回 upload で Turnstile（既存）、publish で Turnstile（既存）
- メリット: `/create` UX が最速、widget loading なし
- デメリット: 「draft を大量に作って削除しない」スパムを防げない（draft_expires_at で 7 日後 cleanup されるが、その間 DB 行が残る）

### 5.3 案 T3 — Turnstile を全段に置く（T1 + 既存 upload / publish）

- 案 T1 と既存の組み合わせ。最も厳格だが UX 摩擦も最大
- abuse が深刻な実例が出てから検討すべき過剰防衛

### 5.4 比較

| 観点 | T1（推奨）| T2 | T3 |
|---|---|---|---|
| draft 行スパム抑止 | 強 | 弱（draft_expires_at 経由のみ） | 強 |
| `/create` UX | Turnstile 1 回 | 最速 | 同 T1 |
| bot 経由 abuse 抑止 | 強 | 中 | 最強 |
| 実装規模 | 小（既存 widget 流用 + Backend turnstile.Verifier 流用 + action 追加）| 0 | 同 T1 |
| ADR-0005 / failure-log §5（Turnstile 必須）整合 | 高 | 中 | 最高 |

→ **案 T1 推奨**。L0-L4 多層防御は ReportForm で既に確立されており、`/create` でも同等のパターンで適用するのが整合的。

## 6. UsageLimit / RateLimit 方針比較

### 6.1 案 U1 — `photobook.create` action 新設（migration あり）

- migration v19 で `usage_counters_action_check` の CHECK 制約を 4 値に拡張
- `usagelimit/domain/vo/action/action.go` の enum に `rawPhotobookCreate = "photobook.create"` 追加
- `/create` submit で source_ip_hash × 1 時間 N 件（候補: 10）を制限
- 業務知識 v4 §3.7 に新規閾値を追記する必要あり（"1 時間 N 件" の確定）

### 6.2 案 U2（推奨）— action 追加せず、Turnstile + `publish.from_draft` で間接抑止

- `/create` は Turnstile のみ（案 T1）+ UsageLimit なし
- 既存 `publish.from_draft` 1h × 5 件で本物の公開乱用を抑止
- draft 行のスパムは Turnstile で大半を弾き、残りは `draft_expires_at` 7 日後の Reconcile（後続 PR33e 任意）に任せる
- migration 不要、PR 規模最小

### 6.3 案 U3 — Turnstile すら無し（やめる）

- `/create` を完全公開、UsageLimit / Turnstile なし
- bot による draft 行大量生成を許容 → DB / R2 容量逼迫
- **採用不可**

### 6.4 比較

| 観点 | U1 | **U2（推奨）** | U3 |
|---|---|---|---|
| draft 行スパム | 強 | 中（Turnstile のみで多くは弾く）| 弱 |
| migration | 必要（STOP migration）| 不要 | 不要 |
| 業務知識 §3.7 改定 | 必要 | 不要 | 不要 |
| PR 規模 | 中 | 小 | 小 |
| false positive | 中（NAT 配下で同 IP 多人数の場合）| なし | なし |
| PR36 設計との整合 | 高（同パターン拡張）| 中（Turnstile + 既存 publish 上限で代替）| 低 |
| Safari smoke / cleanup 負荷 | 中（usage_counters cleanup 必要）| 低 | 最低 |
| 後続柔軟性 | 既に拡張済 | 後続で必要なら U1 にアップグレード可 | 必要時に追加 |

→ **案 U2 推奨**。MVP は migration コストを避けつつ、Turnstile + 既存 publish 上限の組み合わせで十分な抑止になる。**長期的に draft 行スパムが観測されれば後続 PR で U1 にアップグレード**するのが自然な進化路。

## 7. API 設計案

### 7.1 endpoint（推奨）

```
POST /api/photobooks
Content-Type: application/json
Cache-Control: no-store
```

### 7.2 request body

```json
{
  "type": "memory" | "event" | "morning" | "portfolio" | "avatar" | "free",
  "title": "<任意、最大 N 文字>",
  "creator_display_name": "<任意、最大 M 文字>",
  "rights_agreed": false,
  "turnstile_token": "<Turnstile widget で取得した token>"
}
```

メタ詳細:
- `type`: 業務知識 §2.5 のフォトブックタイプ enum、必須、後で編集 UI で変更可
- `title` / `creator_display_name`: 任意、未入力時は空文字（後で edit UI で入力）
- `rights_agreed`: 公開前の権利・配慮確認は publish 時に取るため、ここは常に `false`
- `turnstile_token`: 必須（案 T1）

未受領フィールドの既定値（Backend で決定）:
- `layout`: `simple`
- `opening_style`: `light`
- `visibility`: `unlisted`（業務知識 §2.6 MVP 既定値）
- `draft_expires_at`: `now + 7 days`

### 7.3 response body（成功 201 Created）

```json
{
  "photobook_id": "<UUIDv7 raw>",
  "draft_edit_token": "<raw token>",
  "draft_edit_url_path": "/draft/<draft_edit_token>",
  "draft_expires_at": "2026-05-08T..."
}
```

#### raw token / id の扱い厳守

- response body に raw token / raw photobook_id を含むが、**Frontend が即座に消費**して URL を構築・redirect する
- ログ（access log / structured log / Sentry 等）に response body を出さない（既存 `addNoStore` + handler が body をログしないパターンを踏襲）
- response の `Cache-Control: no-store` を厳守（CDN / ブラウザに保持させない）

### 7.4 error mapping

| HTTP | body kind | 条件 |
|---|---|---|
| 400 | `invalid_payload` | JSON decode 失敗 / 必須フィールド欠落 / type が enum 外 / title 過長 |
| 403 | `turnstile_failed` | Turnstile token 空白のみ / siteverify 失敗 / hostname-action 不一致 |
| 503 | `turnstile_unavailable` | Turnstile siteverify ネットワーク障害（Cloudflare 側）|
| 500 | `server_error` | DB 障害 / 想定外 |

(案 U1 採用時は 429 `rate_limited` + Retry-After も追加。案 U2 採用時は不要。)

### 7.5 セキュリティヘッダ / CORS

- `Cache-Control: no-store`
- `X-Robots-Tag: noindex, nofollow`（middleware が付与済）
- `Referrer-Policy: strict-origin-when-cross-origin`（middleware）
- CORS: `app.vrc-photobook.com` Origin 許容、credentials は不要（Cookie を使わない）

### 7.6 Backend handler 配線

既存 PR36 / PR35b と同パターン:
- `backend/internal/photobook/interface/http/create_handler.go`（新規 file、SubmitReport handler を雛形に）
- `backend/internal/photobook/wireup/`（`create_wireup.go` 新規 + `cmd/api/main.go` で wireup）
- Turnstile verifier は既存 `internal/turnstile` を流用、`TURNSTILE_ACTION` を `photobook-create` で固定
- UsageLimit は案 U2 推奨のため **未配線**（案 U1 採用時のみ配線）
- `internal/http/router.go` に `r.Post("/api/photobooks", cfg.PhotobookCreateHandlers.CreatePhotobook)` を追加（公開 endpoint Group 内）

## 8. Frontend 設計案

### 8.1 page 構成

`frontend/app/(public)/create/page.tsx`（または `(draft)/create/page.tsx`、後者は middleware の Referrer-Policy: no-referrer に乗る）

#### 構成（案 A 推奨）

```
HEADER
  eyebrow "Create"
  h1 "どんなフォトブックを作りますか?"
  body "まずはタイプを選んでください。あとから変更できます。"

TYPE SELECTION（type-card grid）
  6 種: memory / event / morning / portfolio / avatar / free
  各カード: gradient placeholder + label + description + radio dot

TITLE（任意、後でも編集可）
  text input、placeholder "後で入力できます"

CREATOR DISPLAY NAME（任意）
  text input、placeholder "後で入力できます"

NOTICE
  「公開範囲は限定公開（URL を知っている人のみ閲覧可）が既定です。
   公開前の権利・配慮確認は次のステップで行います。」

TURNSTILE WIDGET
  TurnstileWidget action="photobook-create"

SUBMIT BUTTON
  「編集を始める」（Primary teal）
  disable: Turnstile 未通過 OR submitting OR type 未選択

ERROR STATES
  - turnstile_failed: 「認証に失敗しました。再度試してください。」
  - invalid_payload: 「入力内容を確認してください。」
  - server_error / network: 「一時的なエラーです。少し時間をおいて再度試してください。」
  - (案 U1 のみ) rate_limited: PR36 と同じ retryAfter 表示

SUCCESS
  response.draft_edit_url_path で window.location.replace
  → /draft/<token> route handler が Cookie 発行 + /edit/<id> redirect

PUBLIC PAGE FOOTER（5 nav links、no trust strip）
```

### 8.2 Turnstile 多層ガード（L0-L4）

`.agents/rules/turnstile-defensive-guard.md` 準拠、ReportForm と同パターン:

- **L0**: `TurnstileWidget` を `useRef` 安定化、parent re-render での remove → render cycle 抑止
- **L1**: submit ボタン disable は `turnstileToken.trim() !== ""`
- **L2**: onSubmit 冒頭で再度 trim 後 non-empty 確認、空なら early return
- **L3**: `lib/createPhotobook.ts` の API client 冒頭で trim 確認、empty なら fetch せず reject
- **L4**: Backend handler / UseCase で `strings.TrimSpace(token) == ""` 早期 return

### 8.3 LP CTA 差替

`frontend/app/page.tsx` の hero CTA を更新:
- 1st CTA: 「**今すぐ作る**」 → `/create`（teal solid Primary）
- 2nd CTA: 「使い方を見る」 → `/about`（teal ghost）

POLICY セクションに「管理 URL がある方は」 → `/help/manage-url` を補助で残す。

### 8.4 Safari / iPhone Safari 注意

- type-card grid は mobile 1 col / sm 2 col で折り返し
- Turnstile widget は ReportForm と同様に sm width で破綻しないこと
- title / creator_display_name input の font-size は 16px 以上（iOS の autozoom 抑止）
- submit 後の `window.location.replace` は **Safari の back navigation 履歴に残さない**（履歴に raw token が残るリスクを避ける）

### 8.5 Frontend 実装ファイル

| ファイル | 種別 | 概要 |
|---|---|---|
| `frontend/app/(public)/create/page.tsx` | A | /create page（実装は CreateClient component に分離）|
| `frontend/app/(public)/create/CreateClient.tsx` | A | Client Component（type 選択 / Turnstile / submit）|
| `frontend/lib/createPhotobook.ts` | A | API client（fetch + L3 guard + error mapping）|
| `frontend/app/page.tsx` | M | LP CTA 差替（1 行 + hero ボタン UI 修正）|

## 9. 本番 smoke / cleanup 設計

### 9.1 STOP ε で必要な smoke

- LP「今すぐ作る」→ /create → type 選択 + Turnstile + submit → /draft/<token> → /edit/<id> の **完全動線**確認
- Safari macOS + iPhone（最新）両方で実施
- Turnstile 失敗時 / invalid_payload 時 / network 時の error UI 確認

### 9.2 副作用

smoke 1 回で本番 DB に作成される行:
- `photobooks` 1 行（status=draft, draft_edit_token_hash 保存, draft_expires_at = now + 7 day）
- （案 U1 採用時のみ）`usage_counters` 1 行

### 9.3 cleanup 設計

| 観点 | 推奨 |
|---|---|
| 即時 cleanup | 不要（DB 副作用が小さく、draft_expires_at 経由で自然に GC される）|
| 即時削除する場合 | 手動 SQL: `UPDATE photobooks SET status='deleted', deleted_at=now() WHERE id IN (smoke target id list) AND status='draft'`、またはより安全に **直近 1 時間内の status=draft かつ creator_display_name=空 かつ image 0 件** という delta-based 一意確認 |
| draft 行は `draft_expires_at` で自然 GC | **基本これに任せる**（業務知識 §3.1 / §6.13 / Reconcile 設計）|
| usage_counters | 案 U2 採用時は **生成されない**、案 U1 採用時のみ 24h grace で自然 expire |
| `cmd/ops` に削除 CLI | 既存に photobook softDelete CLI **無し**（Moderation 拡張で `softDelete` / `restore` / `purge` 候補だが MVP 範囲外、roadmap §1.3）|

→ **STOP ε cleanup 推奨方針**: smoke で作られた draft 行は **`draft_expires_at` 7 日後の自然 GC に任せる**。手動削除は STOP α で別承認を取った場合のみ実施。raw photobook_id / raw draft_edit_token は記録しない。

### 9.4 raw 値の取り扱い

- chat / commit / docs / failure-log には書かない
- /tmp に raw photobook_id を保存する場合は chmod 600、smoke 後即削除
- DB 操作を伴う場合は cloud-sql-proxy + delta-based 一意確認 + `FOR UPDATE` lock + rowcount assert + ROLLBACK on mismatch（PR36 / SubmitReport 緩和 PR と同流儀）

## 10. STOP 設計

| STOP | 内容 | 必要性 |
|---|---|---|
| **α** | 本書承認、§13 ユーザー判断 #1〜#7 を確定（**現在地**）| 必要 |
| **β** | Backend handler + UseCase wireup + Turnstile 配線 + tests / Frontend /create page + CreateClient + lib + LP CTA 差替 + tests / build / cf:check 全 PASS / 単一 commit + push | 必要 |
| **migration**（**案 U1 採用時のみ**）| Cloud SQL に v19 migration 適用（usage_counters CHECK 拡張）| 案 U2 採用時は **不要** |
| **γ** | Backend Cloud Build deploy + Cloud Run revision 更新 + Cloud Run Job image bump | 必要 |
| **δ** | Workers redeploy（cf:build + wrangler deploy）| 必要（/create 追加）|
| **ε** | Safari 実機 smoke（macOS + iPhone）+ 完全動線（LP → /create → /draft → /edit）+ DB 副作用 cleanup（draft_expires_at に任せる方針）| 必要 |
| **final closeout** | work-log + roadmap + 業務知識 v4 §3.1 改定（**案 W 採用時必須**）+ runbook 必要なら + commit + push | 必要 |

rollback 候補:
- Backend: 現 active `vrcpb-api-00023-pwv` / `vrcpb-api:773d5cc`（PR36 / SubmitReport 緩和反映済）
- Workers: 現 active `c2d35a6c-9d14-4626-886c-47362b78b8e2`（PR37 design rebuild 反映済）
- Cloud SQL: migration v18（PR36 STOP α 適用、本 PR で v19 migration を入れる場合のみ rollback 計画必要）

## 11. テスト方針

### 11.1 Backend tests

- `create_handler_test.go`: Turnstile blank token / invalid_payload / siteverify failure / success path（既存 PR35b ReportForm パターン）
- `create_handler_test.go`: type enum 外、title 過長などの validation
- `create_photobook_wireup_test.go`（任意）: wireup 構造のスモーク
- 既存 `create_draft_photobook_test.go` は変更なし

### 11.2 Frontend tests

- `app/(public)/create/__tests__/CreateClient.test.tsx`: SSR レンダリング（type 選択 / Turnstile placeholder / submit disabled when no token）
- `lib/__tests__/createPhotobook.test.ts`: API client（trim guard / error mapping / success path、既存 `report.test.ts` パターン）
- `app/__tests__/public-pages.test.tsx`: LP の CTA 差替を検証（`href="/create"` を含む）

### 11.3 e2e（Safari smoke で担保）

vitest e2e は導入しない。STOP ε の Safari 実機で完全動線を確認。

### 11.4 Secret / raw grep

- redact 対象値（raw token / raw photobook_id / Cookie 値 / Secret 等）が新規 file に含まれないこと
- Test fixtures には test-only token（`test-turnstile-token` 等）を使用

## 12. 推奨案（まとめ）

| 観点 | 推奨 |
|---|---|
| 導線 | **案 A**（`/create` 専用ページ、prototype `CreateStart` 直系）|
| 業務知識整合 | **案 W**（業務知識 §3.1 を 1 行改定して "type 選択時に server draft 作成" に）|
| Turnstile | **案 T1**（`/create` submit に Turnstile 必須、L0-L4 多層）|
| UsageLimit | **案 U2**（action 追加せず、Turnstile + 既存 `publish.from_draft` 上限で間接抑止、migration 不要）|
| Frontend route 名 | `/create`（最も明示的、user 判断対象）|
| cleanup | smoke で作られた draft 行は `draft_expires_at` 7 日自然 GC に任せる |
| rollback | usecase 変更を 1 commit に分離、Frontend / Backend / Workers それぞれ独立 revert 可能 |
| STOP migration | **不要**（案 U2 採用時、案 U1 を選ぶなら追加）|

→ 推奨フル: **A + W + T1 + U2** で進める。実装規模・rollback 容易性・既存 PR36 設計との整合 のバランスが最良。

## 13. ユーザー判断事項（**本 PR 着手前に確定**）

1. **採用 4 軸の確定**: A / W / T1 / U2 の組み合わせで OK か（推奨案）。修正があれば個別指示
2. **Frontend route 名**: `/create` / `/start` / `/new` のどれにするか（推奨: `/create`）
3. **Turnstile action 値**: `photobook-create`（推奨）/ 既存 `upload` / `report-submit` 等の流用 / それ以外
4. **業務知識 v4 §3.1 改定文**: 案 W 採用時、改定文の表現を本 PR final closeout で確定（草案: 「タイプ選択時に server draft Photobook を作成し、`draft_edit_token` を発行する」）
5. **type 選択肢**: 6 種（memory / event / morning / portfolio / avatar / free）すべて表示 / 最小化（memory + free のみ等）/ prototype `screens-a.jsx CreateStart` の 5 種に揃える
6. **title / creator_display_name の入力**: `/create` で受ける（任意、推奨）/ `/edit` まで持ち越して `/create` は type のみ
7. **既定 visibility**: `unlisted`（推奨、業務知識 §2.6）/ 別の値 / 作成時に選択 UI を出す
8. **smoke 後の draft cleanup**: 自然 GC（推奨、draft_expires_at 7 日）/ 即時手動 SQL 削除 / Moderation 拡張（softDelete）を待つ
9. **`creator_display_name` の MVP 段階での扱い**: `/create` で空欄許容（推奨、`/edit` で入力）/ `/create` で必須 / `/create` では出さない
10. **権利・配慮確認**: 既存通り publish 時に取る（推奨、変更なし）

## 14. 採用後に更新が必要な docs / runbook / roadmap

| ファイル | 更新内容 |
|---|---|
| `docs/spec/vrc_photobook_business_knowledge_v4.md` §3.1 | 案 W 採用時、「初回画像アップロード時に server draft 作成」→「type 選択時に server draft 作成」に 1 行改定 |
| `docs/plan/m2-create-entry-plan.md`（本書） | 採用案を §13 に追記、final closeout で履歴更新 |
| `docs/plan/vrc-photobook-final-roadmap.md` §1.1 / §1.3 | Backend revision / Workers version 更新、`/create` 機能追加マーカー、後続候補から「作成導線追加」を完了扱いに |
| `docs/plan/m2-usage-limit-plan.md` | 案 U1 採用時のみ §x に `photobook.create` 追加、案 U2 採用時は **更新不要** |
| `docs/runbook/usage-limit.md` | 案 U1 採用時のみ §11.x に新 action smoke 注意、案 U2 採用時は **更新不要** |
| `docs/runbook/backend-deploy.md` | 案 U1 で migration 必要なら §x に v19 migration 注意、その他更新不要 |
| `harness/work-logs/2026-05-XX_create-entry-result.md`（新規）| STOP α / β / γ / δ / ε / final closeout の進行記録 |

## 15. raw 値の取り扱い運用ルール（再掲、PR 全期間）

- raw `draft_edit_token` / raw `manage_url_token` / raw `session_token` / Cookie 値 / Secret 値 / `source_ip_hash` 完全値 / `scope_hash` 完全値 / `reporter_contact` / `detail` 実値: chat / commit / docs / work-log / failure-log には **書かない**
- raw `photobook_id` UUIDv7 全長: 同上、redact 表記は先頭 8 文字 prefix + `<redacted>`
- API response body / Cookie / Set-Cookie ヘッダ / Cloud Build log / Cloud Run log / wrangler deploy log: Secret 値が出ていないことを STOP γ / δ / ε 各段階で grep
- `<user-local VRChat photo folder>` のような **redact ラベル**で参照、実 Windows ローカルパスは書かない

## 16. 履歴

| 日付 | 変更 |
|------|------|
| 2026-05-01 | 初版（STOP α 設計判断資料）。導線案 A/B/C/D + 業務知識整合 W/X/Y/Z + Turnstile T1/T2/T3 + UsageLimit U1/U2/U3 を比較。推奨フル: A + W + T1 + U2。ユーザー判断 10 項目 + 採用後の docs/runbook/roadmap 更新リストを整理 |
