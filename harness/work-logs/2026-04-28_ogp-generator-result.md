# PR33b OGP generator 実装結果（2026-04-28）

## 概要

- 新正典 PR33 / `docs/plan/m2-ogp-generation-plan.md` PR33a 計画に従い、OGP 画像生成
  Backend 基盤を実装
- migration 00013（`photobook_ogp_images`）を Cloud SQL `vrcpb-api-verify` に適用済（STOP α）
- Cloud Build manual submit で `vrcpb-api:30f9949` を deploy 済、`vrcpb-api-00014-9sk` に
  traffic 100%（STOP β、`traffic-to-latest` step 通算 3 回目稼働）
- ローカル CLI から本番 R2 PUT PoC を 1 件実施 → DB row + R2 object 確認 → cleanup（STOP γ）
- Workers proxy / Frontend metadata / Outbox handler / Cloud Run Jobs / Scheduler は
  範囲外（PR33c 以降）

## ファイル追加 / 更新（commit `30f9949`）

| ファイル | 役割 |
|---|---|
| `backend/migrations/00013_create_photobook_ogp_images.sql` | photobook_ogp_images 単独 schema（5 status / FK CASCADE+SET NULL / 200 char check / 整合 CHECK） |
| `backend/internal/ogp/domain/ogp_image.go` | OgpImage entity + NewPending / Restore / MarkGenerated / MarkFailed / MarkStale |
| `backend/internal/ogp/domain/vo/ogp_status/ogp_status.go` | 5 値 VO（pending / generated / failed / fallback / stale） |
| `backend/internal/ogp/domain/vo/ogp_version/ogp_version.go` | int >= 1 + Increment |
| `backend/internal/ogp/domain/vo/ogp_failure_reason/ogp_failure_reason.go` | 200 char + Secret パターン redact（PR31 worker と同方式） |
| `backend/internal/ogp/infrastructure/repository/rdb/queries/ogp.sql` | 6 query（Find / CreatePending / MarkGenerated / MarkFailed / MarkStale / ListPending） |
| `backend/internal/ogp/infrastructure/repository/rdb/sqlcgen/*.go` | sqlc 生成 |
| `backend/internal/ogp/infrastructure/repository/rdb/ogp_repository.go` | Repository 実装、ErrNotFound 公開 |
| `backend/internal/ogp/infrastructure/renderer/renderer.go` | 1200×630 PNG renderer。go:embed Noto Sans JP Regular/Bold OTF（OFL 1.1）、cover thumbnail / title 折返し / fallback / panic recover |
| `backend/internal/ogp/infrastructure/renderer/fonts/{NotoSansJP-Regular,Bold}.otf` | フォント本体（合計 8.8MB、SubsetOTF/JP） |
| `backend/internal/ogp/infrastructure/renderer/fonts/LICENSE.txt` | OFL 1.1 ライセンス |
| `backend/internal/ogp/internal/usecase/generate_ogp.go` | UseCase（photobook 確認 → row ensure → render → R2 PUT → 失敗時 MarkFailed） |
| `backend/internal/ogp/wireup/wireup.go` | cmd 用 facade（Runner、--photobook-id / --all-pending dispatch） |
| `backend/cmd/ogp-generator/main.go` | CLI（--photobook-id / --all-pending / --max-events / --timeout / --dry-run） |
| `backend/Dockerfile` | `go build -o /out/ogp-generator ./cmd/ogp-generator` + COPY /usr/local/bin/ |
| `backend/sqlc.yaml` | ogp set 追加（schema は health + photobook 関連 + outbox 関連 + 00013） |
| `backend/internal/ogp/{*,*}_test.go` | VO 7 / renderer 9 / Repository 4 / UseCase 4 ケース |

## 重要な縮小判断（PR33b 範囲）

UseCase は **renderer + R2 PUT までで停止**し、`images.usage_kind='ogp'` 行作成 +
`MarkGenerated` は実施しません。理由は CHECK 制約「status='generated' → image_id NOT NULL」
を満たすには `images` table の正規行（normalized_format / dimensions / byte_size 等
多数の NOT NULL）が必要で、PR33b の最小範囲を超えるためです。

storage_key prefix と `ogp_id` の整合は維持（`photobooks/<pid>/ogp/<ogp_id>/<random>.png`、
ADR-0005）。完了化（status='generated'）は PR33c で images row 作成と組で実施します。

## STOP α: Cloud SQL migration 適用結果

| 観点 | 結果 |
|---|---|
| migration version | 12 → **13**（00013 のみ Applied、224.2ms） |
| `photobook_ogp_images` table 存在 | ✓ |
| index 数 | 3（PK / `photobook_id UNIQUE` / `status_updated_idx`） |
| constraint 数 | 14（PK / UNIQUE / 5 CHECK / 2 FK / NOT NULL 群） |
| 行数 | 0 |
| 既存 photobooks rows | 11（PR30 STOP A と一致） |
| 既存 images / image_variants / sessions / upload_verification_sessions / outbox_events | 6 / 0 / 10 / 5 / 0（変動なし） |
| 既存 revision `vrcpb-api-00013-l9s` への影響 | なし |

