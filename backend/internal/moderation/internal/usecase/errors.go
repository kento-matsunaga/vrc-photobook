// Package usecase: Moderation UseCase 群。
//
// 設計参照:
//   - docs/plan/m2-moderation-ops-plan.md §5 / §13 ユーザー判断 #4 / #5 / #9
//   - docs/design/aggregates/moderation/ドメイン設計.md §1 同一 TX 原則
//
// セキュリティ:
//   - raw token / Cookie / manage URL / storage_key 完全値 / R2 credentials を扱わない
//   - actor_label / detail に個人情報を入れない運用ガイドは runbook で示す
//   - ErrPhotobookNotFound は外部で 404 / exit 1 へ。区別を漏らさないために status の
//     詳細はエラーには含めない（cmd/ops 側でログ出力）。
package usecase

import "errors"

// 共通エラー。
var (
	// ErrPhotobookNotFound は対象 photobook が見つからない場合。
	ErrPhotobookNotFound = errors.New("moderation: photobook not found")

	// ErrInvalidStatusForHide は status=published 以外で hide / unhide を要求した場合。
	// 計画書 §5.3 / §13 ユーザー判断 #4: published のみを対象とする。
	ErrInvalidStatusForHide = errors.New("moderation: photobook status is not 'published' for hide/unhide")

	// ErrAlreadyHidden は既に hidden=true の photobook を hide しようとしたとき。
	// cmd/ops 側で「変更なし」として扱う（exit 0）。
	ErrAlreadyHidden = errors.New("moderation: photobook is already hidden by operator")

	// ErrAlreadyUnhidden は既に hidden=false の photobook を unhide しようとしたとき。
	ErrAlreadyUnhidden = errors.New("moderation: photobook is already not hidden")
)
