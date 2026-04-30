// 429 rate_limited UI 表示用の共通フォーマッタ。
//
// 設計参照:
//   - docs/plan/m2-usage-limit-plan.md §10
//   - PR36 commit 4
//
// 入力 retryAfterSeconds は Backend `Retry-After` header / body の
// `retry_after_seconds` から取得した秒数（>= 1 を期待）。本関数は表示用に
// 「N 分」「N 時間 M 分」「N 秒」等の短い日本語文字列に整形する。
//
// 60 秒未満は「1 分ほど」に丸め、長文化を防ぐ（iPhone Safari でレイアウト崩れ回避）。
export function formatRetryAfterDisplay(retryAfterSeconds: number): string {
  if (!Number.isFinite(retryAfterSeconds) || retryAfterSeconds <= 0) {
    return "1 分ほど";
  }
  if (retryAfterSeconds < 60) {
    return "1 分ほど";
  }
  const minutes = Math.ceil(retryAfterSeconds / 60);
  if (minutes <= 60) {
    return `${minutes} 分ほど`;
  }
  const hours = Math.floor(minutes / 60);
  const rest = minutes - hours * 60;
  if (rest === 0) {
    return `${hours} 時間ほど`;
  }
  return `${hours} 時間 ${rest} 分ほど`;
}
