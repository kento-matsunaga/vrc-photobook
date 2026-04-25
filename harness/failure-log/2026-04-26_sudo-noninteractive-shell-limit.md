# 2026-04-26 Claude Code Bash で sudo パスワード入力ができない

## 発生状況

- **何をしようとしていたか**: WSL Ubuntu に Google Cloud CLI をインストールするため、`sudo apt-get update` 等を Claude Code の Bash ツールで連続実行しようとしていた。
- **どのファイル/モジュールで発生したか**: Claude Code Bash ツール経由のあらゆる sudo 系コマンド。

## 失敗内容

```
sudo: a terminal is required to read the password;
either use the -S option to read from standard input
or configure an askpass helper
sudo: a password is required
```

- ユーザーが対話シェルで `! sudo -v` を打って sudo 認証チケットを取得しても、**Claude Code Bash の別 tty には伝播せず**、後続の `sudo apt-get update` 等が引き続き失敗。
- 結果、インストール手順をユーザー対話シェルで実行してもらう運用に切替え（ただしそれが別の失敗 `2026-04-26_gcloud-install-verification-mismatch.md` を呼んだ）。

## 根本原因

- `sudo` の認証チケット（timestamp）は **tty 単位 / セッション単位**で記録される（`Defaults timestamp_type=tty` がデフォルト）。
- Claude Code Bash ツールは PTY を持たない非対話シェルとして起動し、ユーザー対話シェルの sudo timestamp を引き継げない。
- そのため Claude Code 側からの `sudo` は、たとえユーザーが事前に対話シェルで sudo 認証していても、独立して password を要求する。

## 影響範囲

- WSL 環境で **root 権限が必要な作業（apt install / systemctl / /etc 配下の編集）はすべて Claude Code Bash 経由ではできない**。
- 影響対象: gcloud CLI 導入、Docker daemon 系操作（通常は不要だが）、`/etc/apt/sources.list.d/` 編集、システム証明書追加など。
- 一方、本プロジェクトの主作業（Go / Docker user 領域 / gcloud / wrangler / git）は基本的に **sudo 不要**で完結するため、初期セットアップ以降は再発リスクは低い。

## 対策種別

- [x] ルール化（禁止事項・必須事項の追加）
  - **sudo が必要な処理は Claude Code Bash で実行しない**（毎回詰まる）
  - **ユーザー対話シェルで `! <ワンライナー>` として一連の sudo コマンドをまとめて実行**してもらう運用を明文化
  - ユーザーが手で実行する場合は、**完了後に Claude Code 側で `which` / バージョン / 設定ファイル存在の客観確認**を必須化（`gcloud-install-verification-mismatch.md` と整合）
- [ ] スキル化
- [ ] テスト追加
- [ ] フック追加

## 教訓

- **`sudo -v` は別 tty に伝播しない**。Claude Code 側の sudo を NOPASSWD で通すには `/etc/sudoers.d/` を編集する必要があるが、これも sudo を必要とするので堂々巡り。
- セッション開始時に「sudo が必要な処理は最初にまとめてユーザー対話シェルで実施 → 以後は user 領域だけで進む」設計を取る。
- 本プロジェクトでは初期 CLI 導入以降、sudo は基本不要（gcloud / wrangler / docker user / go / git すべて user 権限で動く）。
