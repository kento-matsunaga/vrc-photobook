# 2026-04-29 PR35b L1-L4 修正 deploy 後の vrcpb-api revision 00020 で `GET /api/public/photobooks/{slug}` が chi default 404 を返す

## 発生状況

- **何をしようとしていたか**: PR35b STOP ε2 として、L1-L4 多層 Turnstile ガード反映後の Backend image (`4c95617`) + Workers (`5d09172b`) で Safari 実機 Report 送信 smoke を行うため、`cmd/ops photobook unhide` で hidden=true → false に切り替え、`/p/<対象>/report` の SSR 動作確認に進もうとした。
- **どのファイル/モジュールで発生したか**:
  - 表面: Backend Cloud Run service `vrcpb-api`、新 revision `vrcpb-api-00020-9jz`（image `vrcpb-api:4c95617`）の `GET /api/public/photobooks/{slug}` route
  - 周辺: `backend/internal/http/router.go` / `backend/cmd/api/main.go`
  - 影響: Frontend Workers `/p/<slug>` SSR / `/p/<slug>/report` SSR が `notFound()` に転落（公開 Viewer / 通報フォーム 全体が劣化）

## 失敗内容

- `vrcpb-api-00020-9jz` で `GET /api/public/photobooks/<実 slug>` が **HTTP 404 + plain text "404 page not found"** を返した（Cloudflare 経路 / Cloud Run direct URL 双方で再現）。
  - response header: `content-type: text/plain; charset=utf-8` / `vary: Origin` / `x-content-type-options: nosniff` / `content-length: 19`
  - これは chi の default `NotFound` 出力（handler 自身なら `{"status":"not_found"}` JSON）。
- 同 revision で `GET /api/public/photobooks/<photobook_id>/ogp` は **HTTP 200**、`GET /api/photobooks/<photobook_id>` は **HTTP 401**（draft session 必須）が正常に返る。**slug GET だけが route 未登録状態**だった。
- Cloud Run startup logs にエラーは無く、`r2 configured; image upload endpoints enabled` `report endpoint enabled` `db pool configured` `server starting` の順で正常起動ログのみ出ていた。
- `cmd/api/main.go` で `photobookPublicHandlers = wireup.BuildPublicHandlers(pool, r2Client)` は実行されている前提（R2 OK のログがある）であり、`router.go` L93 `r.Get("/api/public/photobooks/{slug}", cfg.PhotobookPublicHandlers.GetPublicPhotobook)` も該当 if 条件を満たして実行されているはず。それでも実行時に handler に到達しない。
- **同じソースコードを使う rollback 先 revision `vrcpb-api-00019-jkj`（image `vrcpb-api:f4427b1`）では完全正常**: real slug → JSON 410 gone、fake slug → JSON 404 not_found、OGP → 200。ヘッダ・ボディ完全に一致（Cloud Run direct URL でも api.vrc-photobook.com でも一致）。
- ということは、**ソースコード差分（L1-L4 ガード適用、router 系ファイルは未変更）が同じはずなのに 00020 build ではこの route だけ動かない** という、build / image レベルの謎が残っている。

## 根本原因

- **未確定**。発生時点では rollback で本番劣化を止めることを優先した（`docs/runbook/backend-deploy.md` に従う rollback、 `gcloud run services update-traffic vrcpb-api --to-revisions=vrcpb-api-00019-jkj=100`）。
- 仮説（再現確認待ち）:
  1. **Cloud Build キャッシュ / 別 commit 由来の binary**: Cloud Build が submit した tarball / build cache に何か古い state が混ざり、router.go の登録処理が build 結果から欠落した可能性。git の commit hash を image tag に使うが、binary 中身が必ずしも commit と一致するとは限らない（特に build ログを精査していない）。
  2. **chi の routing tree 衝突**: 同一階層に `/api/public/photobooks/{slug}` (GET) と `/api/public/photobooks/{photobookId}/ogp` (GET) と `/api/public/photobooks/{slug}/reports` (POST) が並ぶ際、chi v5 の radix tree が path parameter 名を区別しないため、tree 構築時に GET `{slug}` ノードが書き換えられる挙動が **特定の build / バイナリでだけ顕在化**する可能性。00019 では同じ source からの build でも問題が出なかったのと矛盾するが、build cache の差で chi 内部の map order が変わって露呈、というシナリオはあり得る。
  3. **build artifact の取り違え**: image tag `4c95617` だが、実際の binary が別 commit から build された可能性（要 image 内 binary の `--version` 出力等で確認）。