## STOP β: Cloud Build manual submit deploy 結果

| 観点 | 値 |
|---|---|
| Build ID | `9d01d397-9379-418c-96b0-ee091b791cc4` |
| Build duration | 3M55S |
| Cloud Build 5 steps | build / push / deploy / **traffic-to-latest** / smoke すべて SUCCESS |
| 新 image | `vrcpb-api:30f9949` |
| 新 revision | `vrcpb-api-00014-9sk`（traffic 100%） |
| `latestReadyRevisionName == status.traffic[0].revisionName` | ✓ |
| `/health` / `/readyz` | 200 / 200 |
| edit-view / publish / manage / upload-intent no Cookie | 全 401 |
| env / secretKeyRef | 9 個維持 |
| Cloud Run command | None（Dockerfile CMD `/usr/local/bin/api` のまま） |
| ogp-generator 同梱 | ✓（Build logs Step 6/14 で `go build -o /out/ogp-generator ./cmd/ogp-generator`、Step 11/14 で `COPY --from=build /out/ogp-generator /usr/local/bin/ogp-generator`） |
| Cloud Build logs Secret 漏洩 | 0 件（229 行スキャン） |
| Cloud Run logs Secret 漏洩（新 revision） | 0 件（15 行スキャン） |

## STOP γ: ローカル CLI で R2 PUT PoC 結果

### 対象選定

- 状態別件数: published 3 / draft 7 / deleted 1
- 選定: published かつ hidden_by_operator=false の中で `created_at` 最古 1 件
- UUID は **チャット / log / commit / work-log 一切に出さない**（実 PoC 中は環境変数 PID 経由）
- UUID 長さ 36 文字（uuid 形式）/ prefix 上 2 文字を `<頭2文字>***` で目印表示のみ

### dry-run 結果

```
ogp-generator starting (photobook_id=<PID>, all_pending=false, dry_run=true, max_events=50, timeout=90s)
ogp dry-run: rendered, would upload to R2 (ogp_image_id=019dd13f-9c52-..., photobook_id=<PID>, png_bytes=19590)
ogp-generator finished (picked=1, rendered=1, uploaded=0, failed=0, skipped=0)
```

- DB row: 作成されず（dry-run は CreatePending を skip、PR33b 仕様）
- R2 PUT: 実行されず

### 本番 PUT 結果

```
ogp-generator starting (photobook_id=<PID>, all_pending=false, dry_run=false, max_events=50, timeout=90s)
ogp rendered and uploaded (ogp_image_id=019dd13f-d79a-..., photobook_id=<PID>, png_bytes=19590)
ogp-generator finished (picked=1, rendered=1, uploaded=1, failed=0, skipped=0)
```

- 本番 R2 PUT 1 件成功（PNG 19590 bytes）
- 別 ogp_image_id（dry-run の row は永続化されないため別 UUID）

### DB / R2 検証

| 観点 | 結果 |
|---|---|
| `photobook_ogp_images` rows where photobook_id=<PID> | **1** |
| status | **pending**（PR33b 仕様、`MarkGenerated` 実施なし） |
| version | 1 |
| image_id_set | false（PR33b は images row 作成しない） |
| R2 objects under `photobooks/<PID>/ogp/` | 1（size 19590 bytes、key 末尾は `<prefix>019dd13f***` で redact） |

### public 取得不可確認

- R2 bucket `vrcpb-images` は **public OFF 維持**（PR33b で bucket 設定を一切変更していない）
- Workers proxy / Backend public endpoint も未実装（PR33c で構築予定）
- 結果: public ドメイン経由での取得経路自体が無いため、外部から OGP object に到達不可
- 計画書 §5.3 / cross-cutting/ogp-generation.md §6 と整合

### Cleanup

| 操作 | 結果 |
|---|---|
| R2 object 削除 | 1 個削除、cleanup 後 0 件 |
| `photobook_ogp_images` row 削除 | 1 行削除、cleanup 後 photobook_id=<PID> の row は 0 件 |
| cloud-sql-proxy 停止 | PID 確認 → 停止確認 |
| /tmp/{cspr-stopg.log,cspr-stopg.pid,stopg-pid.txt,stopg-probe.go,stopg-pick.go,stopg-verify-cleanup.go} | 全削除 |
| env から DATABASE_URL / R2_* / PG_* / PID | 全 unset |
| shell history | clear |

## Test 結果

| 範囲 | ケース | 結果 |
|---|---|---|
| `internal/ogp/domain/vo/ogp_failure_reason` | sanitize 6 ケース（短い / 200 超 / DATABASE_URL redact / presigned redact / nil / FromTrustedString 上限） | 全 pass |
| `internal/ogp/domain/vo/ogp_status` | Parse 7 ケース + Predicates | 全 pass |
| `internal/ogp/infrastructure/renderer` | render 8 ケース（ASCII / 日本語 / 80 文字折返し / creator 50 文字 / emoji / cover nil / cover decode 失敗 / 空 input）+ Secret 非埋め込み確認 | 全 pass |
| `internal/ogp/infrastructure/repository/rdb` | CreatePending / FindByPhotobookID / NotFound / MarkFailed / 200 char CHECK 違反（4 ケース、DB 必須） | 全 pass |
| `internal/ogp/internal/usecase` | Success / NotPublishedSkipped / R2PutFailureMarksFailed / DryRunNoPut（4 ケース、DB + fake R2） | 全 pass |
| `go vet ./...` / `go build ./...` | クリーン |

