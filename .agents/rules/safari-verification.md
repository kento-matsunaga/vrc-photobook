# Safari / iPhone Safari 検証ルール

## 必須事項

以下のいずれかを変更した場合、**macOS Safari と iPhone Safari の動作確認を必須**とする。

| 変更対象 | 確認するもの |
|---------|------------|
| Cookie 発行ロジック（`Set-Cookie` 属性、Cookie 名、Path、Domain） | Cookie が発行され、HttpOnly / Secure / SameSite=Strict が維持される |
| redirect ロジック（302 / 303 / 307、redirect 先 URL） | redirect 後に Cookie が引き継がれる、URL から token が消える |
| OGP / Twitter card / 構造化データ | メタタグが SSR HTML に出力される、og:image の絶対 URL が解決される |
| レスポンスヘッダ制御（`Referrer-Policy` / `X-Robots-Tag` / `Cache-Control` / `CSP` 等） | ページ種別ごとに正しい値が出る、重複出力がない |
| モバイル UI（レイアウト、タップ可能要素、フォーム、画像表示） | iPhone Safari で破綻なくレンダリングされる、タップ操作が機能する |
| token → session 交換フロー（draft / manage / upload-verification） | 入場 URL から redirect → session Cookie 発行 → 認可ページで session found |

## 確認すべき最低限の項目

### macOS Safari（最新）

- [ ] 該当ページにアクセスして DevTools（Web Inspector）で Cookie 属性を目視確認
- [ ] redirect が発生する経路は redirect 後の URL / Cookie / 表示を確認
- [ ] レスポンスヘッダで該当ヘッダが期待値どおり付与されている
- [ ] ページ再読込後も状態が維持される

### iPhone Safari（最新、可能なら 1 世代前も）

- [ ] 同上の経路を実機で再現
- [ ] redirect 後の表示が成立する
- [ ] ページ再読込後も session 維持
- [ ] モバイル UI 変更時はタップ可能領域・横画面を含めて目視確認

### 継続観察（可能であれば、運用開始後）

- [ ] **24 時間後 / 7 日後の Cookie 残存**（ITP 長期影響）
- [ ] プライベートブラウジング動作

## 禁止事項

- 上記変更時に **Chrome / Edge のみで検証完了とする**ことを禁止
- iPhone Safari を「Chrome の互換ブラウザ」として扱うことを禁止（ITP / Cookie / Web Crypto API 等で挙動差がある）
- Cookie 値・raw token を console / 画面 / スクリーンショットに出すことを禁止（検証中も同様）

## Why

VRC PhotoBook はスマホファースト（業務知識 v4 §1.2 / §6.2）であり、X からの流入は iPhone Safari が主要動線。Frontend 設計は token → HttpOnly Cookie session 交換（ADR-0003）に依存しており、Safari ITP の挙動差で session が想定外に消えると **作成者が編集・管理できなくなる**重大事故になる。

M1 PoC（2026-04-25、コミット `42241f1` 時点）で macOS Safari / iPhone Safari の初回動作は問題なし確認済み。ただし 24 時間 / 7 日後の長期 ITP 影響は未確認のため、変更のたびに最低限の Safari 確認を入れることで「いつの間にか壊れる」を防ぐ。

## 関連ルール / 参照

- `.agents/rules/security-guard.md` — Cookie / token / Secret 取り扱い全般
- `.agents/rules/feedback-loop.md` — 失敗 → ルール化の運用
- `docs/adr/0003-frontend-token-session-flow.md` — token → session 交換方式（中核設計）
- `docs/adr/0001-tech-stack.md` §M1 検証結果 — Safari 実機検証結果
- `docs/plan/m1-spike-plan.md` §11 / §13.2 継続観察項目
- `harness/spike/frontend/README.md` §検証チェックリスト（Safari / iPhone Safari 章）

## 履歴

| 日付 | 変更 | 関連コミット |
|------|------|------------|
| 2026-04-25 | 初版作成。M1 PoC で macOS Safari / iPhone Safari 検証成功を受け、変更時 Safari 確認を必須ルール化 | `42241f1`（OpenNext 反映） + 本ルール作成コミット |
