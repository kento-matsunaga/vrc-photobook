# 編集 UI 本格化 実装計画（PR26 計画書）

> 作成日: 2026-04-27
> 位置付け: 新正典 [`docs/plan/vrc-photobook-final-roadmap.md`](./vrc-photobook-final-roadmap.md)
> §3 PR26 の本体。実装は PR27、publish 本実行は PR28。
>
> 上流参照（必読）:
> - [新正典ロードマップ](./vrc-photobook-final-roadmap.md)
> - [業務知識 v4](../spec/vrc_photobook_business_knowledge_v4.md) §3 / §6 / §7
> - [Photobook ドメイン設計](../design/aggregates/photobook/ドメイン設計.md)
> - [Photobook データモデル設計](../design/aggregates/photobook/データモデル設計.md)
> - [Image データモデル設計](../design/aggregates/image/データモデル設計.md)
> - [ADR-0005 Image Upload Flow](../adr/0005-image-upload-flow.md)
> - [PR25 公開 Viewer / 管理ページ計画](./m2-public-viewer-and-manage-plan.md)
> - [PR25b 結果](../../harness/work-logs/2026-04-27_public-viewer-manage-result.md)
> - [PR22 frontend upload UI 結果](../../harness/work-logs/2026-04-27_frontend-upload-ui-result.md)
> - [PR23 image-processor 結果](../../harness/work-logs/2026-04-27_image-processor-result.md)
> - [`.agents/rules/domain-standard.md`](../../.agents/rules/domain-standard.md) §集約子テーブル OCC
> - [`.agents/rules/safari-verification.md`](../../.agents/rules/safari-verification.md)

---

## 0. PR26 から PR28 への流れ

| PR | 内容 |
|---|---|
| **PR26（本書）** | 編集 UI 本格化の **計画書のみ**（実装しない） |
| **PR27** | 編集 UI 本格化の **実装**（photo grid / caption / reorder / cover / settings UI、Backend 拡張、Safari 確認） |
| **PR28** | publish flow 完成（公開ボタン / 完了画面 / URL コピー / manage URL 控え） |

> **PR25b で残課題**: 画像表示を含む 200 経路の **完全 visual Safari 確認**は PR25b では未実施（fixture
> photobook が photo 0 件だったため）。**PR27 で photo grid + display variant を実装した時点で、
> /p/[slug] の Viewer も併せて視覚確認する**こと。PR25b work-log §Safari 確認 を参照。

---

## 1. 目的

- 既存 `frontend/app/(draft)/edit/[photobookId]/UploadClient.tsx`（upload 専用）を
  本格編集 UI に拡張する
- アップロード済 photo を photo grid で表示する（display / thumbnail variant 経由）
- caption 編集 / page-photo reorder / cover 設定 / publish settings 配置を可能にする
- publish 本実行は PR28 に分離し、PR27 では UI 配置と「公開へ進む」遷移までに留める
- OCC（楽観ロック）と version 競合の UI 方針を確定する
- design prototype `screens-a.jsx` Edit / `pc-screens-a.jsx` PCEdit を Tailwind に
  移植する方針を明確化

---

## 2. PR26 対象範囲

### 対象（本書で確定する）

- 編集 UI のレイアウト / コンポーネント分割
- Backend API 候補の I/F 案（追加 / 既存流用の境界）
- OCC / 楽観ロック方針（既存 `.agents/rules/domain-standard.md` 集約子テーブル OCC を遵守）
- Frontend state 設計（Server / Client 分離、optimistic update、競合 UI）
- reorder の実装方針（上下ボタン推奨、drag & drop は PR41+）
- caption / cover / publish settings の UX 仕様
- design 抽出ルール
- Safari 確認チェックリスト
- Test 観点
- PR27 実装の checklist

### 対象外（PR26 で決めない / 触らない）

- 実装本体（PR27）
- publish 本実行 / 完了画面（PR28）
- Outbox / SendGrid / OGP 自動生成
- Moderation / Report / UsageLimit
- LP / terms / privacy / about
- Cloud Run Jobs / Scheduler / Public repo 化

