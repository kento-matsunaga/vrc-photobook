# 2026-04-26 GCP プロジェクト所有者と gcloud ログインアカウントの不一致

## 発生状況

- **何をしようとしていたか**: M1 実環境デプロイ Step 1 で、ユーザーから提示された GCP プロジェクト ID（最初は `aoiproject-404705`）を `gcloud config set project` し、`gcloud billing projects describe` で Billing 状況を確認しようとしていた。
- **どのファイル/モジュールで発生したか**: gcloud CLI の `projects describe` / `billing projects describe` / `services list`。

## 失敗内容

```
WARNING: [k.matsunaga.biz@gmail.com] does not have permission to access projects instance
[aoiproject-404705] (or it may not exist):
The caller does not have permission.

ERROR: (gcloud.billing.projects.describe) ... does not have permission ...
```

- `gcloud projects list` が **0 件**を返す
- `gcloud billing accounts list` も **0 件**
- `core/project = aoiproject-404705` の文字列保存はできるが、API 経由の操作はすべて permission denied

→ **Cloud Console にログインしたアカウント（プロジェクト所有者）と、Claude Code 経由で `gcloud auth login` したアカウントが別だった**ことが後で判明。

## 根本原因

- ユーザーは Google アカウントを複数持つことが一般的で、Cloud Console（ブラウザ）と gcloud CLI（WSL 内の OAuth セッション）で **異なるアカウントが active** になり得る。
- Cloud Console は最後にログインしたアカウントを記憶。gcloud CLI は `gcloud auth login` 時に選んだアカウントを記憶。両者のアカウント整合は CLI 側からは検出しづらい。
- 当方は「プロジェクト ID を `config set` できた」事実だけを根拠に、本当にそのプロジェクトを操作できる権限があるかの**事前ゲート確認**を入れていなかった。

## 影響範囲

- Step 1 で約 1 サイクル分の手戻り（プロジェクト ID 変更・Billing 紐付け再取得）。
- 今後、複数 GCP プロジェクト・複数 Google アカウントを使い分ける運用ではいつでも再発し得る。
- Cloudflare / Twilio / SendGrid 等の外部サービス連携でも同種の「Console とユーザー認証の食い違い」は起こりうる。

## 対策種別

- [x] ルール化（禁止事項・必須事項の追加）
  - `core/project` セット時は、**直後に以下の 3 点を必ず確認**することをルール化
    1. `gcloud projects list --filter='projectId=<ID>'` で 1 件返るか
    2. `gcloud billing projects describe <ID>` で `billingEnabled=True` が返るか
    3. `gcloud config get-value core/account` と Cloud Console 右上のアカウントが一致しているか（ユーザー目視）
- [ ] スキル化
- [ ] テスト追加
- [ ] フック追加

## 教訓

- **「project が見える / config に保存できる」と「project を操作できる」は別問題**。前者は `core/project = <文字列>` の保存だけ、後者は IAM 権限。
- M1 実デプロイで使うプロジェクトは、最初に `gcloud projects list` の出力に **そのプロジェクトが現れることを確認**してから `config set` する。
- 複数アカウント運用時は `gcloud config configurations` で名前付き構成を分け、`gcloud config configurations activate` で切替える運用が安全（B 段階で検討）。
