// Upload Staging 画面の image tile（presentational）。
//
// 設計参照: docs/plan/m2-upload-staging-plan.md §6.5
//
// 役割:
//   - 1 image の status badge / filename / progress を表示
//   - retry / remove は P1 で追加（本 P0 では UI のみ表示、ハンドラ無し）
//
// セキュリティ:
//   - file.name は表示するが、imageId / storage_key / R2 URL は表示しない
//   - failed 時の reason は user-friendly な固定文言にマッピング（敵対者対策で詳細を出さない）

import type { QueueTile, TileFailureReason } from "@/components/Prepare/UploadQueue";

const STATUS_LABEL: Record<string, string> = {
  queued: "待機中",
  verifying: "認証中",
  uploading: "送信中",
  completing: "完了処理中",
  processing: "処理中",
  available: "完了",
  failed: "失敗",
};

const STATUS_COLOR: Record<string, string> = {
  queued: "bg-surface-soft text-ink-medium",
  verifying: "bg-brand-teal-soft text-brand-teal",
  uploading: "bg-brand-teal-soft text-brand-teal",
  completing: "bg-brand-teal-soft text-brand-teal",
  processing: "bg-brand-teal-soft text-brand-teal",
  available: "bg-status-success-soft text-status-success",
  failed: "bg-status-error-soft text-status-error",
};

const FAILED_REASON_LABEL: Record<TileFailureReason, string> = {
  validation_failed: "ファイル形式またはサイズが正しくありません",
  verification_failed: "Bot 検証に失敗しました",
  rate_limited: "短時間に操作が多すぎます。時間をおいて再試行してください",
  upload_failed: "アップロードに失敗しました",
  complete_failed: "アップロード完了処理に失敗しました",
  network: "通信エラーが発生しました",
  processing_failed: "画像処理に失敗しました",
  unknown: "不明なエラーが発生しました",
};

type Props = {
  tile: QueueTile;
};

function statusLabel(tile: QueueTile): string {
  return STATUS_LABEL[tile.status.kind] ?? tile.status.kind;
}

function statusColorClass(tile: QueueTile): string {
  return STATUS_COLOR[tile.status.kind] ?? "bg-surface-soft text-ink-medium";
}

export function ImageTile({ tile }: Props) {
  const isUploading = tile.status.kind === "uploading";
  const isFailed = tile.status.kind === "failed";
  const isAvailable = tile.status.kind === "available";

  return (
    <div
      data-testid={`prepare-tile-${tile.id}`}
      data-status={tile.status.kind}
      className={`flex flex-col gap-2 rounded-md border p-3 ${
        isFailed
          ? "border-status-error bg-status-error-soft"
          : isAvailable
            ? "border-status-success bg-surface"
            : "border-divider bg-surface"
      }`}
    >
      <div className="flex items-center justify-between gap-2">
        <span className="truncate text-xs text-ink-strong" title={tile.file.name}>
          {tile.file.name}
        </span>
        <span
          className={`shrink-0 rounded px-2 py-0.5 text-[10px] font-medium ${statusColorClass(tile)}`}
        >
          {statusLabel(tile)}
        </span>
      </div>

      {isUploading && (
        <div className="h-1 w-full overflow-hidden rounded bg-surface-soft">
          <div className="h-full w-1/2 animate-pulse bg-brand-teal" />
        </div>
      )}

      {tile.status.kind === "processing" && (
        <p className="text-[10px] text-ink-medium">最大 5 分ほどお待ちください</p>
      )}

      {isFailed && tile.status.kind === "failed" && (
        <p className="text-[10px] text-status-error">
          {FAILED_REASON_LABEL[tile.status.reason] ?? FAILED_REASON_LABEL.unknown}
        </p>
      )}

      <p className="text-[10px] text-ink-medium">
        {Math.round(tile.file.size / 1024).toLocaleString()} KB
      </p>
    </div>
  );
}
