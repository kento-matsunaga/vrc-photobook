# Backend CORS / mutation method 追加ルール

## 適用範囲

`backend/internal/http/cors.go` の `AllowedMethods` および新しい mutation HTTP method（PATCH / DELETE / PUT 等）を Backend に追加するすべての PR。

## 原則

> **Backend に新 mutation method を導入したら、CORS `AllowedMethods` に追加し、preflight test を必ず追加する。**

理由:
- Browser cross-origin fetch は preflight（OPTIONS）で `Access-Control-Allow-Methods` を確認し、要求 method が含まれていないと**本体送信を中止する**（fetch Promise が `TypeError` で reject）。
- Backend 側で route を生やすだけで `cors.go` を更新し忘れると、route 自体は起動するが browser からは絶対に到達できない。
- Frontend には generic な network error として伝わり、原因特定が困難。Cloud Run logs にも本体 request は来ないので、`OPTIONS 200 / <METHOD> 0 件` の状態を観察するまで気づけない。
- 2026-05-03 STOP α で `/edit` の全 mutation（PATCH × 4 / DELETE × 2）が PR12 deploy 以降ずっと壊れていたが、本番ユーザが publish 失敗を報告するまで誰も気づかなかった事故が発生。

## 必須パターン

### 1. cors.go の AllowedMethods にすべて列挙

```go
// backend/internal/http/cors.go
return cors.Handler(cors.Options{
    AllowedOrigins:   origins,
    AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
    AllowedHeaders:   []string{"Content-Type", "Authorization"},
    ExposedHeaders:   []string{},
    AllowCredentials: true,
    MaxAge:           600,
})
```

新しい method を追加する PR は **必ず**:
- `AllowedMethods` に追加
- `cors.go` の package コメントを更新（どの method がどの mutation で使われるか明記）

### 2. preflight test を必ず追加

`backend/internal/http/cors_test.go` に table 駆動で各 method の preflight 動作を assert:

```go
cases := []tc{
    {name: "正常_PATCH_preflight許可", method: stdhttp.MethodPatch},
    {name: "正常_DELETE_preflight許可", method: stdhttp.MethodDelete},
    {name: "正常_GET_preflight許可", method: stdhttp.MethodGet},        // regression
    {name: "正常_POST_preflight許可", method: stdhttp.MethodPost},      // regression
    // 新 method を追加したら新 case も追加
}
```

各 case で:
- OPTIONS リクエストに `Origin` + `Access-Control-Request-Method: <method>` を設定
- response の `Access-Control-Allow-Methods` に `<method>` が含まれることを assert
- `Access-Control-Allow-Origin`、`Access-Control-Allow-Credentials: true` も regression check

### 3. 新 mutation 機能の post-deploy smoke にも preflight を入れる

deploy 後の smoke で、本番 URL に対して preflight を実行し、本番の response にも `<method>` が含まれることを確認。

```bash
URL=https://api.vrc-photobook.com
ORIGIN=https://app.vrc-photobook.com
curl -sS -i -X OPTIONS \
  -H "Origin: $ORIGIN" \
  -H "Access-Control-Request-Method: PATCH" \
  "${URL}/api/photobooks/<dummy uuid>/settings" \
  | grep -iE '^HTTP|^access-control'
# 期待: HTTP/2 200 + access-control-allow-methods: PATCH（chi-cors は要求 method を echo する）
```

## 禁止事項

1. **新 mutation method を route に生やしたが `cors.go` を更新しない**
2. **CORS test を追加せずに mutation method を追加**
3. **preflight test を「unit test で確認したから」と言って production smoke を省略**
4. **`AllowedMethods` を `*` にする**（Origin / Credentials の組み合わせで仕様上 invalid、また CORS 緩和は最小化方針）

## チェックリスト

新 mutation method を追加する PR では:

- [ ] `backend/internal/http/cors.go` `AllowedMethods` に追加した
- [ ] `backend/internal/http/cors_test.go` に新 method の preflight test ケースを追加した
- [ ] PR description に「CORS PATCH/DELETE/...」を明記
- [ ] post-deploy smoke で実 production URL の preflight を curl で確認
- [ ] 新 method を呼ぶ Frontend mutation 関数のテストが `credentials: "include"` を渡している（`.agents/rules/client-vs-ssr-fetch.md` 参照）

## Why

2026-05-03 STOP α で `cors.go` の `AllowedMethods` が PR12 deploy 以降 `["GET", "POST", "OPTIONS"]` のままで、PR27 で /edit に PATCH/DELETE 系の mutation が追加されたにも関わらず CORS が更新されていなかった。Frontend が「保存失敗（再試行してください）」を出すだけで原因特定に時間がかかった。

修正自体は 1 行（PATCH / DELETE 追加）だが、事故クラスは「Backend で route 生やしたが CORS / Frontend 経路の整合を取らない」設計レベルの問題。本ルール + cors_test.go の table 駆動で再発防止。

## 関連

- `backend/internal/http/cors.go`
- `backend/internal/http/cors_test.go`
- `.agents/rules/client-vs-ssr-fetch.md`
- `harness/failure-log/2026-05-03_cors-patch-delete-omission.md`
- a8fe0db commit

## 履歴

| 日付 | 変更 |
|------|------|
| 2026-05-03 | 初版作成。STOP α PATCH/DELETE 不在事故をクラス level にルール化 |
