# Public 化前 Security チェックリスト

> 作成日: 2026-04-27（Security / Domain Integrity Audit PR）
> 位置付け: GitHub repository を Public 化する前に通すゲート。Cloudflare R2 /
> Turnstile / Cloud SQL / Secret Manager の実 Secret が登録される PR21 / PR22 で
> 実値混入のリスクが高まるため、**段階的に複数回参照する運用**を想定する。
>
> 上流参照:
> - [`.agents/rules/security-guard.md`](../../.agents/rules/security-guard.md)
> - [`.agents/rules/wsl-shell-rules.md`](../../.agents/rules/wsl-shell-rules.md)
> - [`README.md`](../../README.md) 「重要な運用ルール」

---

## 0. 本チェックリストの使い方

- 本ドキュメント自体は **公開可**（実値を含めない設計）
- Public 化を**実施する直前**に各セクションを順に確認
- 1 つでも未確認 / 未対処があれば **Public 化を保留**
- Public 化後も **R2 / Turnstile / Secret 操作 PR ごとに再スキャン**

---

## 1. 全コミット履歴の secret scan

GitHub Push Protection / TruffleHog 等での自動 scan に頼らず、ローカルでも事前確認する。

```sh
# 全コミット履歴を grep（path フィルタなし）
git log --all -p | grep -InE "DATABASE_URL=|PASSWORD=|SECRET_KEY|API_KEY|sk_live|sk_test|TURNSTILE_SECRET_KEY=|R2_SECRET_ACCESS_KEY=|R2_ACCESS_KEY_ID=" || echo "NO_LITERAL_SECRET_VALUES_IN_HISTORY"
```

許容するヒット:
- ローカル開発 placeholder (`vrcpb_local` / `<your-account-id>` / `<m1-spike-token-secret>` 等)
- Secret 名（識別子）への言及
- ADR / 設計書で**型名としての記述**（`TURNSTILE_SECRET_KEY` の Secret Manager 名等）

許容しないヒット:
- 実 hex / base64 / UUID の値
- 本番 endpoint URL のうち `<account_id>` 部分が実値で埋まっているもの
- production の draft / manage / session token / Cookie 値
- gcloud コマンドの `--data-file=-` パイプ前の値（万一）

---

## 2. .env / config / Cookie / token / Secret の値検査

### 2.1 git tracked file での検査

```sh
grep -RInE "DATABASE_URL=|PASSWORD=|SECRET_KEY|API_KEY|sk_live|sk_test|TURNSTILE_SECRET_KEY=|R2_SECRET_ACCESS_KEY=|R2_ACCESS_KEY_ID=|draft_edit_token=|manage_url_token=|session_token=|Set-Cookie:" \
  $(git ls-files) || echo "OK"
```

### 2.2 .gitignore で除外されている値ファイルの確認

```sh
# 各 .env が gitignore で除外されているか
for f in frontend/.env.production frontend/.env.local backend/.env backend/.env.local harness/spike/backend/.env harness/spike/frontend/.env.local; do
  if [ -f "$f" ]; then
    git check-ignore -v "$f" && echo "$f: ignored OK" || echo "$f: NOT IGNORED!"
  fi
done
```

### 2.3 本番固有の値（Public 化前に必ず削除）

- 実 R2 Account ID（`*.r2.cloudflarestorage.com` の `*` 部分）
- 実 Cloud SQL connection string
- 実 Cloud Run URL（`*.a.run.app` の hash 部分）
- 実 Cloudflare Turnstile sitekey は **公開値**（除外不要）
- Cloudflare 公式 test sitekey (`1x0000...`) も **公開値**（除外不要）

---

## 3. work-logs / harness の公開可否確認

```sh
# 全 work-logs を grep
grep -RInE "DATABASE_URL=|PASSWORD=|SECRET=|TURNSTILE_SECRET_KEY=|R2_SECRET_ACCESS_KEY=" harness/work-logs/ || echo "OK"
```

### 3.1 公開すべきか判断する観点

- 内容が「実 Secret / 実 token / 実 Cookie 値」を含まない作業ログ → 公開可
- 検証手順 / 計画 / 結果（成功 / 失敗）→ 公開可
- 実 endpoint URL（`*.workers.dev` / `*.r2.cloudflarestorage.com` の本番値）→
  既に公開ドメイン（`app.vrc-photobook.com` / `api.vrc-photobook.com`）を
  使うため公開しても新たな漏洩面はない

### 3.2 削除 / Sanitize する候補

- ない（2026-04-27 時点）

### 3.3 失敗ログ（`harness/failure-log/`）

- 失敗内容と対策のみで実 Secret は含まない設計
- ただし最終確認: `grep -RInE "..." harness/failure-log/`

---

## 4. README / docs の公開向け調整

### 4.1 README.md

- [ ] サービス概要 / 使い方が公開向けに書かれているか
- [ ] 内部 URL（spike / verify 系）が記載されていないか
- [ ] 「重要な運用ルール」が外部読者にも読めるか
- [ ] 開発者向けの secret 取扱注意が公開向けに書かれているか

### 4.2 docs/plan / docs/adr / docs/design / docs/security