---

## 3. 編集 UI 構成

### 3.1 既存 `UploadClient.tsx` の扱い

- PR22 で実装した upload 機構（Turnstile → upload-verifications → upload-intent →
  R2 PUT → complete）は **そのまま再利用**する
- PR27 では `UploadClient` を「アップロード専用 widget」としてリファクタし、
  本格編集ページの 1 セクションに組み込む（責務を分離）
- 完了 callback で photo grid を refresh する hook を持たせる

### 3.2 ページレイアウト（モバイル）

`design/mockups/prototype/screens-a.jsx` の Edit を参照。

```
[ Header ]                       — title 編集 inline / 保存ステータス表示
[ Steps progress ]               — design prototype `Steps`（公開フロー進捗）
[ Photo grid ]                   — display variant + caption 編集 / 並び替え / 削除 / cover 指定
  [ + Add photos ]               — upload widget をモーダル / 折りたたみで起動
[ Cover settings ]               — 現在の cover 表示 + クリア
[ Publish settings panel ]       — type / visibility / description 編集 + 「公開へ進む」ボタン（PR28 で実機能化）
```

### 3.3 ページレイアウト（PC）

`design/mockups/prototype/pc-screens-a.jsx` の PCEdit を参照（3 列レイアウト）。

- 左: page list / 並び順 / page caption
- 中央: 選択 page の photo grid + 詳細パネル
- 右: publish settings / cover preview

PR27 ではモバイル優先 + PC は 2 列簡易（左 page list、右 main）で開始。3 列は PR41+ で評価。

### 3.4 Loading / Error / Empty state

- Loading: skeleton（photo placeholder）
- Error: ErrorState コンポーネント（PR25b で導入済）
- Empty: 「最初の写真をアップロードしましょう」+ Upload widget を強調表示
- Conflict (409): 「他の編集が反映されました」+「最新を取得」ボタン → 全体 reload

---

## 4. Backend API 候補

### 4.1 新規 endpoint（PR27 で追加）

| method | path | 役割 |
|---|---|---|
| GET    | `/api/photobooks/{id}/edit-view` | 編集ページ初期データ（photobook + pages + photos + variants の URL）|
| PATCH  | `/api/photobooks/{id}/photos/{photoId}/caption` | photo caption 更新 |
| PATCH  | `/api/photobooks/{id}/photos/reorder` | 並び替え（複数 photo の display_order を一括 set） |
| PATCH  | `/api/photobooks/{id}/cover-image` | cover_image_id 更新 |
| DELETE | `/api/photobooks/{id}/cover-image` | cover_image_id クリア |
| PATCH  | `/api/photobooks/{id}/settings` | title / description / type / layout / opening_style / visibility 更新 |
| PATCH  | `/api/photobooks/{id}/pages/{pageId}/caption` | page caption（page_meta.note 等）更新 |
| DELETE | `/api/photobooks/{id}/pages/{pageId}` | page 削除（既存 RemovePage UseCase 流用） |
| DELETE | `/api/photobooks/{id}/photos/{photoId}` | photo 削除（既存 RemovePhoto UseCase 流用） |
| POST   | `/api/photobooks/{id}/pages` | page 追加（既存 AddPage UseCase 流用） |

> 既存 `internal/photobook/internal/usecase/photobook_edit.go` には **RemovePage /
> RemovePhoto / ReorderPhoto / SetCoverImage / ClearCoverImage / UpsertPageMeta** が
> 既に実装済。PR27 では HTTP layer の実装と **caption 単独編集 / settings 更新 / edit-view 返却**
> の 3 系統を新規追加する。

### 4.2 認可と境界

- すべて **draft session middleware（既存 `RequireDraftSession`）必須**
- manage Cookie では編集不可（業務知識 v4 §6: manage は閲覧 / 再発行のみ、編集は draft 経由）
- `status='published'` 以降は編集不可（PR27 では明示 409 を返す。publish 後の編集導線は PR28+）
- `status='draft'` AND `version=$expected_version` を全 UPDATE の WHERE に含める（OCC、既存ルール）