DB 必須 test は `go test -p 1 ./internal/ogp/...` で実行（同 DB を共有する別 package
test と並列衝突するため `-p 1` 必須、PR31 と同方針）。

## PR closeout チェックリスト（pr-closeout.md §6）

- [x] **コメント整合チェック実施**: `bash scripts/check-stale-comments.sh --extra "OGP|og:image|twitter:card|SNS|crawler|R2 public|public URL|Cloud Run Jobs|Scheduler|ogp-generator"`
- [x] **古いコメント修正**: 該当なし（PR33b 範囲のヒットは全て C 過去経緯 / B 状態ベース）
  - C: 計画書 / outbox-plan の `PR23 image-processor` / `PR25 公開 Viewer` / `PR30 Outbox` / `PR31 outbox-worker` 参照
  - B: 本実装内の「PR33c で images row 作成と組で完了」のような状態ベース TODO（新正典 §3 PR33 と整合）
- [x] **残した TODO とその理由**:
  - C 過去経緯: `internal/image/.../storage_key.go:132` 「PR18 では ogp_id は別集約」（既存、変更なし）
  - B 状態ベース: cmd/ogp-generator / generate_ogp.go の「PR33c で images row 作成と組で完了させる」「Workers proxy で配信は PR33c」
- [x] **先送り事項の記録**:
  - PR33c（Workers proxy + Frontend metadata + SNS validator）: 計画書 §2 / 新正典 §3 PR33 に明記済
  - PR33d（Outbox handler + Cloud Run Jobs）: 計画書 §2 / m2-outbox-plan.md 履歴に明記済
  - PR33e（Reconcile）: 計画書 §2 で任意扱い
- [x] **generated file 未反映コメント**: 該当なし（sqlc 再生成済）
- [x] **Secret 漏洩 grep**: 実値 0 件（既存ローカル dev DSN / Turnstile 公式 test key の用語ヒットのみ、PR33b で追加したコード / 本 work-log に実値含まず）

## Secret 漏洩がないこと（明示確認）

- DATABASE_URL 実値: 一時 env 経由のみ、unset 済、commit / chat / log に未含有
- R2 credentials 実値: 同上、unset 済
- 対象 photobook UUID: 環境変数 PID + /tmp/stopg-pid.txt で短期保持、cleanup で削除済、commit / chat / work-log に未含有
- storage_key 完全値: CLI 出力 / repo の grep で生 prefix の `photobooks/<実UUID>/ogp/...` は **未出現**（log は `<PID>` で redact、verify ツールも `mask` で 8 文字 + `***`）
- 生成された 2 つの ogp_image_id（dry-run / 本番）は uuid v7 で **公開可能な内部識別子**（業務知識 v4 §3.5 の「公開識別子レベル」、token / Secret ではない）
- shell history clear / tmp file 全削除 / cloud-sql-proxy 停止確認済

## 実施しなかったこと（PR33b 範囲外）

- Workers proxy（Cloudflare Workers binding 追加）→ PR33c
- Frontend `generateMetadata` 更新（og:image を生成 URL に切替）→ PR33c
- Backend `/api/public/photobooks/<id>/ogp` endpoint → PR33c
- SNS validator 実機確認（X / Discord / Slack）→ PR33c
- macOS / iPhone Safari 実機確認 → PR33c
- Outbox handler の OGP 連携（`photobook.published` → 自動生成）→ PR33d
- Cloud Run Jobs / Scheduler 作成 → PR33d
- images table の usage_kind='ogp' 行作成 + `MarkGenerated` → PR33c
- Reconcile（stale_ogp_enqueue / 手動 ogp_stale.sh）→ PR33e（任意）
- Email Provider 連携 / ManageUrlDelivery → 別 PR ライン（PR32c 以降）
- Cloud SQL 削除 / spike 削除 / Public repo 化

## 関連ロードマップ更新

PR33a で更新済（本 PR で追加更新なし）:
- 新正典 §3 PR33 段階分割（PR33a/b/c/d/e）
- m2-outbox-plan.md 履歴（Cloud Run Jobs 初回稼働は PR33d で副作用 handler と組）

## 履歴

| 日付 | 変更 |
|------|------|
| 2026-04-28 | 初版（PR33b 完了）。OGP renderer + Repository + UseCase + CLI + Dockerfile 同梱。STOP α (migration 13 適用) / β (Cloud Build deploy `vrcpb-api-00014-9sk` traffic 100%) / γ (ローカル CLI で R2 PUT PoC + DB row 確認 + cleanup) すべて完了。**PR33c（Workers proxy / Frontend metadata）に進む準備完了** |
