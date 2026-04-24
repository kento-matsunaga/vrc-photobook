---
name: "pull-request-creation"
description: "PR作成ガイドライン — 品質を担保したプルリクエストを作成する"
---

# プルリクエスト作成スキル

## 入力
- 完了したタスクのブランチ
- self-verification レポート（PASS であること）

## 前提条件
- self-verification スキルが PASS であること
- すべてのテストが通っていること

## 手順

### Step 1: 変更サマリー作成
```bash
git diff main --stat
git log main..HEAD --oneline
```

### Step 2: PR テンプレート

```markdown
## 概要
{1-3文で変更の目的を説明}

## 変更内容
- {変更点1}
- {変更点2}

## テスト
- [ ] ユニットテスト追加/更新
- [ ] 既存テスト全パス
- [ ] self-verification PASS

## 品質チェック
- [ ] テストガイドライン準拠
- [ ] ドメインモデルルール準拠
- [ ] セキュリティガードルール準拠

## レビュー観点
{レビュアーに特に確認してほしい点}
```

### Step 3: 自己検証の最終確認
PR作成前に self-verification を再実行する。

## 出力
- PR（GitHub）