### 4.3 競合時の挙動

- 0 行 UPDATE → 409 Conflict + body `{"status":"version_conflict"}` 固定
- Frontend は 409 を受けたら edit-view を再 fetch して全体 reload を促す
- DB 内部の状態（draft / version 不一致 / etc）は外部に区別を漏らさない

### 4.4 画像 / variant の扱い

- edit-view で返す photo は **`status='available'`** の image のみが対象
- `processing` / `failed` の image は別欄「処理中 N 件」「失敗 N 件」として件数だけ表示
  （実装の選択肢、§17 ユーザー判断事項）
- variant URL は PR25 と同じ短命 presigned GET（display + thumbnail、15 分有効）

---

## 5. Domain / Repository 方針

### 5.1 必ず守るルール（`.agents/rules/domain-standard.md`）

- 集約子テーブル更新（`photobook_pages` / `photobook_photos` / `photobook_page_metas` /
  `image_variants`）は **必ず親 `photobooks.version` +1 と同一 TX**で実施
- 子テーブル単独更新を public Repository メソッドに**出さない**
- すべての操作は UseCase 経由
- `display_order` の連続性は domain / UseCase で保証（DB は UNIQUE のみ）
- DEFERRABLE UNIQUE / 子テーブル単独 trigger は採用しない

### 5.2 既存資産の流用範囲

| 操作 | 既存 UseCase | PR27 実装する HTTP layer |
|---|---|---|
| ページ追加 | `AddPage` | POST endpoint |
| ページ削除 | `RemovePage` | DELETE endpoint |
| 写真追加 | `AddPhoto` | (既存 `AddPhoto` を edit-view 内で起動、PR22 upload 完了後の経路と統合) |
| 写真削除 | `RemovePhoto` | DELETE endpoint |
| 写真並び替え | `ReorderPhoto` | PATCH endpoint（複数行 = 既存 UseCase を per-row 呼び出し）|
| Cover 設定 | `SetCoverImage` | PATCH endpoint |
| Cover クリア | `ClearCoverImage` | DELETE endpoint |
| Page meta upsert | `UpsertPageMeta` | PATCH endpoint |

### 5.3 新設が必要な domain / UseCase

- **`UpdatePhotoCaption`** UseCase: `photobook_photos.caption` 単独編集（既存に photo caption 単独
  更新の専用 UseCase は無いため新設）
- **`UpdatePhotobookSettings`** UseCase: title / description / type / layout / opening_style /
  visibility / cover_title 等を一括 PATCH（既存にはない）
- **`GetEditView`** Query UseCase: 編集画面の read（PR25 `GetPublicPhotobook` の draft 版に相当、
  manage Cookie ではなく draft Cookie 必須）

### 5.4 Reorder 時の UNIQUE 衝突回避

`photobook_photos` には `UNIQUE (page_id, display_order)` がある。複数 photo の swap や
shift は単純 UPDATE では 23505 衝突。PR27 では:

- 一括 reorder API は「photo_id → 新 display_order の対応表」を受け取り、
  - **方式 A（推奨、§17 で確定）**: 全 photo を `display_order = old + 1000` に一旦退避（同 TX）→ 順次新 order に書き戻し（同 TX）
  - 方式 B: 1 件ずつ swap（実装複雑、性能悪い）

採用は方式 A。実装は新規 sqlc query `BulkReorderPhotosOnPage`。

### 5.5 `cover_image_id` の所有者整合

`SetCoverImage` 既存 UseCase は image owner / status を同 TX で検証する。HTTP 層は
ユーザーの選択した image_id を渡すだけ。owner 不一致 → ErrImageNotAttachable に
集約されて 409 / 403 へ変換（仕様確定は PR27 着手時）。

---

## 6. Frontend state 設計

### 6.1 Server / Client 分離