- 上記のいずれもまだ **検証されていない**。本 failure-log は事象記録であり、root cause は STOP ρ 後の追加調査で確定する。

## 影響範囲

- **本番劣化（rollback で復旧済み）**: revision 00020 が traffic 100% を持っていた時間帯（2026-04-29T08:36 UTC deploy 〜 2026-04-29T09:0X UTC rollback、推定 25〜30 分）、`/p/<slug>` 公開 Viewer / `/p/<slug>/report` 通報フォーム双方が `notFound()` 転落していた可能性。
- **データへの影響**: 直接的には無し。Report 送信が試されていれば `404 page not found` で Frontend が SSR 段階で notFound を返すため、reports row も outbox event も作成されない（Backend に request が届かない）。
- **設計への影響**: 「deploy 後 smoke は `/health` `/readyz` だけでは不十分」という運用の弱点が露呈。新 revision でも `health` `readyz` `OGP` が緑のままで `slug GET` だけが赤になりうる。
- **harness 上の影響**: 「同じ commit から build した binary が異なる挙動をする」という、ハーネスエンジニアリングの根本仮定（再現性）に対する疑念を生じた。要再現確認 + commit-to-binary 一致性検査の仕組み導入。

## 対策種別

- [x] ルール化（後述、deploy 後 smoke で public slug route を必須化）
- [ ] スキル化
- [x] テスト追加（`chi.Walk()` ベースの route registration テストを Backend 単体で追加検討）
- [ ] フック追加（Cloud Build / deploy 直後の自動 smoke 化は PR40 で）

## 取った対策

### 1. 即時 rollback（公開 Viewer 復旧）

- `gcloud run services update-traffic vrcpb-api --region=asia-northeast1 --to-revisions=vrcpb-api-00019-jkj=100 --project=project-1c310480-335c-4365-8a8`
- 結果: traffic 100% → `vrcpb-api-00019-jkj`（image `vrcpb-api:f4427b1`）。
- 検証: real slug `/api/public/photobooks/<対象>` → **HTTP 410 + JSON `{"status":"gone"}`**（hidden=true なので gone を返すのが正常）、fake slug → **HTTP 404 + JSON `{"status":"not_found"}`**、OGP `/api/public/photobooks/<id>/ogp` → **HTTP 200 + `{"status":"not_public","image_url_path":"/og/default.png"}`**、Workers `/p/<対象>` / `/p/<対象>/report` → **HTTP 200 + gone / 404 文字含む** moderation 連動表示 ✅。

### 2. 影響データの整理

- 対象 photobook を `hidden_by_operator=true` に再復元済み（unhide → hide 両方 cmd/ops 経由）。最終 hidden_at = 2026-04-29T08:56:22Z。
- 上記の unhide / hide で生成された outbox `photobook.unhidden` + `photobook.hidden` の pending 2 件は、`vrcpb-outbox-worker --once --max-events 1` を 2 回実行して no-op processed 化済（pending(available)=0）。
- reports / outbox `report.submitted` の追加は無し（実機 smoke を実施しなかった）。

### 3. 再発防止ルール（候補）

- **deploy 後 smoke の必須項目に `/api/public/photobooks/<bad-slug>` の handler JSON 404 確認を追加**（chi default plain text の場合は failed と判定）。
- **deploy 後 smoke の必須項目に `/api/public/photobooks/<published-target-slug>` の handler JSON 200 確認を追加**（任意の published photobook が利用できる場合）。
- 上記を満たさない deploy は traffic 100% 切り替えを保留（または直後 rollback）。
- `backend/internal/http/router_test.go`（または同等）に `chi.Walk()` で全 route が想定どおり登録されていることを確認するテーブル駆動テストを追加（routing 不在を CI で捕捉）。

## 関連

