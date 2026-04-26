# 2026-04-27 Safari / iPhone Safari 実 token 結合確認 実施結果（PR17）

## 概要

`docs/plan/m2-frontend-workers-deploy-plan.md` PR17 / `.agents/rules/safari-verification.md`
に基づき、macOS Safari / iPhone Safari の実機で実 token 経路（`/draft/<token>` /
`/manage/token/<token>` → HttpOnly Cookie 発行 → 302 redirect）が成立することを確認した。

これにより M2 ドメイン疎通フェーズ（PR12 〜 PR17）が完了する。

- 実施日時: 2026-04-27 02:24〜02:26 JST（Safari 確認〜cleanup まで約 2 分）
- 対象ドメイン: `https://app.vrc-photobook.com` → `https://api.vrc-photobook.com`
- 対象ブラウザ: macOS Safari 最新 / iPhone Safari 最新
- Cloud SQL: `vrcpb-api-verify`（**PR17 完了後の保持/削除判定が必要**）

## 前提

- PR15 で `app.vrc-photobook.com` Workers Custom Domain 稼働中
- PR12 で `api.vrc-photobook.com` Cloud Run Domain Mapping 稼働中
- PR16 で curl 経路の実 token 結合は成立済（draft / manage 200）

## tokengen（一時、コミット禁止、cleanup 済）

- 配置: `backend/internal/photobook/_tokengen/main.go`
- 同じ手順で draft + manage の raw token を発行し、URL を `/tmp/vrcpb-safari-urls.txt`
  へ保存（チャット・コミットには貼らない）
- ユーザーが Windows / WSL 経由で Safari に URL を転送し、実機確認を実施
- 確認後に tokengen / `backend/_tokengen` バイナリ / token URL ファイル /
  Cloud SQL Auth Proxy をすべて停止・削除

## 検証結果

### macOS Safari — draft 経路

- draft URL 投入後、アドレスバーが `https://app.vrc-photobook.com/edit/<photobook_id>`
  に変わる ✅
- raw token がアドレスバーに残らない ✅
- Web インスペクタ → ストレージ → Cookie に `vrcpb_draft_<id>` が存在 ✅
  - Domain: `.vrc-photobook.com` ✅
  - HttpOnly ✅
  - Secure ✅
  - SameSite=Strict ✅
  - Path=/ ✅
  - Expires が約 7 日後 ✅
- ⌘R 再読込後も `/edit/<id>` のままで session 維持 ✅

### macOS Safari — manage 経路

- アドレスバーが `https://app.vrc-photobook.com/manage/<photobook_id>` に変わる ✅
- raw token は残らない ✅
- Cookie `vrcpb_manage_<id>`、属性は draft と同じ ✅
- 再読込後も session 維持 ✅

### iPhone Safari — draft / manage 経路

- 両 URL とも redirect が即座に成立、URL から raw token が消える ✅
- `/edit/<id>` / `/manage/<id>` ページが正常に表示される ✅
- 再読込しても同じページが開ける（session 維持）✅
- 戻るボタン操作で raw token URL が再表示される事象は **観測されず** ✅
- iPhone Safari 上の Cookie 詳細値の確認は OS 制約で不可（macOS Safari 側の
  Web Inspector で属性確認済のため、同一サーバ実装で同じ Cookie が発行されることを根拠）

### Safari Private Browsing

- 本実施では **未実施**（通常モードでの確認を優先、Private Browsing は ITP 影響の
  別観点として後日継続観察項目）
- `.agents/rules/safari-verification.md` の必須項目（macOS Safari + iPhone Safari）は
  満たしているため、本 PR の完了判定には影響しない

## Backend (`api.vrc-photobook.com`) 200 確認

`gcloud logging read` で Safari 確認時刻帯（17:24:00Z 以降）を確認:

| timestamp (Z) | endpoint | status |
|---|---|---|
| 17:25:59 | draft-session-exchange | **200** |
| 17:25:07 | manage-session-exchange | **200** |
| 17:24:57 | draft-session-exchange | **200** |

- Workers (`app.vrc-photobook.com`) 経由で Safari からの実 token POST が Cloud Run に
  届き、Backend が 200 を返している ✅
- 3 件のうち draft が 2 件あるのは macOS Safari / iPhone Safari の両端末による
  並行確認（manage は片端末で実施）
- request URL path には raw token が残らない（POST body のため access log に出ない）

## 漏洩 grep

```
gcloud run services logs read vrcpb-api --region=asia-northeast1 --limit=500 |
  grep -iE "(SECRET|API_KEY|PASSWORD|PRIVATE|sk_live|sk_test|draft_edit_token|
            manage_url_token|session_token|set-cookie|DATABASE_URL=)"
```

→ **マッチなし** ✅

## 一時ファイル・一時コード削除

- `backend/internal/photobook/_tokengen/`: 削除済 ✅
- `backend/_tokengen`（go build 副産物バイナリ）: 削除済 ✅
- `/tmp/vrcpb-safari-urls.txt`（token URL 一時ファイル）: 削除済 ✅
- `/tmp/vrcpb-safari-start.epoch`（時刻マーカー）: 削除済 ✅
- Cloud SQL Auth Proxy: 停止済 ✅
- 環境変数 `DB_PASSWORD` / `DATABASE_URL_PROXY` / `DRAFT_RAW` / `MANAGE_RAW`: unset 済 ✅
- git status: clean（作業ログのみ追加）✅

