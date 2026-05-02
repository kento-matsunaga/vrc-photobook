# Zenn 記事ドラフト: AIに同じミスを2度させない仕組みを1週間で回した話

> 用途: Zenn 公開用ドラフト（ローンチ後に公開予定）
> 連動: X 長文投稿、サムネ画像（「たった1週間で」「AIに同じミスを2度させない」「再発ほぼゼロ」）
> 公開可否: PR38（公開リポジトリ化判断）後に最終確認。本番 URL / 生 ID / token は本ドラフト時点で含めていない
> 更新時の注意: 失敗ログ原文の引用部分は redact 済（slug / row id prefix / scope_hash 等は伏せている）

---

# AIに同じミスを2度させない仕組みを1週間で回した話 ─ 2026話題のハーネスエンジニアリング実践

## TL;DR

- 個人開発の Claude Code 運用で、**直近1週間で失敗ログ18本を記録**
- すべて `failure-log/` → `.agents/rules/` に正典化したら、**同種ミスの再発がほぼゼロ**
- これが 2026 年 Claude Code 界隈で話題の **「ハーネスエンジニアリング」** の運用実例

---

## ハーネスエンジニアリングとは

Mitchell Hashimoto（HashiCorp 共同創業者）が 2026 年 2 月のブログ "My AI Adoption Journey" で提唱した概念。AI 導入の 6 段階のうち **第 5 段階 "Engineer the Harness"** として登場した。

要点はシンプルで:

> Claude Code を「賢いチャットボット」から「自分専用の開発環境」に変える

`CLAUDE.md` / Skills / Agents / Hooks / Settings ── これらを組み合わせて AI の「ハーネス（馬具）」を整備し、意図した方向に動かすのがハーネスエンジニアリング。

SWE-bench でハーネス設計の違いだけでスコアが最大 22 ポイント変動した一方、モデル入れ替えではわずか 1 ポイントしか変わらなかったというデータもある。**モデル < ハーネス**、という構図。

ただ「概念は分かった、で何をすればいいの？」という人が多い。本記事では、個人開発で実際に運用しているディレクトリ構成と運用ループを **全部出す**。

---

## このリポジトリの構成

VRChat 向けフォトブックサービスを、Claude Code でハーネスを組みながら開発している。コア構成は以下:

```
project-root/
├── CLAUDE.md                    # プロジェクト最上位の指針
├── .agents/
│   └── rules/                   # 禁止/必須を明文化（× 9本）
│       ├── coding-rules.md
│       ├── domain-standard.md
│       ├── testing.md
│       ├── security-guard.md
│       ├── safari-verification.md
│       ├── feedback-loop.md
│       ├── wsl-shell-rules.md
│       ├── pr-closeout.md
│       └── turnstile-defensive-guard.md
├── harness/
│   ├── failure-log/             # 失敗の正典（× 18本）
│   ├── work-logs/               # 各タスクの実施記録
│   └── QUALITY_SCORE.md         # 品質スコア
└── docs/
    ├── adr/                     # 設計判断
    ├── design/                  # 集約・横断設計
    └── plan/                    # ロードマップ
```

回しているループ:

```
Spec → Implement → Verify → Feedback
                              ↓
                         失敗を検知
                              ↓
                  failure-log に記録
                              ↓
                  対策種別を必ず選ぶ
                              ↓
       ルール化 / スキル化 / テスト追加 / フック追加
                              ↓
                  次のサイクルから自動防止
```

`CLAUDE.md` は Claude Code が **毎セッション読み込む** ので、そこから `.agents/rules/` を参照させれば、ルールが自動的にエージェントの判断に効く。

---

## failure-log のテンプレ

すべての失敗は同じテンプレで記録する。これが効く:

```markdown
# YYYY-MM-DD_失敗の短い説明.md

## 発生状況
- 何をしようとしていたか
- どのファイル/モジュールで発生したか

## 失敗内容
- 具体的なエラーまたは問題の症状
- テスト出力やエラーログ

## 根本原因
- なぜ失敗したか（表面的な原因ではなく根本原因）

## 影響範囲
- この失敗が他に影響する箇所

## 対策種別
- [ ] ルール化（禁止事項・必須事項の追加）
- [ ] スキル化（手順の自動化）
- [ ] テスト追加（検出の自動化）
- [ ] フック追加（イベント駆動の防止策）
```

ポイントは 2 つ:

1. **「根本原因」を表面的原因と分けて書く**
   - 「コミットが失敗した」ではなく「pre-commit hook の相対パス解決が cwd drift で破綻した」レベルまで掘る
2. **対策種別から必ず 1 つ以上選ぶ**
   - 反省で終わらせない。チェックボックスを埋める形にすると「結局どうしたか」が消えない

---

## 実例 3 つ

