# 2026-04-26 gcloud CLI「インストール完了」報告と実態の乖離

## 発生状況

- **何をしようとしていたか**: M1 実環境デプロイ Step 1 の前提として、WSL Ubuntu に Google Cloud CLI を `apt` 公式リポジトリ経由で導入し、`gcloud --version` / `gcloud auth list` から先に進めようとしていた。
- **どのファイル/モジュールで発生したか**: ユーザー対話シェルでの `sudo apt-get install google-cloud-cli`（複数行ワンライナー）と、Claude Code Bash 側での `gcloud --version` 確認。

## 失敗内容

- ユーザーから「インストール完了しました」とご報告いただいたが、Claude Code 側で再確認したところ:
  - `which gcloud` → 空
  - `gcloud --version` → `command not found`
  - `dpkg -l | grep google-cloud` → ヒットなし
  - `/etc/apt/sources.list.d/google-cloud-sdk.list` → 存在しない
  - `/usr/share/keyrings/cloud.google.gpg` → 存在しない
- つまり、**インストールコマンドは「打ったが途中で失敗していた」可能性が高い**にもかかわらず、対話シェル側のスクロールバックでは判別しづらく、完了として報告された。

## 根本原因

- インストール系コマンドの「成功判定」を、コマンド実行直後の終了コード / エラーメッセージではなく、**最後に表示される行の見た目**で判断してしまう運用上のリスク。
- ユーザー対話シェルと Claude Code Bash は **別 PTY / 別環境変数**で動いており、片方で成功しても他方が認識できないケースがある（PATH / 認証 / sudo timestamp など）。
- 当方が「インストール完了」報告をそのまま受け取り、`which gcloud` / `gcloud --version` で **実物の存在確認**を最初の 1 ステップに固定していなかった。

## 影響範囲

- Step 1 の進行が約 1 サイクル分遅れた（再インストール依頼のやり取り）。
- 同種の「ユーザーが手元で実行 → 完了報告 → Claude Code が再確認」パターンは今後も Cloud SQL / Secret Manager / Wrangler / Docker 操作で発生する。
- 検証コマンドを最初に決めておかないと、毎回ロスが出る。

## 対策種別

- [x] ルール化（禁止事項・必須事項の追加）
  - 「**インストール / セットアップ / 認証完了の報告は、Claude Code 側で `which` / `--version` / 設定ファイル存在 / `gcloud auth list` 等の客観確認を 1 度走らせてから次に進む**」を運用ルールに加える
- [ ] スキル化
  - 将来的に「install-verification」スキルとして CLI / SDK 導入後の確認コマンド集をテンプレ化（B 段階で検討）
- [ ] テスト追加
- [ ] フック追加

## 教訓

- **「完了」は宣言ではなく検証で確定する**。`which` / バージョン出力 / 期待される副作用（apt source / keyring / config 配置）の最低 3 点セットで判定する。
- インストールが multi-step（apt source 追加 → key → install）の場合、**ワンライナー全体の終了コードが非ゼロでも、画面の最後に `gcloud --version` が出ていない**ことを必ず明示確認する。
- ユーザー対話シェルと Claude Code Bash の **PATH / sudo timestamp / 認証チケットは共有されない**前提を、最初に説明資料化する。