- [ ] 計画書 / ADR / 設計書に実値が混入していないか
- [ ] テスト用 placeholder（`vrcpb_local` / `<your-...>`）のみか
- [ ] M1 spike の R2 / Cloud Run の内部 URL は spike 専用と明記されているか

### 4.3 work-logs

- 公開しない選択肢もあり（`/harness/work-logs/` を `.gitignore` 追加で除外）
- 公開する場合は §3 のスキャンで OK となること

---

## 5. GitHub visibility 変更前の最終ゲート

```sh
# Public にする直前
unset GITHUB_TOKEN
gh repo view kento-matsunaga/vrc-photobook --json visibility,name,description
```

### 5.1 変更前確認

- [ ] §1 全履歴 scan = NO_LITERAL_SECRET_VALUES_IN_HISTORY
- [ ] §2 .env 除外 OK
- [ ] §3 work-logs / failure-log scan OK
- [ ] §4 README / docs 公開向け OK
- [ ] 直近 24 時間以内に Force push / amend していないことを確認

### 5.2 変更コマンド（ユーザー対話シェルで実施）

```sh
gh repo edit kento-matsunaga/vrc-photobook --visibility public --accept-visibility-change-consequences
```

### 5.3 変更直後の確認

- [ ] `gh repo view ... --json visibility` で `PUBLIC` 確認
- [ ] GitHub web UI で Issue / PR / Actions が想定通り公開されているか
- [ ] Star / Watch / Fork カウントが Public 仕様で表示されるか

---

## 6. GitHub Free + Public での branch protection

GitHub Free + Private は branch protection 不可（README に既存記述）。
Public 化すると Free でも以下が使える:

- branch protection rules（main への direct push 禁止 / PR 必須 / status checks 必須）
- CODEOWNERS
- Required reviewers

### 6.1 Public 化と同時に設定推奨

- [ ] `main` を default branch に保持
- [ ] branch protection: Require pull request before merging（自分のみの repo でも、
  履歴整理の意味で）
- [ ] Force push 禁止
- [ ] Branch deletion 禁止

ただし PR 必須にすると 1 人運用が大変なので、**当面は force push 禁止 + branch
deletion 禁止 + status checks 必須まで**を推奨。

---

## 7. GitHub Actions / CI / status checks

- [ ] Actions が機密値を出力していないか（log で `--debug` 系を含むか確認）
- [ ] Secrets が repo Settings に登録されているか
- [ ] PR builds で `pull_request` から secret にアクセスする権限制御
- [ ] Codecov / Codecov 等の外部 CI に secret を出していないか

PR21 までで Backend test を CI で走らせる予定がない場合は本セクションは保留。

---

## 8. Public 化する / しないの判断ポイント

### 8.1 Public 化する利点

- ポートフォリオ / 採用面接で見せられる
- branch protection が使える（Free プランでも）
- Issue / Discussion の Public 投稿で外部知見を得られる
- 設計 / ADR / 計画を世界に Open Source 的に共有できる

### 8.2 Public 化する欠点

- Cloudflare R2 / Turnstile / Cloud SQL / Cloud Run の本番値が誤って混入したら
  即時悪用リスク
- M1 spike / 検証中の URL を見られる（PR12 で domain mapping 完了済のため、
  spike URL は使われていない）
- VRChat 関連の表現規定 / コミュニティルール考慮（業務知識 v4 §6 / 表現規定）
- Issue / PR で外部から「コードを盗用された」「クローンが出た」リスク

### 8.3 段階的判断

- **PR21 完了前**: Public 化を保留（Cloudflare R2 / Cloud Run secret が直近で動く）
- **PR21 完了後 + 1 週間 secret 安定**: Public 化検討タイミング
- **PR22 / PR23 完了後**: Public 化候補（Frontend / image-processor 含めて開示可）

---

## 9. 実 Secret 登録後の再スキャン（必須）

PR21 / PR22 で R2 / Turnstile の実 Secret を登録した後、**必ず再スキャン**:

```sh
# Secret 登録 PR の commit 直前 + 直後
grep -RInE "DATABASE_URL=|R2_SECRET_ACCESS_KEY=|R2_ACCESS_KEY_ID=|TURNSTILE_SECRET_KEY=" $(git diff --name-only HEAD~5..HEAD)
git log --all -p HEAD~5..HEAD | grep -InE "DATABASE_URL=|R2_SECRET_ACCESS_KEY=|R2_ACCESS_KEY_ID=|TURNSTILE_SECRET_KEY="
```

---

## 10. 関連

- [`.agents/rules/security-guard.md`](../../.agents/rules/security-guard.md) — Secret / Cookie / token 全般
- [`.agents/rules/wsl-shell-rules.md`](../../.agents/rules/wsl-shell-rules.md) — `cat .env` 禁止 / sudo 制約
- [`.agents/rules/domain-standard.md`](../../.agents/rules/domain-standard.md) — 集約子テーブル / OCC / now() 方針
- [`README.md`](../../README.md) — 「重要な運用ルール」
- [`docs/plan/m2-r2-presigned-url-plan.md`](../plan/m2-r2-presigned-url-plan.md) — R2 Secret 登録手順
- [`docs/plan/m2-upload-verification-plan.md`](../plan/m2-upload-verification-plan.md) — Turnstile Secret 登録手順