- `app/(draft)/edit/[photobookId]/page.tsx` (Server Component) → edit-view を fetch
- 取得した data を `<EditClient />` (Client Component) に props として渡す
- `<EditClient />` 内で:
  - photo grid の選択 state
  - caption 編集 buffer
  - reorder pending state
  - upload widget の連携
  - 409 conflict 時の reload 誘導

### 6.2 Optimistic update 方針

| 操作 | optimistic | 失敗時 |
|---|---|---|
| caption 変更 | **適用**（UI 即時反映） | toast でエラー、buffer を元に戻す |
| reorder（上下 1 段） | **適用** | toast で「他の編集が反映されました」+ reload ボタン |
| cover 設定 | **適用** | 同上 |
| upload | 既存 PR22 の通り（processing 表示） | 既存 |
| settings 変更 | **適用しない**（保存ボタン経由）| toast |

### 6.3 version 保持

- edit-view 取得時に `expected_version` を保持（Client 側 state）
- 各 PATCH に `expected_version` を載せ、成功で `+1`、失敗で reload 誘導

### 6.4 processing / failed 画像の扱い

- §17 ユーザー判断事項で確定。デフォルト案:
  - **processing**: edit-view 応答に件数のみ含める。Client 側で 5 秒ごとに edit-view を
    polling する（§7 reorder 実装と並行する設計）。**PR27 では simple polling 採用、後続
    PR で SSE 等を評価**
  - **failed**: 件数表示 + 「失敗詳細」モーダル placeholder（PR41+ で実装）

### 6.5 Form dirty / Save status

- caption / settings の dirty state を Client で持つ
- header 右に「保存中…」「保存済み」「未保存」を表示（design prototype の保存インジケータに合わせる）
- save status は SR でも読み上げられるよう aria-live を使う

---

## 7. Reorder 方針

### 7.1 選択肢

| 案 | 利点 | 欠点 |
|---|---|---|
| A: 上下ボタン（↑↓） | iOS Safari で安全、a11y 良好、実装単純 | 多数の写真を移動するのが面倒 |
| B: drag & drop（HTML5 DnD） | UX 直感的 | iOS Safari の touch 対応が脆弱、reorder 中の事故率が高い |
| C: 上下ボタンのみ（PR27）+ drag & drop は PR41+（dnd-kit 等） | UX 段階的拡張、安全 | 完全 UX は後回し |
| D: dnd-kit を PR27 から導入 | 1 度で完了 | バンドルサイズ増、PR27 の範囲拡大 |

### 7.2 推奨: **案 C**

- PR27 は **上下ボタン**で確定（iOS Safari / 高齢者 / アクセシビリティ重視）
- 「先頭へ」「末尾へ」「1 つ上」「1 つ下」の 4 操作
- drag & drop は PR41+ で dnd-kit を評価導入

---

## 8. Caption 編集方針

### 8.1 仕様

- caption max = **200 runes**（既存 `caption.Caption` VO で保証）
- 改行 / 全角空白許容（既存 VO 仕様）
- photo / page 両方に caption（PR27 では photo を優先、page caption は時間が許せば）

### 8.2 保存タイミング

| 案 | 利点 | 欠点 |
|---|---|---|
| A: blur で自動保存 | 手数最少、失敗 toast で気付ける | 編集中にうっかり離脱で意図せず保存 |
| B: 明示保存ボタン | 意図が明確、未保存検知が容易 | クリック手間 / ボタン UI 必要 |
| C: debounce 自動保存（800ms） | UX 滑らか、自動 | 連打通信、失敗 UX が読みにくい |

### 8.3 推奨: **案 A（blur 保存）+ 未保存検知バナー**

- blur で PATCH を発火、結果は Save status インジケータに反映
- 未保存変更 + ページ離脱で `beforeunload` 警告（モバイルは制約あり、PC のみ）
- 409 conflict 時は buffer を維持しつつ reload 誘導

---

## 9. Cover 設定方針

### 9.1 仕様

