# 2026-04-26 PR10 作業中の cwd drift 再発

## 発生状況

PR10（Frontend Route Handler）の作業で、E2E ローカル確認用に backend 配下に一時ファイルを置こうとして、Bash で `cd /home/erenoa6621/dev/vrc_photobook/backend && go run ...` を実行した。

その結果、後続の `python3 scripts/hooks/quality-check/track-quality-execution.py` フックが、cwd が `backend/` に切り替わったまま起動してしまい、`scripts/hooks/...` の相対パスを解決できず以下のエラーで blocking した。

```
PostToolUse:Bash hook blocking error: python3: can't open file
'/home/erenoa6621/dev/vrc_photobook/backend/scripts/hooks/quality-check/track-quality-execution.py':
[Errno 2] No such file or directory
```

## 失敗内容

- `cd <subdir> && cmd` の形式で cwd が後続呼び出しに持続
- `.agents/rules/wsl-shell-rules.md` §1 の「`cd` の使用は最小化」「サブシェルで完結させる」に明確に違反
- 同じ事象は `2026-04-26_wsl-cwd-drift-recurrence.md` で既に記録済みだったが、PR10 で再発した

## 根本原因

- 本ルールが頭から抜けていた。具体的には `go -C <dir>` / `docker -f <path>` / `npm --prefix <dir>` のいずれも使える状況だったのに、安易に `cd && ...` を選んでしまった
- backend 配下に internal package 制約越しに main を作ろうとして、慌てて `cd` を打ったのが直接原因

## 影響範囲

- Bash hook が blocking error を返した（コマンド自体は成功していたが、報告で明示が必要）
- 一時的に repo root を見失い、その後 `cd /home/erenoa6621/dev/vrc_photobook` で復帰
- 作業の手戻り 1 ターン分

## 対策種別

- [x] 既存ルール（`wsl-shell-rules.md`）への違反のため、ルール変更は不要
- [x] 失敗事例として再記録し、再発防止の意識を強化
- [ ] 自動検出フック（cwd drift を検出して即警告）は検討余地あり、ただしコスト高なので保留

## 再発防止メモ

- backend 配下で何か実行する必要が出たら、まず `go -C backend ...` で書けないかを確認
- どうしてもサブディレクトリで `cd` したいときは `( cd subdir && cmd )` のサブシェルで完結
- 一時的な探索・スクラッチ作業でも `cd` は repo root から離れない

## 関連

- [`.agents/rules/wsl-shell-rules.md`](../../.agents/rules/wsl-shell-rules.md)
- [`harness/failure-log/2026-04-26_wsl-cwd-drift-recurrence.md`](./2026-04-26_wsl-cwd-drift-recurrence.md)
