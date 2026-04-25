---
description: "WSL / Bash ツール運用ルール — cwd drift / hook 整合性 / sudo を扱う際の安全規則"
globs: ["**/*"]
---

# WSL / Bash ツール運用ルール

## 適用範囲

Claude Code の Bash ツール、および WSL 上の Linux シェルで Git / Docker / Go / gcloud / wrangler 等の CLI を実行する全作業に適用する。

## 原則

> **作業ディレクトリは repo root（`/home/erenoa6621/dev/vrc_photobook`）に固定する。**
> サブディレクトリで実行する必要があるコマンドは、cd ではなく `-C` / `--workdir` / 絶対パス / サブシェルで指定する。

理由:
- `.claude/settings.json` に登録された hook 群（`scripts/hooks/track-edit.sh` / `track-quality-execution.py` 等）は repo root を cwd とした **相対パス前提**で動作する。
- `cd <subdir>` を使うと Bash ツールの後続呼び出し / hook がすべてそのサブディレクトリ起点で動き、相対パスが解決できず破綻する（過去事例: `harness/failure-log/2026-04-26_wsl-cwd-drift-recurrence.md`）。

## 必須パターン

### 1. `cd` の使用は最小化

```bash
# ❌ 禁止: cwd が後続呼び出し / hook に持続する
cd harness/spike/backend && docker build .

# ✅ 推奨: -f / 絶対パスで対応
docker build -f harness/spike/backend/Dockerfile harness/spike/backend

# ✅ どうしても cd が必要なら、サブシェルで完結させる
( cd harness/spike/backend && some-command )
```

### 2. Go コマンドは `-C` で対象ディレクトリを指定

```bash
# ✅ 推奨
go -C harness/spike/backend test ./...
go -C harness/spike/backend build ./cmd/api
go -C harness/spike/backend vet ./...

# ❌ 禁止
cd harness/spike/backend && go test ./...
```

### 3. Docker は `-f <Dockerfile>` + build context を絶対/相対パスで指定

```bash
# ✅ 推奨
docker build -f harness/spike/backend/Dockerfile -t vrcpb-spike-backend:local harness/spike/backend
docker build --target=build -f harness/spike/backend/Dockerfile harness/spike/backend
```

### 4. gcloud / wrangler 等の外部 CLI も同じ

- 設定ファイル（`wrangler.jsonc` / `cloudbuild.yaml` 等）を読むツールは、設定ファイルパスを引数で渡すか `--cwd` 系オプションを使う。
- `wrangler deploy --cwd harness/spike/frontend` のように明示。

### 5. `pwd` を疑ったら明示確認

意図しない cwd に入っているか不安なときは、コマンド冒頭で `pwd` を出力して確認してよい。

```bash
pwd; ls -d harness/spike/backend
```

## sudo の扱い

### 1. Claude Code Bash で sudo を使わない

- `sudo` は **tty / セッション単位**で認証チケットを管理するため、Claude Code Bash の非対話シェルでは password を渡せず常に失敗する（過去事例: `harness/failure-log/2026-04-26_sudo-noninteractive-shell-limit.md`）。
- 例: `sudo apt-get install` / `sudo systemctl` / `/etc/` 配下の書き込み等。

### 2. sudo が必要な作業はユーザー対話シェルでまとめて

- 必要なコマンドは **1 本のワンライナー**として整理し、ユーザーに `! <ワンライナー>` で対話シェル実行を依頼する。
- ワンライナーは `&&` で連結し、いずれか失敗すれば停止する形にする。

### 3. ユーザー実行後は Claude Code 側で**客観確認**

「インストール完了」「設定変更完了」のユーザー報告は宣言ベースで信用しない。Claude Code Bash 側で以下を必ず実行して実態を確認する（過去事例: `harness/failure-log/2026-04-26_gcloud-install-verification-mismatch.md`）:

- 実行ファイル: `which <cmd>` / `<cmd> --version`
- パッケージ: `dpkg -l | grep <pkg>`
- 設定ファイル: `ls -l <path>`
- 認証: `gcloud auth list` / `gcloud projects list` 等

## hook 整合性

### 1. hook が壊れたら即報告し、進行を止める

- Bash ツール出力末尾に `PostToolUse hook blocking error` 等が出ている場合、**たとえそのコマンド自体が成功していても、報告で明示**する。
- hook エラーの典型は `not found` / 相対パス解決失敗 → cwd drift を疑う。
- cwd drift を確認したら `cd /home/erenoa6621/dev/vrc_photobook` で repo root に戻す。

### 2. hook が前提とする相対パスを変える場合

- `.claude/settings.json` 上の `bash scripts/hooks/...` のパス変更は repo root 前提で行う。
- hook スクリプトが内部で相対パスを使う場合は、スクリプト先頭で `cd "$(git rev-parse --show-toplevel)"` 等で repo root に揃えるのが安全（B 段階の整備で検討）。

## ファイル操作

### 1. Read / Edit / Write は絶対パス

- Claude Code の Read / Edit / Write ツールは絶対パスを要求する。`/home/erenoa6621/dev/vrc_photobook/...` から始める。
- Bash 経由の `cat` / `sed` / `grep` も絶対パスを優先（Read / Edit / Write を使えば不要）。

### 2. `.env.local` / `.env` 系の中身を表示しない

- `cat .env.local` / `printenv | grep <SECRET>` / 値を引数にした `grep -F` は禁止。
- 値の存在確認は `[ -n "$VAR" ] && echo set || echo unset` のような形にとどめる。

## Why（なぜこのルールが必要か）

- Claude Code の hook は repo root を起点にした相対パス前提で組まれており、cwd drift で確実に壊れる。
- 過去に同種の問題を経験したが、口頭ガイドだけで明文化していなかったため再発した（`harness/failure-log/2026-04-26_wsl-cwd-drift-recurrence.md`）。
- sudo / 認証 / install 系は「対話シェルと Claude Code Bash が別環境」であることを忘れると、ユーザーから見て「完了」報告と Claude Code 側の状態認識がずれて手戻りが発生する。
- 本ルールは検証済みの最小ガードであり、B 段階以降で hook 側の自動 cwd 補正・Stop hook での状態検査を追加する余地を残す。

## 関連

- `.agents/rules/coding-rules.md`（コーディング基本ルール）
- `.agents/rules/security-guard.md`（Secret / 認証情報の扱い）
- `.agents/rules/feedback-loop.md`（失敗 → ルール化の運用）
- `harness/failure-log/2026-04-26_wsl-cwd-drift-recurrence.md`
- `harness/failure-log/2026-04-26_gcloud-install-verification-mismatch.md`
- `harness/failure-log/2026-04-26_sudo-noninteractive-shell-limit.md`
- `harness/failure-log/2026-04-26_gcp-account-billing-mismatch.md`

## 履歴

| 日付 | 変更 |
|------|------|
| 2026-04-26 | 初版作成。M1 実環境デプロイ着手前のハーネス補強として、cwd drift / sudo / install 検証 / hook 整合の最低限ルールを明文化 |