- cover 候補は `status='available'` の image のみ
- 設定済 cover が `deleted` / `failed` になった場合は cover が外れる（FK ON DELETE SET NULL は既存 schema）
- cover 未設定時は viewer / OGP で title / type ベースのプレースホルダ（OGP 本実装は PR33）

### 9.2 PR27 で実装する範囲

- **photo grid から右クリック / メニューで「cover に設定」**（モバイルは長押しメニュー回避、専用ボタン）
- 「cover をクリア」ボタン
- 現在の cover を panel に preview 表示
- viewer / OGP との視覚的整合は PR33 で OGP 実装時に再検証

### 9.3 PR33 OGP との連携

- cover_image_id があれば OGP 生成 Job がそれを使う（PR33 計画書で詳細）
- PR27 段階では cover_image_id を保持するだけで OGP 生成は触らない

---

## 10. Publish settings 方針

### 10.1 PR27 で配置する UI（実機能化は PR28）

- title / description / type / layout / opening_style / visibility / cover_title 編集
- 「公開へ進む」ボタン（PR27 は disabled / placeholder、PR28 で publish 実行に接続）
- 公開チェックリスト（rights_agreed 等）
- 公開済 photobook の場合は読み取り専用 + 公開停止 placeholder

### 10.2 settings PATCH の境界

- title 1〜80 文字 / description 0〜500 文字 / 各 enum 値（既存 VO で validation）
- visibility 切替時の警告（public ↔ unlisted の意味）
- 設定保存は **保存ボタン経由**（caption と異なり一括）

---

## 11. design 参照

### 11.1 抽出ルール

- prototype は **値の抽出元**。直接 import / コピペしない
- design-system 第一弾（PR25b）を流用（colors / typography / spacing / radius-shadow）
- アイコンは prototype `shared.jsx` の `Icon` 40 種から必要分だけ SVG 化

### 11.2 PR27 で抽出する prototype 資産

| 用途 | 参照点 |
|---|---|
| edit モバイル | `design/mockups/prototype/screens-a.jsx` `Edit` |
| edit PC（3 列） | `design/mockups/prototype/pc-screens-a.jsx` `PCEdit` |
| Photo placeholder | `shared.jsx` `Photo`（`v-a`〜`v-f`）→ MVP では実画像差し替え |
| Avatar | `shared.jsx` `Av`（イニシャル + 5 色グラデ） |
| Steps progress | `shared.jsx` `Steps` / `pc-shared.jsx` `PCSteps` |
| 各種 icon | `shared.jsx` `Icon` から必要分のみ |

### 11.3 design-system 第二弾の範囲（PR41+）

- `components.md`（Photo / Av / TopBar / Steps / UrlRow 等の正典）
- motion / transition の段階値
- icon set の正典化
- PR27 では token を流用するのみで、新規 design-system の章は**作らない**

---

## 12. Safari / iPhone Safari 確認

`.agents/rules/safari-verification.md` 発火条件に該当（Cookie / モバイル UI / form / token→session）。

### 12.1 PR27 完了前の必須項目

- macOS Safari（最新）
- iPhone Safari（最新、可能なら 1 世代前も）

### 12.2 シナリオ

#### Edit ページ
- `/draft/<token>` から redirect → `/edit/<id>` 着地 → edit-view 表示
- photo grid に display variant が表示される（**PR25b で残課題だった完全 visual 確認をここで実施**）
- upload → 自動 grid 更新 → caption 編集（blur 保存）
- 上下ボタン reorder
- cover 設定 / クリア
- settings 編集 / 保存
- 409 conflict 時の reload 誘導

#### 公開 Viewer（PR25 残課題と統合）
- PR27 で実装した photo を含む photobook を fixture で publish → `/p/<slug>` で
  display 画像が iPhone Safari で正しく表示されることを確認
- reload で presigned URL 再取得 + 画像表示維持

### 12.3 一般確認

- Cookie 維持（draft + manage 両方）
- raw token / Cookie 値が URL に出ない
- noindex / no-store ヘッダ
- iPhone Safari でレイアウト崩れなし（横画面含む）

