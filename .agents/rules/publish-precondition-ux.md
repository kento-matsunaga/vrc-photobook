# Publish / mutation precondition の UX ルール

## 適用範囲

`POST /api/photobooks/{id}/publish` を始め、**authenticated draft session 必須**な mutation の error response 設計、および対応する Frontend エラー文言。

## 原則

> **authenticated owner には reason enum を返し、ユーザが直せる具体文言で示す。**
> **「ユーザが直せないエラー文言」「曖昧な汎用文言」は禁止。**

理由:
- `/publish` 等の draft session 必須経路は **攻撃者が到達不能**。「敵対者観測抑止のため理由を区別しない」設計判断は、本経路では復旧導線を消すデメリットの方が大きい。
- 認証された owner にとって「何が公開条件を満たしていないか」を伝えないと、reload しても解消しない事態に陥り、サポート問い合わせ → 個別救済しか道がない。
- 2026-05-03 STOP α で `/publish` 409 response が `bodyConflict` (`{"status":"conflict"}`) のみで、Frontend は「公開条件に合致しません。最新を取得して再度確認してください。」固定文言を出していた。実態は (1) creator 空（a8fe0db で hotfix）/ (2) rights_agreed=false（9c4fb7d で同意 UI 実装）/ (3) version mismatch / (4) not_draft の 4 種が混在しており、ユーザが何を直せばよいか不明 + 「最新を取得」を押しても reload で解消しないループに陥っていた。

## 必須パターン

### 1. Backend: response shape を分離

409 を以下 2 種類の structured response に分離:

| 種別 | body | 該当 error |
|---|---|---|
| state / OCC 競合 | `{"status":"version_conflict"}` | OCC 違反 / repository OCC / 楽観ロック / pgErr unique 等 |
| 公開前提未達 | `{"status":"publish_precondition_failed","reason":"<enum>"}` | rights 未同意 / not_draft / empty_creator / empty_title 等 |

reason enum:
- `rights_not_agreed` — 権利・配慮確認 checkbox 未同意
- `not_draft` — 既に published / deleted（再公開不能）
- `empty_creator` — creator_display_name 空（B 案で再活用）
- `empty_title` — title 空
- `unknown_precondition` — 想定外への safeguard（追加した reason がここに落ちないよう監視）

raw ID / 内部詳細（SQL state / repository error message / pgErr code 等）は body に出さない。authenticated owner 向けでも内部実装のリーク抑止は維持。

### 2. Frontend: reason 別の具体文言 + 「最新を取得」CTA は version_conflict のみ

```ts
// EditClient.onPublish 内
if (e.kind === "version_conflict") {
  setConflict("conflict");                                    // ← banner + 「最新を取得」CTA
  setErrorMsg("他の編集が反映されました。最新を取得して再度公開してください。");
  return;
}
if (e.kind === "publish_precondition_failed") {
  setConflict("ok");                                          // ← banner / CTA は出さない
  switch (e.reason) {
    case "rights_not_agreed":
      setErrorMsg("公開前に権利・配慮確認への同意が必要です。チェックを入れてから公開してください。");
      return;
    case "not_draft":
      setErrorMsg("このフォトブックは既に公開済み、または編集できない状態です。");
      return;
    case "empty_creator":
      setErrorMsg("作者名が未設定です。現在の画面では修正できません。");
      return;
    // ...
  }
}
```

### 3. body 解釈失敗の安全側

malformed body / status 不在 → `version_conflict` 既定にフォールバック（旧互換）。未知 reason → `unknown_precondition` に丸める。Frontend は「公開条件を満たしていません。入力内容を確認してください。」のような generic だが「ユーザが何かを確認すれば直る可能性」を示唆する文言にする。

## 禁止事項

1. **「公開条件に合致しません。最新を取得して再度確認してください。」固定文言**（または同等の reload で解消しない曖昧文言）を再混入させる
2. **複数の異質な error を 1 つの body に集約する**（precondition と OCC を混ぜる）
3. **reason enum に raw ID / 内部詳細を含める**
4. **新 precondition error を追加したのに reason enum に追加しない**（`unknown_precondition` に流すだけにすると検知できない）
5. **「最新を取得」CTA を precondition_failed 経路で出す**（reload で解消しないため UX の罠）

## チェックリスト

publish / mutation の precondition error response を変更する PR では:

- [ ] Backend: 新 error が `version_conflict` / `publish_precondition_failed` のどちらに属するか判断
- [ ] Backend: precondition なら新 reason enum を追加、`writePublishPrecondition` mapping 更新
- [ ] Backend: handler test に新 case を追加（`assertPreconditionReason` / `assertVersionConflict` helper）
- [ ] Frontend: `publishPhotobook` の `parse409Body` / `normalizePublishPreconditionReason` に新 reason 追加
- [ ] Frontend: `EditClient.onPublish` の switch に新 reason の文言を追加
- [ ] Frontend: 旧 antipattern 文言が source に存在しないことを source guard test で確認
- [ ] Frontend: 「最新を取得」CTA が version_conflict 経路のみで出ることを source guard で確認

## Why

`/publish` のような authenticated owner 経路では、敵対者観測抑止より UX 復旧導線を優先する。reason enum の分離は public 経路（report / public viewer 等）には適用しない（攻撃者が到達可能なため敵対者観測抑止を維持）。

修正自体は handler の error mapping + Frontend parse / 文言変更だが、事故クラスは「authenticated 経路で情報を隠しすぎてユーザを詰ませる UX 設計」。本ルール + 旧曖昧文言の不在 source guard で再発防止。

## 関連

- `backend/internal/photobook/interface/http/publish_handler.go` `writePublishError` / `writePublishPrecondition` / `writePublishVersionConflict`
- `frontend/lib/publishPhotobook.ts` `parse409Body` / `normalizePublishPreconditionReason`
- `frontend/app/(draft)/edit/[photobookId]/EditClient.tsx` `onPublish`
- `harness/failure-log/2026-05-03_publish-409-vague-error.md`
- `harness/failure-log/2026-05-03_create-publish-precondition-mismatch.md`
- 9c4fb7d commit

## 履歴

| 日付 | 変更 |
|------|------|
| 2026-05-03 | 初版作成。STOP α publish 409 集約事故をクラス level にルール化 |
