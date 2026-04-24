# 品質チェックフック（3点セット）

## 概要

ソースファイルの変更を追跡し、品質パイプライン（build → lint → test）が
完了するまでリマインダーを出し続ける3点フックシステム。

## フック構成

### 1. track-quality-edits.py（afterFileEdit）
- **トリガー**: ソースファイル編集時
- **動作**: 変更ファイルを記録し、品質フラグ（build/lint/test）をリセット
- **対象**: `.go`, `.ts`, `.tsx`, `.py`, `.rs` 等（テストファイルは除外）

### 2. track-quality-execution.py（afterShellExecution）
- **トリガー**: シェルコマンド実行後
- **動作**: build/lint/testコマンドを検知し、成否に基づいてフラグを更新
- **検知例**: `go test`, `npm test`, `make lint`, `eslint`, `cargo build` 等

### 3. stop-quality-check.py（stop）
- **トリガー**: エージェント停止時
- **動作**: 未完了の品質チェックがあればリマインダーを出力
- **ループ上限**: 4回（無限ループ防止）

## 状態管理

- 状態ファイル: `/tmp/ai-driven-quality-state/quality-state.json`
- フラグ: `build_ok`, `lint_ok`, `test_ok`
- ファイル変更があるたびにフラグはリセットされる
- 全チェック完了で状態ファイルは自動削除

## カスタマイズ

### コマンド検知パターンの追加
`track-quality-execution.py` の `BUILD_PATTERNS`, `LINT_PATTERNS`, `TEST_PATTERNS` を
プロジェクトに合わせて編集してください。

### 対象ファイル拡張子の追加
`track-quality-edits.py` の `SOURCE_EXTENSIONS` を編集してください。

## セットアップ

`.claude/settings.json` に以下を追加:

```json
{
  "hooks": {
    "afterFileEdit": [{
      "command": "python3 scripts/hooks/quality-check/track-quality-edits.py",
      "timeout": 5000
    }],
    "afterShellExecution": [{
      "command": "python3 scripts/hooks/quality-check/track-quality-execution.py",
      "timeout": 10000
    }],
    "stop": [{
      "command": "python3 scripts/hooks/quality-check/stop-quality-check.py",
      "timeout": 5000
    }]
  }
}
```
