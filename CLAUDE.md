# vrc_photobook

## プロジェクト概要

VRChat向けフォトブックサービス（詳細仕様は `docs/spec/` に記述予定）。

本リポジトリは [ai-driven-template](../ai-driven-template/) をベースに、
ハーネスエンジニアリング（Spec → Implement → Verify → Feedback）で開発する。

## ディレクトリ構造

```
vrc_photobook/
├── .agents/              # AIエージェント正規ソース（rules/skills/hooks）
├── .claude/              # Claude Code用アダプター（.agentsへのシンボリックリンク）
├── .github/workflows/    # AIレビュー + CI
├── frontend/             # フロントエンドアプリケーション
├── design/               # デザイン資産
│   ├── mockups/          # 画面モックアップ
│   ├── design-system/    # デザインシステム定義（カラー/タイポ/UI）
│   ├── figma-exports/    # Figma原本・SVGエクスポート
│   └── assets/           # ロゴ・アイコン・画像素材
├── docs/                 # ドキュメント（コードの外にある"なぜ"）
│   ├── spec/             # 仕様書（What）
│   ├── design/           # 設計書（How）
│   ├── business/         # 業務知識（Why / Domain）
│   └── adr/              # アーキテクチャ決定記録
├── harness/              # 品質管理（QUALITY_SCORE / failure-log / work-logs）
├── scripts/              # セットアップ・フック・ユーティリティ
└── tests/                # テスト（実装開始後に追加）
```

詳細な使い分けは各ディレクトリの `README.md` を参照。

## コアコンセプト: Spec → Implement → Verify → Feedback

```
Human: Spec（仕様策定）       ← docs/spec/ に記述。AIはドラフト支援
  ↓
AI Agent: Implement（実装）    ← .agents/rules に従い自律実行
  ↓
Automated + Human: Verify      ← テスト + AIレビュー + 自動承認
  ↓
失敗 → ルール化 / スキル化      ← harness/failure-log → .agents/rules
```

## 開発フロー

1. **仕様作成**: `docs/spec/` に機能仕様を記述
2. **デザイン**: Figma で作成 → `design/` にエクスポート
3. **設計**: 重要な技術決定は `docs/adr/` に記録
4. **実装**: `frontend/` で開発。`design/design-system/` を Single Source of Truth とする
5. **レビュー**: PR作成時に `@claude frontreview` でフロントエンド専用レビュー
6. **失敗の学習**: 失敗は `harness/failure-log/` → `./scripts/failure-to-rule.sh` でルール化

## AIレビュー

| コマンド | モード | 用途 |
|---------|-------|------|
| PR作成時に自動 | Standard | コードレビュー |
| `@claude deepreview` | Comprehensive | 最大9サブエージェント並列の深層レビュー |
| `@claude frontreview` | Frontend | フロントエンド4観点 + 自動承認判定 |
| `@claude {質問}` | Assist | 調査・修正・質問回答 |

## 技術スタック（未確定）

- Frontend: TBD（Next.js / Astro / Vite + React の中から選定）
- デプロイ: Cloudflare Pages 想定
- Figma: デザイン原本管理

技術選定が決まり次第、`docs/adr/0001-tech-stack.md` に記録する。

## クイックリファレンス

- ルール一覧 → `.agents/rules/`
- スキル一覧 → `.agents/skills/`
- サブエージェント → `.claude/agents/review/`
- フック設定 → `.claude/settings.json`
- 品質スコア → `harness/QUALITY_SCORE.md`
- ディレクトリ対応 → `docs/ディレクトリマッピング.md`

## ハーネス原則

> **すべてのエージェントの失敗は、再発を防止するルールまたはスキルになる。**

1. 失敗を記録（`harness/failure-log/`）
2. 原因分析
3. ルール/スキル化（`.agents/rules/` or `.agents/skills/`）
4. テストで検証可能に（`tests/`）
5. 品質スコア更新（`harness/QUALITY_SCORE.md`）