### ① Turnstile widget が無限ループに見えるのに、裏で 1 回 submit が成立した

**症状**: iPhone Safari で公開フォームを表示中、Turnstile（Cloudflare の reCAPTCHA 代替）が「ロボットではありません → ロード → 再度チェックボックス → ロード」を繰り返し、ユーザー視点では **無限ループ**。「submit はまだ押していない」と認識して中断。

**実態**: DB を確認すると **submit が 1 件成立済み**。アクセスログでも POST が 1 回だけ正常に通っている。

**根本原因**:

```tsx
// TurnstileWidget.tsx の useEffect
useEffect(() => {
  // turnstile.render({ callback: onVerify, "error-callback": onError, ... })
}, [scriptLoaded, sitekey, action, onVerify, onError, onExpired]);
//                                  ^^^^^^^^^^^^^^^^^^^^^^^^^^^
//                                  inline arrow を依存に入れていた
```

親コンポーネント側:

```tsx
<TurnstileWidget
  onError={() => setTurnstileToken("")}     // 毎 render で新参照
  onExpired={() => setTurnstileToken("")}   // 毎 render で新参照
/>
```

連鎖:

1. ユーザーが入力する → 親が re-render
2. inline arrow の参照が変わる → useEffect が依存変化と判定
3. `turnstile.remove()` → `turnstile.render()` の cycle が走る
4. ユーザー視点: 「無限ループ」
5. 入力が止まった瞬間に verification 完了 → token がセット → submit 成立

**対策**:

- ルール化: `.agents/rules/turnstile-defensive-guard.md` に **「L0: TurnstileWidget の安定 mount」** セクションを追加
- 実装: callback prop を内部で `useRef` 保持し、useEffect 依存配列から外す
- 親側でも `useCallback` で安定化（防御的に二重）
- L1〜L4 の多層ガード（trim 後 empty 拒否）が機能していたことも併せて確認

これで Turnstile を使う **すべての form** が同じガード下に入った。

> **学び**: 「機能は正しく動いていたが、ユーザー体験は壊れていた」というクラスの不具合は、自動テストでは捕捉しづらい。実機 smoke を必須化する `safari-verification.md` ルールも併せて運用する。

---

### ② 「sudo apt 完了しました」報告と実態の乖離

**症状**: ユーザーから「gcloud CLI のインストール完了」報告 → Claude Code 側で確認すると `which gcloud` が空、`dpkg -l | grep google-cloud` がヒットなし、`/etc/apt/sources.list.d/` にも該当ファイルなし。

**根本原因**:

- `sudo` は **tty / セッション単位** で認証チケットを管理する
- ユーザーの対話シェルと Claude Code Bash は **別 PTY / 別環境**で動く
- 対話シェルでインストールが途中で失敗していたが、スクロールバックでは判別しづらく完了として報告された

**対策**:

ルール化: `.agents/rules/wsl-shell-rules.md` に **「インストール完了の宣言は信用しない、Claude Code 側で `which` / `--version` / 設定ファイル存在 / 認証状態を客観確認してから次に進む」** を追加。

```bash
# ✅ install 後の客観確認テンプレ
which gcloud && gcloud --version
dpkg -l | grep google-cloud-cli
gcloud auth list
gcloud config get-value project
```

同種の「ユーザー対話シェルでの作業 → Claude Code が再確認」パターンは Cloud SQL / Secret Manager / wrangler / docker でも頻発する。**install / setup / 認証系のあとは必ず客観確認をワンライナーに含める** をルール化した。

> **学び**: 「ユーザーが手元でやった」ことを宣言ベースで信用しない。AI と人間で環境が分離していることを前提に、客観確認を運用に組み込む。

---

### ③ 5 分固定窓 RateLimit の smoke が window rollover で再発火

**症状**: RateLimit（固定窓カウンタ）の 429 動作確認のため、`usage_counters` テーブルを `count = limit` に手動 UPDATE → ユーザーが iPhone Safari で submit → **2 連続とも正常に通った**（429 が出ない）。

**根本原因**: 5 分固定窓の rollover。

```
17:05:00 ─── window開始（手動UPDATE） count=3, limit=3
17:10:00 ─── window境界 ★
17:12:18 ─── ユーザーがsubmit（新窓 17:10:00- に該当、count=0からスタート）
```

PostgreSQL の固定窓 counter は `(scope_type, scope_hash, action, window_start)` の複合キーで row を識別するため、`window_start` が変われば **別 row**。手動 UPDATE で埋めた旧窓は新窓に影響しない。

人間の操作時間（ops コマンド → ユーザー画面操作 → submit）と 5 分窓の境界がぶつかる確率は十分高く、smoke 設計上の見落としだった。