## 継続観察項目（PR17 で完了させない）

- **24 時間後 / 7 日後の Cookie 残存**（Safari ITP の長期影響、`.agents/rules/safari-verification.md`）
- macOS Safari / iPhone Safari の Private Browsing 動作（後日、編集 UI などの実装後に
  あらためて確認）
- 編集 UI 実装後の Safari 確認（PR22 以降の UI 系変更時にも `safari-verification.md`
  に従って必須確認）

## 実施しなかったこと

- Cloud SQL `vrcpb-api-verify` の削除（**PR17 完了直後に保持/削除判定**）
- Backend / Frontend / Workers / DNS / Secret Manager の変更
- SendGrid / Turnstile / R2 設定
- 本番 router への debug endpoint 追加
- dummy token 成功経路の追加
- tokengen コードのコミット
- raw token URL / raw token / session_token / Cookie 値 / DATABASE_URL /
  DB password / Secret payload の表示・記録
- Safari Private Browsing での確認

## Cloud SQL 残置/一時削除の判断材料

PR17 完了後に必ず判断する。

判断材料:

- **PR18 (Image / Upload) にすぐ進むか**
  - すぐ進むなら DB が要るため残置が合理的
  - 数日空くなら一時削除でゼロ円維持が合理的
- **費用見込み**
  - 残置: db-f1-micro + 10GB SSD で ~¥55/日（30 日で ~¥1,650）
  - 一時削除: ¥0/日。再作成時に migration 再実行 + Secret 更新で 5-10 分の手間
- **DB に残っている検証データ**
  - PR16 / PR17 で作成した draft / published photobook が複数件残っている
  - これらは検証用の捨てデータなので、削除して再作成しても問題なし
- **再作成コスト**
  - `gcloud sql instances create` ~3-5 分
  - migration `goose up` ~1 分
  - Secret Manager `DATABASE_URL` 更新 + Cloud Run revision 切替 ~2 分
  - 合計 ~10 分
- **現時点の累計経過**: ~6 時間 / ~¥14

推奨（roadmap §A の方針 + 計画書 §13.2 のコスト方針整合）:

- 「**PR18 の作業着手を 2 日以内に始めるなら残置**」
- 「**3 日以上空くなら一時削除して PR18 着手時に再作成**」
- 中間ケース（1〜2 日）: 残置のまま PR18 計画策定を進めるのが心理的負担が低い

## Cloud SQL 残置/一時削除 — 決定（PR17 完了直後、2026-04-27）

ユーザー判断により **残置** に決定。

### 決定内容

- Cloud SQL `vrcpb-api-verify`: **残置**
- Secret Manager `DATABASE_URL`: **残置**
- Cloud Run `vrcpb-api` の DB あり revision (`vrcpb-api-00002-pdn`): **維持**
- DB なし revision (`vrcpb-api-00001-q9h`): rollback 先として **残置**
- 削除コマンドは現時点では **実行しない**

### 残置理由

- 次の PR18 (Image / Upload) に連続して進む予定
- `/readyz 200` + 実 token E2E + Safari 実機確認まで通った状態を維持したい
- 再作成・migration・Secret 更新・revision 切替の手戻りを避ける
- 費用は当面許容

### 費用目安

- db-f1-micro + 10GB SSD で **約 ¥55/日**
- 30 日放置は予算 ¥1,000 を超えるので避ける
- 累計（2026-04-26 作成〜本判断時点）: ~6 時間 / ~¥14

### 期間ガード（無期限残置を防ぐ）

- まず **2 日以内** に PR18 計画へ着手する
- 次回判断タイミング: **PR18 計画書完了時** または **2 日後**（早い方）
- 3 日以上作業が空く見込みになった時点で **一時削除を再検討**
- 検証用 DB を本番相当としてなし崩しに使い続けない

### 削除手順（参考、今回は実行しない）

PR12 計画書 / 既存の cloud-sql-deploy 系計画書を参照。最低限:

```sh
# Cloud Run revision を DB なし版に切戻し（必要なら）
gcloud run services update-traffic vrcpb-api \
  --region=asia-northeast1 --to-revisions=vrcpb-api-00001-q9h=100

# Cloud SQL instance 削除
gcloud sql instances delete vrcpb-api-verify --quiet

# Secret Manager DATABASE_URL は残しても課金はかからないが、
# 値がスタブとして残ると誤解を招くため、必要に応じて destroy
gcloud secrets versions destroy <version> --secret=DATABASE_URL
```

再作成は migration `goose up` + `gcloud sql users create` + Secret 更新 +
Cloud Run revision 切替で ~10 分。

## 関連

- [`.agents/rules/safari-verification.md`](../../.agents/rules/safari-verification.md)
- [PR16 実 token 結合確認結果](./2026-04-27_frontend-backend-real-token-e2e-result.md)
- [PR15 Frontend Custom Domain 結果](./2026-04-27_frontend-custom-domain-result.md)
- [PR14 Frontend Workers Deploy 結果](./2026-04-27_frontend-workers-deploy-result.md)
- [PR12 Backend Domain Mapping 結果](./2026-04-27_backend-domain-mapping-result.md)
- [Post-deploy Final Roadmap §A](./2026-04-27_post-deploy-final-roadmap.md)
