# /prepare reload で queue が消える

## 発生日

2026-05-02 STOP ε（ユーザ実機 Chrome smoke）以前に観測。
原因実装は β-2 / β-3 の前。

## 症状

`/prepare/<photobookId>` で複数画像 upload 中にブラウザ reload すると、画面 tile が消えて空 queue になる。upload 自体は server 側で進行しているが UI から見えない → ユーザは「失敗した」と判断して再 upload してしまう / 別の photobook を作ってしまう。

## 根本原因

`/prepare` の queue 状態が React `useState` ローカル state にしか存在せず、reload で揮発していた。Server 側には image record（owner_photobook_id + status）が確実にあるのに、Frontend がそれを読み出して queue に復元する経路を持っていなかった。

事故クラス: **client-only state にユーザ作業状況を保存し、server ground truth を持たない設計**。

## 修正

- Backend (β-2, commit 9ac7699): `GET /api/photobooks/{id}/edit-view` の response に `images: []`（imageId / status / sourceFormat / originalByteSize / failureReason / createdAt）を追加。attach 済 / 未配置を問わず photobook 配下の全 active image を列挙。
- Frontend (β-3, commit f455fe4):
  - SSR initialView.images から queue を初期化（reload 後も「全部消えた」状態にしない）。
  - polling 中も `mergeServerImages(queue, server.images, placedImageIds, labelLookup, idGen)` で server を ground truth に reconcile。
  - localStorage は filename 補助のみ（imageId は key 保管に限定、UI/DOM/data-testid に raw image_id を出さない）。

## 追加した test

| File | 内容 |
|---|---|
| `frontend/components/Prepare/__tests__/UploadQueue.test.ts` | `mergeServerImages` 9 ケース（空 queue / labelLookup / placedImageIds 除外 / 既存 tile status 更新 / failed→processing_failed / uploading 復元 / local-only 維持 / placed 移動で削除 / createdAt 順）+ `imageIdOf` 3 ケース |
| `frontend/app/(draft)/prepare/[photobookId]/__tests__/PrepareClient.test.tsx` | reload 復元 SSR で server 3 image (processing/available/failed) → 3 tile 表示、**raw image_id 3 種が HTML に絶対出ないこと**を assert |
| `frontend/lib/__tests__/prepareLocalLabels.test.ts` | filename 補助 cache 9 ケース（往復 / cross-photobook 隔離 / TTL / 上限 / SSR 不在 / Quota 例外 / raw imageId 非露出） |

## 今後の検知方法

- `mergeServerImages` の動作変更があれば table 駆動 test が即落ちる。
- raw image_id が UI に出る regression は SSR test が secret pattern 検出で落ちる。
- 新画面で同種の reload-loss を発生させないため、`.agents/rules/state-restore-on-reload.md` に「client state は server ground truth または local persistence で復元できる設計を必須」とルール化。

## 残る follow-up

- DOM testing library を入れて、polling 中の merge 動作を behavior test で確認（現状は SSR markup + pure-fn unit test のみ）
- /edit でも同様の「reload 後に caption / cover state が消える」事故が起きないか design 確認

## 関連

- `docs/plan/m2-prepare-resilience-and-throughput-plan.md` v2 §3.2 / §3.4
- `harness/work-logs/` β-2 / β-3 commit
- `.agents/rules/state-restore-on-reload.md`
