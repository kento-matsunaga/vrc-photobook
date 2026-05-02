# VRChat PNG 13.5MB が upload 前 validation で拒否される

## 発生日

PR22 / Issue A hotfix 期。
本 failure-log は 2026-05-03 STOP α harness 強化で正典化。

## 症状

VRChat デフォルトの PNG キャプチャ（2160x3840 / 約 13.5 MB）を `/prepare` で選択すると、`validateFile` の 10MB 上限で「サイズ過大」として拒否されていた。VRChat ユーザの最も基本的なユースケースが入口で失敗する。

事故クラス: **生成元プラットフォームの実値を測らず、最終 server 制約をそのまま client validation に当てる**。

## 根本原因

`frontend/lib/upload.ts` の `validateFile` は Backend の `declared_byte_size` 上限 10 MB に合わせて入力 file を拒否していたが、**圧縮を挟まず**入力をそのまま判定していた。VRChat PNG は spec 上 13〜18 MB 程度で 10 MB を超えるのが常態。

## 修正

Frontend (Issue A hotfix) に **client-side 圧縮**を追加:

`frontend/lib/imageCompression.ts`:
- 入力 max 50 MB（input_too_large）。それ以上は decode 前に拒否。
- canvas で decode → 長辺 max 2400px に縮小（VRChat 2160x3840 → 1350x2400）。
- JPEG q=0.85 でエンコード。これで超なら q=0.7 fallback、それでも超なら still_too_large。
- PNG / WebP / JPEG いずれでも 10 MB 以内に収まるよう調整。

PrepareClient / EditClient の `handleFileSelect` は `compressImageForUpload(f)` 経由で File を加工し、その結果に対して `validateFile` を当てる。圧縮済 File は当然 10 MB 以内に入る。

## 追加した test

`frontend/lib/__tests__/imageCompression.test.ts`:
- 「正常_VRChat PNG 13.5MB → 縮小+JPEG q=0.85で2MB相当」
- 「正常_long edge >maxLongEdgeで縮小（VRChat 2160x3840 → 1350x2400）」
- 「正常_横長画像も同じロジックで縮小（3840x2160 → 2400x1350）」
- 「正常_q=0.85 が大きすぎたら q=0.7 fallback」
- 「異常_input_too_large（既定 50MB 超）はdecode前に拒否」
- 「異常_q=0.85もq=0.7も超なら still_too_large」
- 「正常_PNG → JPEG renameToJpg」
- 他 22 ケース

## 今後の検知方法

- `compressImageForUpload` の no-op path / fallback / error path が 22 ケースで網羅されており、圧縮しないで validate に流す regression は即落ちる。
- 新しい入力形式（HEIC / HEIF 等）追加時も「圧縮前にフィルタ → 圧縮 → validate」順序を保つことで検知。

## 残る follow-up

- HEIC 対応は入っていない（仕様上「HEIC / HEIF は現在未対応」を表示）
- JPEG-from-DSLR 大型ファイル（30-40 MB）の動作実測値の定期観測

## 関連

- `harness/failure-log/2026-05-03_prepare-reload-queue-loss.md`
- `frontend/lib/imageCompression.ts`
- `frontend/lib/upload.ts` `validateFile`