---

## 13. Test 方針

### 13.1 Backend

- handler test: 各 PATCH / DELETE / POST endpoint の 200 / 400 / 401 / 404 / 409 経路
- usecase test: OCC conflict（draft 以外 / version 不一致）
- reorder test: UNIQUE 衝突回避（一時退避 + 順次 UPDATE）
- caption test: 200 runes 境界 / 不正 rune
- cover test: 別 photobook 所有 image を cover に設定 → 失敗
- settings test: 各 enum / 文字数境界

### 13.2 Frontend

- component test: photo grid / caption editor / reorder buttons / cover panel / settings panel
- API client test: error mapping（401 / 404 / 409 / 500 / network）
- optimistic update test: 失敗時のロールバック
- 409 conflict test: reload 誘導 UI

### 13.3 Safari manual

- §12 の全項目

### 13.4 Security

- response / log に raw token / Cookie / presigned URL / storage_key 完全値が出ない
- caption の HTML / script を render 時に escape（React の標準 escape を信頼、`dangerouslySetInnerHTML` 禁止）

---

## 14. Security

### 14.1 守るべき不変条件

- 編集系は **draft Cookie 必須**。manage Cookie / public Cookie では編集不可
- `status='published'` 以降は編集不可（PR27 で 409 / 403）
- public viewer から edit URL / draft token が漏れない（PR25b 確認済の方針を維持）
- storage_key 完全値を UI / log / API response に出さない（PR25 と同じ）
- presigned URL を console.log しない
- caption は React の text 出力で XSS 抑止（自前 sanitize は不要、`dangerouslySetInnerHTML` 禁止）
- 画像 URL の期限切れ時は edit-view 再 fetch で再取得（自動 polling と兼用）
- token / Cookie / Secret を log に出さない

### 14.2 grep 監査（PR27 commit 前）

```
grep -RInE "DATABASE_URL=|PASSWORD=|SECRET=|SECRET_KEY|API_KEY|sk_live|sk_test|draft_edit_token=|manage_url_token=|session_token=|R2_SECRET_ACCESS_KEY=|TURNSTILE_SECRET_KEY=" \
  backend/internal/photobook backend/internal/image backend/internal/imageupload \
  frontend/app frontend/components frontend/lib
```

実値ヒット 0 件。用語は許容。

---

## 15. PR27 実装範囲（最終 checklist）

### 15.1 Backend

- [ ] `GetEditView` UseCase + `GET /api/photobooks/{id}/edit-view` handler
- [ ] `UpdatePhotoCaption` UseCase + `PATCH /api/photobooks/{id}/photos/{photoId}/caption`
- [ ] `BulkReorderPhotosOnPage` UseCase + sqlc query + `PATCH /api/photobooks/{id}/photos/reorder`
- [ ] `UpdatePhotobookSettings` UseCase + `PATCH /api/photobooks/{id}/settings`
- [ ] `PATCH /api/photobooks/{id}/cover-image` / `DELETE /api/photobooks/{id}/cover-image`（既存 SetCoverImage / ClearCoverImage を HTTP layer に出す）
- [ ] `POST /api/photobooks/{id}/pages` / `DELETE /api/photobooks/{id}/pages/{pageId}`
- [ ] `DELETE /api/photobooks/{id}/photos/{photoId}`
- [ ] `PATCH /api/photobooks/{id}/pages/{pageId}/caption`（UpsertPageMeta 経由）
- [ ] router 配線 + main.go wiring
- [ ] 全 endpoint で OCC + draft Cookie + status=draft 確認
- [ ] handler / usecase / repository test（OCC conflict 含む）
- [ ] Cloud Run revision 更新（PR27 完了時）

### 15.2 Frontend

