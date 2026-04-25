# Auth 認可機構 全体概要

> 上流: [業務知識定義書 v4](../../spec/vrc_photobook_business_knowledge_v4.md) / [ADR-0003 フロントエンド認可フロー](../../adr/0003-frontend-token-session-flow.md) / [ADR-0005 画像アップロード方式](../../adr/0005-image-upload-flow.md)
>
> このディレクトリは VRC PhotoBook の **認可機構**（Authorization）に関する設計を集めたもの。集約（DDD の意味での aggregate root）ではなく、**アプリケーション層・横断的なデータモデル + 認可フロー** として扱う。

---

## なぜ集約ではないのか

VRC PhotoBook はログイン不要のため、認可は「token を持っている人 = 操作権限を持つ人」という単純な構造になる。これに対して以下の事情から、**ドメイン集約として扱わない**。

- 認可状態は業務ドメインの語彙（フォトブック・ページ・写真）に登場しない
- ビジネス不変条件が薄い（「期限切れなら無効」「revoked なら無効」程度）
- 認可は技術的横断機構であり、ドメイン挙動より**ADR の決定に直結**する

業務知識 v4 §2.4 でも session は技術用語として整理されている。本設計は「集約として書こうとすると `Session.revoke()` 等の貧血ドメインモデルになり、ApplicationService がほぼ全てのロジックを抱える」という DDD アンチパターンを避けるための判断である。

ただし、データモデルとして明確に必要なため、本ディレクトリでテーブル定義・操作定義・ライフサイクルを記述する。

---

## ディレクトリ構成

```
docs/design/auth/
├── README.md                            # 本ファイル
├── session/                             # draft/manage の操作 session
│   ├── ドメイン設計.md
│   └── データモデル設計.md
└── upload-verification/                 # Turnstile 検証セッション
    ├── ドメイン設計.md
    └── データモデル設計.md
```

`session/` と `upload-verification/` を分離した理由:

| 観点 | session | upload-verification |
|------|---------|---------------------|
| 主な役割 | draft/manage URL アクセス時の認可 session | Turnstile 検証結果の短期保持（画像アップロード許可） |
| 入場手段 | `/draft/{token}` または `/manage/token/{token}` | upload-intent 直前の Turnstile チャレンジ |
| 期限 | draft: `draft_expires_at` まで（最大 7 日） / manage: 24h〜7d | 30 分固定 |
| 利用制限 | 通常 API 認可（毎リクエスト） | 1 検証あたり最大 20 回 upload-intent |
| 上流 ADR | ADR-0003 | ADR-0005 |

ライフサイクル・有効期限・責務がすべて異なるため、テーブルもファイルも分離する。

---

## 認可機構の全体像

```
┌─────────────────────────────────────────────────────────────┐
│ raw token を URL で受け取る (/draft/{token}, /manage/token/...) │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
              [ Token 検証 ]
                         │
       ┌─────────────────┴─────────────────┐
       │                                   │
       ▼                                   ▼
[ 256bit 乱数 session_token を生成 ]    [ Turnstile 検証 ]
       │                                   │
       ▼                                   ▼
[ sessions に hash 保存 ]            [ upload_verification_sessions に hash 保存 ]
       │                                   │
       ▼                                   ▼
[ HttpOnly Cookie で session_token 配布 ]  [ HttpOnly Cookie / Header で session_token 配布 ]
       │                                   │
       ▼                                   ▼
[ /edit/{photobook_id} に redirect ]    [ upload-intent API 呼び出し ]
       │                                   │
       ▼                                   ▼
[ 以後の API は Cookie session で認可 ]  [ intent 1 件消費、最大 20 回まで ]
```

---

## Photobook 集約との接続点

本機構は **Photobook 集約から呼び出される側**。Photobook 集約 §14.1 で約束した 2 操作:

| 呼び出し元 | 操作 | 用途 |
|----------|------|------|
| Photobook `publishFromDraft` | `revokeAllDrafts(photobookId)` | publish 時に対象 Photobook の draft session を全 revoke（同一 TX） |
| Moderation `reissueManageUrl` | `revokeAllManageByTokenVersion(photobookId, oldVersion)` | manage_url_token 再発行時に旧 version 由来の manage session を全 revoke（同一 TX） |

加えて API Middleware からの **session 検証**:

- `validate(rawToken, photobookId, sessionType)`: 各リクエストで Cookie session の有効性を判定
- `touch(sessionId)`: `last_used_at = now()` を更新

詳細は `session/ドメイン設計.md` §6 を参照。

---

## Image 集約との接続点

`upload-verification/` は Image 集約から呼び出される。

| 呼び出し元 | 操作 | 用途 |
|----------|------|------|
| Image `upload-intent` UseCase | `validateAndConsume(rawToken, photobookId)` | Turnstile セッションを検証し、intent 残数を 1 消費 |
| API（Turnstile 直後） | `issue(photobookId, turnstileResult)` | Turnstile 検証成功時に新規 upload_verification_session を発行 |

詳細は `upload-verification/ドメイン設計.md` §6 を参照。

---

## 共通方針（ADR-0003 / ADR-0005 ベース）

両 session で共通する原則:

### Cookie / Token 取り扱い

- raw token は **DB に保存しない**
- Cookie / クライアント保持値には **256bit 以上の暗号論的乱数を base64url 化** したものを使う
- DB には **SHA-256 ハッシュのみ** を保存（`bytea` 32 バイト固定）
- Cookie 属性: `HttpOnly: true` / `Secure: true` / `SameSite: Strict` / `Path: /`
- Cookie の `Domain` 属性は M1 スパイク結果で確定（未解決事項 U2）

### CSRF

- `SameSite=Strict` + `Origin` ヘッダ検証必須
- 状態変更 API は POST/PUT/PATCH/DELETE のみ
- CORS は自サイトのみ
- 破壊的操作（削除、公開範囲変更、管理 URL 再発行、センシティブ変更）には**ワンタイム確認トークン**を要求（実装は Application/API 設計に委ねる、未解決事項 U4）

### ログ・エラートラッキング

- raw token / hash 値どちらも構造化ログに出さない
- Cookie の値そのものをログ出力しない
- Sentry 等のエラートラッキングへ URL 全文を送らない（ADR-0003）

### Reconcile

- 期限切れ session の GC は自動 reconciler で実施
- expired upload_verification_session も同様

---

## 関連 ADR / 業務知識

| 参照先 | 主な内容 |
|--------|---------|
| ADR-0003 | token→session 交換、Cookie 属性、URL 設計、CSRF 対策、draft 延長ルール |
| ADR-0005 | upload-intent / complete 2段 API、Turnstile セッション化、failure_reason |
| v4 §2.3 | URL とトークン / セッションに関する用語 |
| v4 §2.4 | セッションと入場フロー |
| v4 §3.1 | フォトブック作成機能（draft アップロードフロー） |
| v4 §3.2 | フォトブック公開機能（publish 時の draft session 全 revoke） |
| v4 §3.4 | フォトブック管理機能（manage session、明示破棄、再発行時の一括 revoke） |
| v4 §3.7 | 荒らし対策（Turnstile セッション化） |
| v4 §6.15 | token→session 交換方式（横断ルール） |
| 付録C P0-14〜P0-18 | 本機構で反映する 5 項目の追跡 ID |

---

## 未解決事項（横断、各設計で詳述）

- **U1**: revoke 方式は `revoked_at` UPDATE で確定。INSERT-only 履歴方式は Phase 2 検討
- **U2**: Cookie の `Domain` 属性は M1 スパイクで確定（未指定が基本候補）
- **U3**: 別端末同時編集時の楽観ロック衝突 UX は M7 で確定（ドメインは `OptimisticLockConflict` を返す）
- **U4**: 破壊的操作のワンタイム確認トークンは Application/API 設計へ引き継ぐ
