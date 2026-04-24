# 品質チェックリマインダーフック

## 概要

ソースファイルの変更を追跡し、品質パイプライン（build → lint → test）が
完了するまでリマインダーを出し続ける3点フックシステム。

## 詳細

実装とセットアップの詳細は `scripts/hooks/quality-check/README.md` を参照。

## フック構成

| フック | トリガー | 実装 |
|-------|---------|------|
| track-quality-edits | afterFileEdit | Python（状態管理） |
| track-quality-execution | afterShellExecution | Python（コマンド検知） |
| stop-quality-check | stop | Python（リマインダー出力） |