- [ ] `app/(draft)/edit/[photobookId]/page.tsx` を Server Component で edit-view fetch
- [ ] `<EditClient />` の Client Component（photo grid / caption / reorder / cover / settings）
- [ ] Photo grid（display + thumbnail variant）
- [ ] Caption editor（blur 保存 + Save status）
- [ ] Reorder 上下ボタン
- [ ] Cover panel（preview + 設定 / クリア）
- [ ] Publish settings panel（保存ボタン、公開ボタンは disabled placeholder）
- [ ] processing / failed 件数表示 + simple polling
- [ ] 409 conflict reload 誘導
- [ ] API client（lib/editPhotobook.ts）
- [ ] component test / api client test
- [ ] design 抽出（icon の必要分 SVG 化）
- [ ] Workers redeploy（PR27 完了時）

### 15.3 検証

- [ ] Safari macOS / iPhone（§12）
- [ ] Photo を含む publish → /p/[slug] visual 確認（**PR25b 残課題の完了**）
- [ ] log / response の secret leak 監査

### 15.4 PR27 で実装しない（再掲）

- publish 本実行（PR28）
- drag & drop reorder（PR41+）
- OGP 自動生成（PR33）
- SendGrid（PR32）/ Outbox（PR30）/ Moderation（PR34）/ Report（PR35）/ UsageLimit（PR36）/ LP（PR37）

---

## 16. 実リソース操作

| 操作 | 必要性 |
|---|---|
| Workers redeploy | **必要**（編集 UI 拡張） |
| Cloud Run revision 更新 | **必要**（新 endpoint 追加） |
| sqlc generate（reorder 用 query 追加） | **必要**（schema 変更なし） |
| Cloud SQL migration | **不要**（schema 変更なし） |
| Secret 追加 | **不要** |
| Dashboard 操作 | **不要** |
| Safari 実機確認 | **必要** |

---

## 17. ユーザー判断事項（PR27 着手前に確定）

| 判断項目 | 推奨 | 代替 |
|---|---|---|
| Reorder 方式 | **上下ボタン**（PR27） / drag & drop は PR41+ | PR27 から dnd-kit |
| Caption 保存タイミング | **blur 保存** + 未保存検知 | 明示保存ボタンのみ |
| Cover 設定 PR27 含めるか | **含める**（実装軽量、UX 完成度上がる） | PR41+ に分離 |
| Publish settings 編集範囲 | title / description / type / layout / opening_style / visibility / cover_title | type / layout 等は PR41+ |
| Processing 画像の polling | **simple polling 5s** | 手動 reload のみ |
| Failed 画像 UI | **件数のみ表示** + モーダル placeholder | 詳細 UI を PR27 に含める |
| Page caption 編集 PR27 含めるか | **時間が許せば（後段）** / 必須は photo caption | PR27 では photo caption のみ |
| 公開済 photobook の編集 | **PR27 では 409 / 403 で禁止** / 編集導線は PR28 以降 | PR27 でも編集可（複雑化） |
| Cloud SQL `vrcpb-api-verify` 残置 | **PR39 まで継続**（新正典通り） | 早期 rename |
| Public repo 化 | **PR38 まで保留** | 早期公開 |

---

## 18. 完了条件

- 本計画書 review 通過
- §17 ユーザー判断事項が確定
- §15 checklist が PR27 着手時にそのまま使える状態

## 19. 次 PR への引き継ぎ事項

PR27 実装着手時に必ず参照する設計確定事項:

- §4 Backend API 候補
- §5 Domain / Repository ルール（OCC 遵守）
- §6 Frontend state 設計
- §7 Reorder 方針（上下ボタン）
- §8 Caption（blur 保存）
- §9 Cover 設定範囲
- §10 Publish settings の境界（PR28 への引き継ぎ）
- §12 Safari 確認チェックリスト
- §13 Test 観点
- §14 Security 不変条件

PR28 への引き継ぎ:

- §10.1 publish ボタンを PR28 で実機能化
- §15.4 publish 本実行 / 完了画面 / URL コピー / manage URL 控え / Outbox INSERT placeholder

---

## 20. 履歴

| 日付 | 変更 |
|------|------|
| 2026-04-27 | 初版作成。PR25b 完了時点での編集 UI 本格化計画 |