**対策**: 検証手順の修正 ── **狭粒度（5 分窓）と広粒度（1 時間窓）の両方を threshold 化** する。

```
narrow: 17:20:00- count=3,  limit=3   ← 5分窓で429
broad:  17:00:00- count=20, limit=20  ← 1時間窓で429（保険）
```

仮に狭粒度が rollover しても、広粒度が確実に 429 を返す。runbook 化: `docs/runbook/usage-limit.md` §11 に「smoke 設計時は両粒度を埋めること」を追記。

> **学び**: 固定窓 + 人間操作の組み合わせは踏みやすい罠。失敗を踏むまで気付けないクラスの問題は、ルール化と runbook 反映が刺さる。

---

## なぜ効くのか

3 つの要素が組み合わさって初めて回る。

### 1. 失敗の構造化（4 ブロックテンプレ）

「発生状況 / 根本原因 / 影響範囲 / 対策種別」を必ず埋めるフォーマットにすると、**書き手が自分で根本原因まで掘る** ようになる。「動かなかった、直した」レベルでは failure-log に書けない。

### 2. 対策種別の必須選択（4 択チェックボックス）

「ルール化 / スキル化 / テスト追加 / フック追加」のどれかを必ず選ぶ。曖昧な反省を許さない。

特に **ルール化** は `.agents/rules/{name}.md` として残るので、次回以降の Claude Code が `CLAUDE.md` 経由で **自動的に読む** ことになり、再発を機械的に防げる。

### 3. ルールに「Why」必須

ルールには必ず「Why（なぜこのルールが必要か）」セクションを書く。

```markdown
## Why

エージェントが以下のセキュリティ問題を引き起こしたため:
- リクエストパラメータから executorID を取得 → 監査ログのなりすましが可能に
- テナントスコープなしのクエリ → 他テナントのデータが漏洩
```

「なぜ」が分かれば、新しい状況でも応用できる。「なぜ」が消えるとルールは形骸化する。

---

## 読者が今すぐ使えるテンプレ

そのままコピペして使えるテンプレを置いておく。

### failure-log テンプレ（`harness/failure-log/YYYY-MM-DD_短い説明.md`）

```markdown
# YYYY-MM-DD_失敗の短い説明

## 発生状況
- 何をしようとしていたか:
- どのファイル/モジュールで発生したか:

## 失敗内容
- 症状 / エラー:
- 期待と実際:

## 根本原因
- 表面ではなく根本まで:

## 影響範囲
- 本番への影響:
- 設計への影響:
- 横展開すべき箇所:

## 対策種別
- [ ] ルール化（`.agents/rules/{name}.md`）
- [ ] スキル化（`.agents/skills/{name}/SKILL.md`）
- [ ] テスト追加
- [ ] フック追加

## 取った対策
（具体的にどう変更したか）

## 関連
- 関連ルール / 関連 work-log / 参考リンク
```

### ルールテンプレ（`.agents/rules/{name}.md`）

```markdown
# {ルールタイトル}

## 適用範囲
（どのコード / どの作業に適用するか）

## 原則
（1〜2文で要点）

## 必須パターン
（コード例で「✅ 正しい / ❌ 禁止」を併記）

## 禁止事項
（箇条書きで列挙）

## Why
（なぜこのルールが必要か、過去の失敗事例リンク）

## 関連
- 他ルール / failure-log / ADR
```

### `CLAUDE.md` からの参照（最上位）

```markdown
## 守るべきルール（必読）

- `.agents/rules/coding-rules.md`
- `.agents/rules/domain-standard.md`
- `.agents/rules/testing.md`
- `.agents/rules/security-guard.md`
- ...
```

これで Claude Code が毎セッション読み込む `CLAUDE.md` 経由で、すべてのルールが自動的に効く。

---

## まとめ

「AI に同じミスを 2 度させない仕組み」は、**ルールを書くだけ** では回らない。

- 失敗を構造化して残す（failure-log）
- 対策種別を必ず選ぶ（4 択）
- ルールに Why を残す（応用が効く）

この 3 点セットがあって初めて、ハーネスが「育つ」状態になる。

直近 1 週間で 18 本の失敗を踏んだが、すべてルール / テスト / runbook に変換できた。次の 1 週間も新しい失敗を踏むだろうが、**同じ場所では転ばない** という確信がある。

これが 2026 年話題の「ハーネスエンジニアリング」の運用実体です。

---

VRChat 向けフォトブックサービス開発中（スマホファースト / ログイン不要 / 管理 URL 方式）。ローンチ後にリポジトリも公開予定。

## 参考

- Mitchell Hashimoto, "My AI Adoption Journey"（2026/2）
- 逆瀬川（@gyakuse）「Claude Code / Codex ユーザーのための誰でもわかる Harness Engineering ベストプラクティス」（2026/3）
