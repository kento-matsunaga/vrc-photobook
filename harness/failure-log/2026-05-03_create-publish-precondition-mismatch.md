# /create と publish の precondition mismatch（creator 空 / rights_agreed false）

## 発生日

2026-05-03 STOP α 調査で発見（実装は PR9b publish 追加 + 1f2af4d /create 空欄許容時に整合が崩れた）。
本番影響期間: 1f2af4d deploy 〜 a8fe0db (creator 部分) / 9c4fb7d (rights 部分) deploy。

## 症状

ユーザが `/create` から photobook を作成 → `/prepare` で写真追加 → `/edit` で「公開へ進む」を押す → 「公開条件に合致しません。最新を取得して再度確認してください。」エラー。何が足りないかユーザに伝わらず、何度 reload しても解消しない。

Cloud Run logs `/publish` で `3 POST 409 / 0 POST 200`。a8fe0db deploy 前は creator 空が、a8fe0db deploy 後 9c4fb7d deploy 前は rights_agreed=false が原因。

事故クラス: **入口（/create）で許容した state を、出口（publish）で禁止する設計矛盾。両者の整合を取る経路がコードベース上に存在しない**。

## 根本原因

### Issue A: creator 空欄

| layer | 動作 |
|---|---|
| `/create` request | `creator_display_name` 任意フィールド、空文字許容 |
| domain `NewDraftPhotobook` | length-only チェック、空 OK |
| domain `CanPublish` (a8fe0db 前) | `creatorDisplayName == ""` → `ErrEmptyCreatorName` |
| `UpdatePhotobookSettings` SQL | creator_display_name 列なし、更新不能 |
| `/edit` UI `PublishSettingsPanel` | creator 入力欄なし |

→ 空 creator で作成された draft は **どこからも creator を埋められず**、publish で 409 に阻まれる。

### Issue B: rights_agreed=false ハードコード

| layer | 動作 |
|---|---|
| `/create` UI | 「公開前の権利・配慮確認は publish 時に行います」と表示 |
| `/create` request | `rights_agreed` field 不在 |
| `create_handler.go` | `RightsAgreed: false` ハードコード |
| domain `CanPublish` | `!rightsAgreed` → `ErrRightsNotAgreed` |
| `/edit` UI / その他 endpoint | rights_agreed=true を保存する経路なし |

→ **全 photobook が rights_agreed=false で作成され、publish で必ず 409**。

両者とも単独で publish を完全 block する致命的バグ。

## 修正

### Issue A (a8fe0db: creator hotfix)

- `domain.CanPublish` から `creatorDisplayName == "" → ErrEmptyCreatorName` を削除
- `ErrEmptyCreatorName` 定義は残置（B 案 = /edit に creator 入力 UI 追加 で再活性化する将来用）
- 短期 hotfix 方針。長期の B 案は後続 PR

### Issue B (9c4fb7d: publish 時同意取得)

業務知識 v4 §3.1（公開前の権利・配慮確認は必須、同意日時記録）を満たす形で実装:

- `domain.Photobook.WithRightsAgreed(now)` 追加（不変、新値返し）
- `PublishFromDraftInput.RightsAgreed bool` 追加。false → early `ErrRightsNotAgreed`、true → 同 TX で `pb.WithRightsAgreed(in.Now) → CanPublish → repository.PublishFromDraft`
- `PublishPhotobookFromDraft` SQL に `rights_agreed=true` / `rights_agreed_at=$4` 追加（partial に同意だけ残らない / publish のみ通って同意未保存にならない）
- `publishRequest.RightsAgreed bool` 受け取り
- Frontend `PublishSettingsPanel` に同意 checkbox 追加、未チェックで `disabled`、`publish-rights-required-hint` 表示
- Frontend `publishPhotobook(photobookId, expectedVersion, rightsAgreed)` の 3 引数化、body に rights_agreed を含める

## 追加した test

### Backend

- `backend/internal/photobook/domain/photobook_test.go` (a8fe0db):
  - 「正常_空creator_name許容」: 空 creator で `NewDraftPhotobook` が成功
  - 「正常_creator空でもpublish可_hotfix」: 空 creator + rights_agreed=true で `CanPublish() == nil`
  - 「正常_WithRightsAgreed適用後はpublish可_p0v2」: rights_agreed=false 始点 → `WithRightsAgreed(now)` 適用後 `CanPublish() == nil`
- `backend/internal/photobook/domain/photobook_test.go` (9c4fb7d):
  - `TestPhotobook_WithRightsAgreed`: false→true 反映 + 不変性 + 既に true でも now 更新
- `backend/internal/photobook/interface/http/publish_handler_test.go` (9c4fb7d):
  - 「異常_rights_agreed_false_409_rights_not_agreed」
  - 「異常_rights_agreed_missing_409_rights_not_agreed」
  - 「正常_publish成功時にDBのrights_agreed_atが永続化される」
  - 「異常_already_published_は_409_not_draft」: publish 後の再 publish が `not_draft` reason
  - 「異常_version_mismatch_409_version_conflict」

### Frontend

- `frontend/components/Edit/__tests__/PublishSettingsPanel.test.tsx` (9c4fb7d): 4 ケース（checkbox 描画 / 初期 disabled + hint / publishDisabledReason 共存 / placeholder）
- `frontend/lib/__tests__/publishPhotobook.test.ts` (9c4fb7d): rights_agreed 送信 16 ケース

## 今後の検知方法

- creator gate を戻すと `「正常_creator空でもpublish可_hotfix」` が落ちる
- rights gate を撤廃しようとすると `「異常_rights_agreed_false_409_rights_not_agreed」` が落ちる（撤廃後は false でも 200 になるため）
- **`.agents/rules/publish-precondition-ux.md` で「authenticated owner には reason enum を返す」「ユーザが直せない文言禁止」をルール化**

## 残る follow-up

- B 案: `/edit` に creator 入力欄 + `UpdatePhotobookSettings` API 拡張で creator も更新可能にする（後続 PR）
- 業務知識 v4 §3.1 に「rights agreement は publish 操作と同 TX で保存」を明文化
- `/create` で rights_agreed checkbox UI を追加するか、publish 時取得継続のままかの最終判断（現状は publish 時取得）

## 関連

- `docs/spec/vrc_photobook_business_knowledge_v4.md` §3.1
- `backend/internal/photobook/domain/photobook.go` `CanPublish` / `WithRightsAgreed`
- `backend/internal/photobook/internal/usecase/publish_from_draft.go`
- `backend/internal/photobook/interface/http/publish_handler.go` `writePublishError`
- `frontend/components/Edit/PublishSettingsPanel.tsx` 同意 checkbox
- `.agents/rules/publish-precondition-ux.md`
- a8fe0db / 9c4fb7d commit
