# .agents/ — AI エージェント設定の正規ソース

## 設計思想

`.agents/` はすべてのAIツール（Claude Code, Cursor, Codex）が参照する **Single Source of Truth**。
各ツール固有のディレクトリ（`.claude/`, `.cursor/`, `.codex/`）はシンボリックリンクで接続する。

## シンボリックリンク・トポロジー

```
.agents/rules/    ← 正規ソース
  ├── .claude/rules/    (symlink)    Claude Code が自動読込
  ├── .cursor/rules/    (変換コピー)  Cursor が .mdc 形式で読込
  └── .codex/rules/     (symlink)    Codex が読込

.agents/skills/   ← 正規ソース
  ├── .claude/skills/   (symlink)
  └── .codex/skills/    (symlink)

.agents/hooks/    ← 正規ソース
  └── .claude/settings.json に変換
```

## ファイル規約

### ルール（rules/）
- 1ファイル = 1ルール
- フロントマター: `description`, `globs`（適用対象パターン）
- 本文: ルール内容 + `# Why:` セクション（なぜこのルールが生まれたか）

### スキル（skills/）
- 1ディレクトリ = 1スキル
- `SKILL.md` — スキル定義（入力・手順・出力）
- 追加ファイル: テンプレート、設定ファイル等

### フック（hooks/）
- イベント駆動の自動実行設定
- `README.md` — フックの説明と設定方法

## 新規追加手順

1. `.agents/` 内に正規ファイルを作成
2. 対応ツールのディレクトリにシンボリックリンクを作成
3. `scripts/setup.sh` を更新（新規プロジェクト適用時に含まれるよう）
4. `tests/harness_test.sh` にリンク整合性テストを追加
