# CORS に PATCH / DELETE が無く、/edit mutation が browser で失敗する

## 発生日

2026-05-03 STOP α 調査で発見（実装は PR12 CORS 設定時から残っていた）。
本番影響期間: PR12 deploy 〜 a8fe0db deploy（2026-05-02）。

## 症状

ユーザが `/edit` 画面で「設定を保存」を押すと「保存失敗（再試行してください）」赤字。同様に caption 編集 / 並び替え / 表紙設定 / 写真削除も全部沈黙失敗。

Cloud Run logs では `/settings` への `OPTIONS 200 / PATCH 0 件`。preflight は通っているが本体送信が来ていない。

事故クラス: **Backend が新 mutation method を導入しても CORS allowed_methods に追加しない**。

## 根本原因

`backend/internal/http/cors.go` line 30 (a8fe0db 前):
```go
AllowedMethods: []string{"GET", "POST", "OPTIONS"},
```

PATCH / DELETE が含まれていない。`/edit` UI の全 mutation（PATCH `/settings`, PATCH `/photos/{id}/caption`, PATCH `/photos/reorder`, PATCH `/cover-image`, DELETE `/cover-image`, DELETE `/photos/{id}`）は preflight で `Access-Control-Allow-Methods` に PATCH / DELETE が含まれないため、ブラウザが本体送信を中止 → fetch が `TypeError` で reject → Frontend は generic な network error として「保存失敗」を表示。

## 修正

`backend/internal/http/cors.go` (a8fe0db):
```go
AllowedMethods: []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
```

経緯コメントを `cors.go` 上に明記:
> PATCH / DELETE は Edit UI の mutation（settings 保存 / caption / reorder / cover 設定 / cover クリア / photo 削除）で使用する。preflight Allow-Methods に含まれていないとブラウザが本体送信を中止し、Frontend 側に generic な network error として伝わる。

## 追加した test

`backend/internal/http/cors_test.go` (a8fe0db):
- `TestNewCORS_PreflightAllowsPATCHandDELETE` 4 case (PATCH / DELETE / GET regression / POST regression):
  - preflight OPTIONS で `Access-Control-Request-Method: <M>` を送信
  - response の `Access-Control-Allow-Methods` に `<M>` が含まれることを assert
  - `Access-Control-Allow-Origin` が `https://app.vrc-photobook.com`、`Access-Control-Allow-Credentials: true` も regression check
- `TestNewCORS_DefaultOriginWhenEmpty`: `ALLOWED_ORIGINS` 空 → 既定 origin が反映されること

## 今後の検知方法

- 新しい mutation method（例: 将来 `PUT`）を追加しても、CORS test で当該 method の preflight を assert しない限り regression。
- `.agents/rules/cors-mutation-methods.md` で「Backend に新しい mutation method を導入する PR は CORS preflight test 必須」をルール化。

## 残る follow-up

- 直近の本番動作確認（a8fe0db deploy 後の実機 smoke）
- 他の認証経路（manage / public / report）でも mutation method の追加時に同テストを横展開

## 関連

- `backend/internal/http/cors.go`
- `backend/internal/http/cors_test.go`
- `.agents/rules/cors-mutation-methods.md`
- a8fe0db commit
