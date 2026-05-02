# publish 409 が version conflict / precondition failed を区別せず曖昧

## 発生日

2026-05-03 STOP α 調査で発見。実装は PR28 publish 追加時から。
本番影響期間: PR28 deploy 〜 9c4fb7d deploy。

## 症状

`/edit` で「公開へ進む」を押して 409 が返ると、Frontend は常に「公開条件に合致しません。最新を取得して再度確認してください。」を表示。何が問題か（rights 同意か、既に published か、本当に version mismatch か、creator 空か、title 空か）が一切伝わらず、ユーザが「最新を取得」を押しても reload では解消しないケースが大半。**ユーザが回復不能なループ**。

事故クラス: **エラーの「種別を消す」設計（敵対者観測抑止）を、authenticated owner 向けに無分別に適用した結果、復旧導線まで消してしまう**。

## 根本原因

`backend/internal/photobook/interface/http/publish_handler.go` の `writePublishError` (9c4fb7d 前):

```go
case errors.Is(err, usecase.ErrPublishConflict),
    errors.Is(err, photobookrdb.ErrOptimisticLockConflict),
    errors.Is(err, photobookrdb.ErrNotDraft),
    errors.Is(err, domain.ErrNotDraft),
    errors.Is(err, domain.ErrRightsNotAgreed),
    errors.Is(err, domain.ErrEmptyCreatorName),
    errors.Is(err, domain.ErrEmptyTitle):
    writeJSONStatus(w, http.StatusConflict, bodyConflict)  // = `{"status":"conflict"}`
```

**6 種類の異質なエラーを 1 つの `bodyConflict` に集約**。Frontend は kind だけ見て「他の編集が反映されました」固定文言を出す経路に入る。

設計コメント:
> 状態不整合 / OCC 違反は 409 に集約。「draft 以外」「version 不一致」「rights 未同意」「title / creator 空」を区別しない（情報漏洩抑止）。

しかし `/publish` は **draft session 必須**で攻撃者は到達不能。authenticated owner にとっての復旧導線（何を直すか）を消すデメリットが、敵対者観測抑止のメリットを大きく上回っていた。

## 修正

### Backend (9c4fb7d)

`writePublishError` を 2 種類の structured response に分離:

| HTTP | body | 該当 error |
|---|---|---|
| 409 | `{"status":"version_conflict"}` | `ErrPublishConflict` / `ErrOptimisticLockConflict` / pgErr unique violation |
| 409 | `{"status":"publish_precondition_failed","reason":"rights_not_agreed"}` | `domain.ErrRightsNotAgreed` |
| 409 | `{"status":"publish_precondition_failed","reason":"not_draft"}` | `domain.ErrNotDraft` / `photobookrdb.ErrNotDraft` |
| 409 | `{"status":"publish_precondition_failed","reason":"empty_creator"}` | `domain.ErrEmptyCreatorName`（dead path、B 案再活用用） |
| 409 | `{"status":"publish_precondition_failed","reason":"empty_title"}` | `domain.ErrEmptyTitle`（dead path、B 案再活用用） |

reason は **authenticated owner 向けの enum**。raw ID / 内部詳細は含めない。

### Frontend (9c4fb7d)

`publishPhotobook` で 409 body を parse:
- `version_conflict` → `kind: "version_conflict"`
- `publish_precondition_failed` + reason → `kind: "publish_precondition_failed", reason`
- 未知 reason → `unknown_precondition` に丸める
- body 解釈失敗 → `version_conflict` 既定

`EditClient.onPublish` で reason 別文言:
- `rights_not_agreed`: 「公開前に権利・配慮確認への同意が必要です。チェックを入れてから公開してください。」
- `not_draft`: 「このフォトブックは既に公開済み、または編集できない状態です。」
- `empty_creator`: 「作者名が未設定です。現在の画面では修正できません。」
- `empty_title`: 「タイトルを入力してください。」
- `unknown_precondition`: 「公開条件を満たしていません。入力内容を確認してください。」
- `version_conflict`: 「他の編集が反映されました。最新を取得して再度公開してください。」+ banner で「最新を取得」CTA 表示

旧固定文言「公開条件に合致しません。最新を取得して再度確認してください。」は完全撤去。

## 追加した test

### Backend (9c4fb7d)

`backend/internal/photobook/interface/http/publish_handler_test.go`:
- 「異常_rights_agreed_false_409_rights_not_agreed」: body assertion `assertPreconditionReason(... "rights_not_agreed")`
- 「異常_already_published_は_409_not_draft」: 同上 `not_draft`
- 「異常_version_mismatch_409_version_conflict」: `assertVersionConflict(...)`
- helper `assertPreconditionReason` / `assertVersionConflict`

### Frontend (9c4fb7d)

`frontend/lib/__tests__/publishPhotobook.test.ts`:
- 「異常_409_version_conflictをparse」
- 「異常_409_publish_precondition_failed_reason_<reason>」（4 reason）
- 「異常_409_未知reasonはunknown_preconditionにフォールバック」
- 「異常_409_status不在bodyはversion_conflictに既定」（malformed body 安全処理）

`frontend/app/(draft)/edit/[photobookId]/__tests__/EditClient.publish.test.ts`:
- 「正常_publish_precondition_failed reason 別文言が出る」
- 「正常_version_conflict のみ「最新を取得」案内、precondition_failed では出さない」: 旧固定文言「公開条件に合致しません。最新を取得して再度確認してください。」が source に存在しないことを assert
- 「正常_publish_precondition_failed 経路では setConflict('conflict') にしない」

## 今後の検知方法

- 旧固定文言再混入は EditClient.publish.test.ts source guard で即落ち
- reason mapping の脱落は publishPhotobook.test.ts table 駆動で検知
- Backend の reason 集約への退化は publish_handler_test.go の `assertPreconditionReason` で検知
- **`.agents/rules/publish-precondition-ux.md` で「authenticated owner には reason enum」「ユーザが直せない文言禁止」をルール化** → 新たな曖昧文言の追加が PR レビューで指摘される

## 残る follow-up

- DOM testing library 導入後に「checkbox 押下 → button enable → click で publishPhotobook 呼出」を behavior test 化
- 他の 409 経路（settings PATCH 等）も同様に分離するかの判断（現状 settings は単純な OCC のみ）

## 関連

- `backend/internal/photobook/interface/http/publish_handler.go` `writePublishError` / `writePublishPrecondition` / `writePublishVersionConflict`
- `frontend/lib/publishPhotobook.ts` `parse409Body` / `normalizePublishPreconditionReason`
- `frontend/app/(draft)/edit/[photobookId]/EditClient.tsx` `onPublish` reason switch
- `.agents/rules/publish-precondition-ux.md`
- 9c4fb7d commit
