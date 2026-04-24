# docs/

プロジェクトのドキュメント集約場所。「コードの外にある"なぜ"」を記述する。

## ディレクトリ構成

| パス | 目的 | 例 |
|------|------|----|
| `spec/` | 仕様書 | 機能仕様、ユーザーストーリー、受け入れ基準 |
| `design/` | 設計書 | アーキテクチャ、データモデル、API設計、シーケンス図 |
| `business/` | 業務知識 | ドメインモデル、業務ルール、用語集 |
| `adr/` | アーキテクチャ決定記録 | 技術選定理由、設計トレードオフの記録 |
| `ディレクトリマッピング.md` | コード⇔docs対応 | sync-check スキルが参照 |

## 使い分け

- **spec/**: 「何を作るか」（What）
- **design/**: 「どう作るか」（How）
- **business/**: 「なぜ作るか／業務上の制約」（Why / Domain）
- **adr/**: 「なぜこの技術・設計にしたか」（Why this choice）

## ADR フォーマット

`adr/NNNN-title.md` の形式で連番管理。テンプレート:

```markdown
# ADR-NNNN: タイトル

- Status: Proposed / Accepted / Deprecated / Superseded
- Date: YYYY-MM-DD

## Context
## Decision
## Consequences
```

## フロントデザインとの棲み分け

- ビジュアルデザイン原本・素材 → `/design/`
- デザイン意図・設計思想の記述 → `docs/design/ui-design-policy.md`
