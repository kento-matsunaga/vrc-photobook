// Upload Staging 画面用の queue 状態管理（pure function 集合）。
//
// 設計参照: docs/plan/m2-upload-staging-plan.md §6.3 / §11.1
//
// 方針:
//   - 副作用なし（fetch / DOM 操作は呼び出し側で実施）
//   - tile 状態遷移は型で安全に表現（discriminated union）
//   - concurrency 制御は selectNextRunnable で行う
//   - server 側との整合は reconcileWithServer で imageId map ベースで判定
//
// セキュリティ:
//   - tile.id は client-side UUID（photobook_id / image_id とは別）
//   - tile.file は React state にのみ保持、log には出さない
//   - reason / failure_reason の raw 値は外部に出さない（ユーザ表示は別 mapping）

/** 1 image の状態遷移。queued → verifying → uploading → completing → processing → available。
 *  失敗時は failed に倒れる（途中段階どこからでも）。 */
export type TileStatus =
  | { kind: "queued" }
  | { kind: "verifying" }
  | { kind: "uploading" }
  | { kind: "completing" }
  | { kind: "processing"; imageId: string }
  | { kind: "available"; imageId: string }
  | { kind: "failed"; reason: TileFailureReason; imageId?: string };

export type TileFailureReason =
  | "validation_failed"
  | "verification_failed"
  | "rate_limited"
  | "upload_failed"
  | "complete_failed"
  | "network"
  | "processing_failed"
  | "unknown";

export type QueueTile = {
  /** client-side 一意 ID（React key 用、photobook_id とは別物）。 */
  id: string;
  file: File;
  status: TileStatus;
};

export type QueueState = {
  tiles: QueueTile[];
};

const ACTIVE_KINDS: ReadonlyArray<TileStatus["kind"]> = [
  "verifying",
  "uploading",
  "completing",
];

const TERMINAL_KINDS: ReadonlyArray<TileStatus["kind"]> = [
  "available",
  "failed",
];

export function emptyQueue(): QueueState {
  return { tiles: [] };
}

/** 選択された File 群を queued tile として末尾に追加する。
 *  idGen はテスト容易性のため注入（uuid / counter どちらでも）。 */
export function addFiles(
  s: QueueState,
  files: ReadonlyArray<File>,
  idGen: () => string,
): QueueState {
  const newTiles: QueueTile[] = files.map((f) => ({
    id: idGen(),
    file: f,
    status: { kind: "queued" },
  }));
  return { tiles: [...s.tiles, ...newTiles] };
}

/** "verifying" / "uploading" / "completing" 中の tile 数。 */
export function activeUploadCount(s: QueueState): number {
  return s.tiles.filter((t) =>
    (ACTIVE_KINDS as ReadonlyArray<string>).includes(t.status.kind),
  ).length;
}

/** 次に upload を開始すべき queued tile を返す。concurrency 上限なら null。 */
export function selectNextRunnable(
  s: QueueState,
  concurrency: number,
): QueueTile | null {
  if (activeUploadCount(s) >= concurrency) return null;
  return s.tiles.find((t) => t.status.kind === "queued") ?? null;
}

/** 指定 tile の status を更新する。 */
export function markStatus(
  s: QueueState,
  tileId: string,
  status: TileStatus,
): QueueState {
  return {
    tiles: s.tiles.map((t) => (t.id === tileId ? { ...t, status } : t)),
  };
}

/** 指定 tile を queue から削除する（P1 retry / remove で使用、P0 では未使用）。 */
export function removeTile(s: QueueState, tileId: string): QueueState {
  return { tiles: s.tiles.filter((t) => t.id !== tileId) };
}

/** queue 内全 tile が terminal（available / failed）かつ tile 数 > 0。 */
export function isAllSettled(s: QueueState): boolean {
  if (s.tiles.length === 0) return false;
  return s.tiles.every((t) =>
    (TERMINAL_KINDS as ReadonlyArray<string>).includes(t.status.kind),
  );
}

/** processing 中 tile の imageId 一覧。reconcile 入力用。 */
export function processingTileImageIds(s: QueueState): string[] {
  return s.tiles
    .filter((t): t is QueueTile & { status: { kind: "processing"; imageId: string } } =>
      t.status.kind === "processing",
    )
    .map((t) => t.status.imageId);
}

/**
 * server 側の edit-view と queue を突き合わせる。
 *
 * - placedImageIds: edit-view.pages.*.photos.*.imageId の Set
 *   （server で available になり page に配置された image の imageId）
 * - serverProcessingCount: edit-view.processingCount
 *
 * 各 processing tile について:
 *   - imageId が placedImageIds に含まれる → "available" に遷移
 *   - serverProcessingCount === 0 かつ未配置 → "failed" (processing_failed) に遷移
 *     （server の処理が完了してかつ available にもならなかった = 失敗とみなす）
 *   - それ以外 → 据え置き（次の poll で再判定）
 */
export function reconcileWithServer(
  s: QueueState,
  placedImageIds: ReadonlySet<string>,
  serverProcessingCount: number,
): QueueState {
  const tiles = s.tiles.map((t) => {
    if (t.status.kind !== "processing") return t;
    const imgId = t.status.imageId;
    if (placedImageIds.has(imgId)) {
      return { ...t, status: { kind: "available", imageId: imgId } as TileStatus };
    }
    if (serverProcessingCount === 0) {
      return {
        ...t,
        status: {
          kind: "failed",
          reason: "processing_failed",
          imageId: imgId,
        } as TileStatus,
      };
    }
    return t;
  });
  return { tiles };
}

/** UI 表示用 summary。 */
export type QueueSummary = {
  total: number;
  queued: number;
  active: number;
  processing: number;
  available: number;
  failed: number;
};

export function summary(s: QueueState): QueueSummary {
  let queued = 0;
  let active = 0;
  let processing = 0;
  let available = 0;
  let failed = 0;
  for (const t of s.tiles) {
    switch (t.status.kind) {
      case "queued":
        queued++;
        break;
      case "verifying":
      case "uploading":
      case "completing":
        active++;
        break;
      case "processing":
        processing++;
        break;
      case "available":
        available++;
        break;
      case "failed":
        failed++;
        break;
    }
  }
  return { total: s.tiles.length, queued, active, processing, available, failed };
}

/**
 * 「編集へ進む」ボタンの enable 判定。
 *
 * 条件:
 *   - server 側 processingCount === 0
 *   - 1 枚以上の "available" image が存在する（queue 内 or server 配置済 photo）
 *   - queue に upload 中 tile が無い（queued / active / processing が残っていない）
 *
 * 空 queue の場合は server 状態のみで判定（戻ってきたユーザがそのまま /edit へ進めるよう）。
 */
export function canProceedToEdit(
  s: QueueState,
  serverProcessingCount: number,
  serverPlacedPhotoCount: number,
): boolean {
  if (serverProcessingCount > 0) return false;
  if (s.tiles.length === 0) {
    return serverPlacedPhotoCount > 0;
  }
  if (!isAllSettled(s)) return false;
  const queueHasAvailable = s.tiles.some((t) => t.status.kind === "available");
  return queueHasAvailable || serverPlacedPhotoCount > 0;
}

/** polling 待機秒（exponential backoff）。tick は 0 始まり。
 *
 *   tick: 0 1 2 3 4+
 *   sec : 5 5 10 20 60
 *
 * 上限 60 秒。 */
export function pollDelaySeconds(tick: number): number {
  if (tick <= 0) return 5;
  if (tick === 1) return 5;
  if (tick === 2) return 10;
  if (tick === 3) return 20;
  return 60;
}