- `docs/runbook/backend-deploy.md` — deploy 手順（更新候補：上記 smoke 必須化）
- `.agents/rules/feedback-loop.md` — 失敗 → ルール化 の運用原則
- `harness/work-logs/2026-04-29_report-result.md` — PR35b 進行記録（STOP γ2 / δ2 / ε2 の経緯）
- `docs/plan/vrc-photobook-final-roadmap.md` — PR35b 関連の計画と現在地
- `harness/failure-log/2026-04-29_report-form-turnstile-bypass.md` — 同日に検出した Turnstile bypass の失敗（独立事象だが同 PR35b で連動）

## 追加調査（2026-04-29 STOP ρ 内）

rollback（traffic 100% → `vrcpb-api-00019-jkj`）後、`vrcpb-api-00020-9jz` に tag `v20`
を付与（traffic 0%）し、`https://v20---vrcpb-api-7eosr3jcfa-an.a.run.app` 経由で
revision 00020 を直接叩いて再現確認した結果:

| URL | 結果 |
|---|---|
| `/api/public/photobooks/<bad-slug>` | **`{"status":"not_found"}` HTTP 404**（handler JSON）✅ |
| `/api/public/photobooks/<対象 slug>`（hidden=true） | **`{"status":"gone"}` HTTP 410**（handler JSON）✅ |
| `/api/public/photobooks/<対象 id>/ogp` | **`{"status":"not_public",...}` HTTP 200** ✅ |
| `/health`, `/readyz` | **200 / 200** ✅ |

→ revision 00020（image `4c95617`）の **binary / route 登録は完全に正常**であり、
chi route collision / image binary 取り違え等の仮説は **棄却寄り**。

### 仮説の更新（root cause 候補の絞り込み）

- **Cloud Run deploy 直後の routing transient（最有力）**: deploy 完了直後（数分以内）の
  routing 切替期間で、L7 / instance side で短時間 chi default 落ちする挙動が起きた可能性。
  発生時刻（私が初回 smoke した時刻）は Cloud Build SUCCESS 直後（推定 5 分以内）に集中。
- **Cloud Run instance routing race condition**: 旧 / 新 revision の instance が同時に
  受け持つ瞬間で、Cloud Run の internal load balancer が一時的に不整合な状態を返した可能性。
- **chi route collision**: tag URL での再現が無いことから **棄却**。
- **build artifact 取り違え**: 同じ image SHA で route が完全に動くため **棄却**。

ただし「deploy transient」は再現性が低く、断定はしない。**運用面で smoke 強化**することで
次回の同種事象を確実に検出できる体制にした。

## 採用した再発防止策（採用日 2026-04-29 / 採用 PR PR35b 内 closeout）

`docs/runbook/backend-deploy.md` を強化（§1.4.1 / §1.4.2 / §2.1 / §3 / §7 履歴）:

- deploy / traffic 切替直後に **5〜10 分待ち**を必須化
- smoke では `/health` / `/readyz` だけで合格扱いにせず、`/api/public/photobooks/<bad-slug>` の
  **handler JSON 404** が返ることを必須確認
- chi default plain text "404 page not found" が返った場合は **failed 判定**として、
  5 分待って再確認、それでも NG なら rollback
- hidden 対象 / published 対象がある場合の handler JSON 410 / 200 確認も推奨化

## 後続候補（今回の PR35b には含めない）

- `backend/internal/http/router_test.go` 等で `chi.Walk()` を使った route registration
  テーブル駆動テストを追加し、CI で route 登録漏れを検出する仕組み（**ローンチ前の
  安全性強化タスク**として `docs/plan/vrc-photobook-final-roadmap.md` PR40 周辺で扱う）。

## 履歴

| 日付 | 変更 |
|------|------|
| 2026-04-29 | 初版作成。本事象の症状と rollback、根本原因未確定を記録。STOP ρ 後の追加調査結果を後日追記 |
| 2026-04-29 | STOP ρ 内で revision 00020 tag URL 再現確認を実施し、binary は正常と判定。最有力仮説を「deploy 直後 transient」に更新。再発防止策として `docs/runbook/backend-deploy.md` の smoke 強化（§1.4.1 / §1.4.2）を採用 |
