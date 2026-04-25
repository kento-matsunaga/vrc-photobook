# 2026-04-26 WSL cwd drift 再発で quality-check hook がエラー

## 発生状況

- **何をしようとしていたか**: M1 実環境デプロイ検証 A-1 で `harness/spike/backend/Dockerfile` を 2 バイナリ化し、ローカル `docker build` で確認していた。
- **どのファイル/モジュールで発生したか**: Bash ツール経由の `docker build`、および `.claude/settings.json` に登録された `scripts/hooks/quality-check/track-quality-execution.py` フック。

## 失敗内容

```
PostToolUse:Bash hook blocking error from command: "python3 scripts/hooks/quality-check/track-quality-execution.py":
[python3 scripts/hooks/quality-check/track-quality-execution.py]:
python3: can't open file
'/home/erenoa6621/dev/vrc_photobook/harness/spike/backend/scripts/hooks/quality-check/track-quality-execution.py':
[Errno 2] No such file or directory
```

→ Claude Code の Bash 呼び出しで `cd harness/spike/backend && docker build .` を打った結果、cwd が `harness/spike/backend` のまま残り、その後の hook が repo root 基準の相対パス `scripts/hooks/quality-check/...` を解決できなくなった。

## 根本原因

- Bash ツールは複数回の呼び出しで cwd を引き継ぐ仕様。`cd` で作業ディレクトリを変えると、その変更がセッションを通じて持続する。
- hook は **repo root を cwd として動く前提**の相対パスで設定されている（`bash scripts/hooks/track-edit.sh` 等）。
- `cd <subdir>` を使うと、以後すべての Bash 呼び出し / hook がそのサブディレクトリ起点で動いてしまい、相対パス解決が破綻する。
- 過去に同種の問題を経験しており、メモリ・ガイドライン上でも `go -C <dir>` 推奨が共有されていたが、**Docker / shell コマンドには明文化されておらず、再発を許した**。

## 影響範囲

- 本セッションで `track-quality-execution.py` が「実行されているがエラー」となり、品質計測が一部欠落した可能性。
- hook 全般（`track-edit.sh` / `capture-test-result.sh` / `check-untested.sh` / quality-check Python 群）が同様の前提に依存しており、すべて潜在的な影響あり。
- 今後 Cloud Run / Cloud SQL / Cloudflare 操作で `gcloud ...` / `wrangler ...` を `cd` 起点で実行すると、同じ問題が広範囲に再発する。

## 対策種別

- [x] ルール化（禁止事項・必須事項の追加）
  - `.agents/rules/wsl-shell-rules.md` を新規作成し、`cd` を避け、`-C` / `--workdir` / 絶対パス / サブシェルを使うことを明記する
- [ ] スキル化（手順の自動化）
- [ ] テスト追加（検出の自動化）
  - 将来的には hook 側で「呼び出し時の cwd を強制的に repo root に戻す」ようなガードを入れる（B 段階の整備で検討）
- [ ] フック追加（イベント駆動の防止策）
  - Stop hook で「最終 cwd が repo root か」を検査する案を将来検討

## 教訓

- **`cd` は 1 行コマンド内のサブシェル `( cd <dir> && ... )` に限定する**か、`-C` / `--workdir` / 絶対パスで代替する
- **Docker**: `docker build -f harness/spike/backend/Dockerfile harness/spike/backend` の形式を必須とする
- **Go**: `go -C harness/spike/backend test ./...` / `go -C harness/spike/backend build ./cmd/api` の形式を必須とする
- **hook が壊れた場合は黙って通さず即報告する**（本セッションでは Bash 出力末尾に hook エラーが出ていたが、`docker build` が成功したのでそのまま進行してしまった）
