// Upload Staging 画面用の queue 状態管理（pure function 集合）。
//
// 設計参照: docs/plan/m2-upload-staging-plan.md §6.3 / §11.1
// β-3 拡張: plan v2 §3.4 P0-c（reload 復元 / server ground truth マージ）
//
// 方針:
//   - 副作用なし（fetch / DOM 操作は呼び出し側で実施）
//   - tile 状態遷移は型で安全に表現（discriminated union）
//   - concurrency 制御は selectNextRunnable で行う
//   - server 側との整合は reconcileWithServer / mergeServerImages で imageId base に判定
//
// セキュリティ:
//   - tile.id は client-side のみで完結する識別子（imageId / photobook_id を含めない）
//   - tile.file は React state にのみ保持、log には出さない
//   - reason / failure_reason の raw 値は外部に出さない（ユーザ表示は別 mapping）
//   - server 復元 tile の displayLabel は filename 補助のみ（imageId を表示文字列に出さない）

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
  /** client-side 一意 ID（React key 用、photobook_id / image_id を含めない）。 */
  id: string;
  /** local upload File。server 復元 tile では undefined。 */
  file?: File;
  /** tile UI に表示する label（filename 補助、raw image_id ではない）。 */
  displayLabel: string;
  /** byte size（local tile は file.size、server 復元 tile は image.original_byte_size）。 */
  byteSize: number;
  /** tile が server image から復元されたかどうか（UI 装飾の差別化に使用）。 */
  origin: "local" | "server";
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
    displayLabel: f.name,
    byteSize: f.size,
    origin: "local",
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

/** server image の status を tile status に投影する（imageId 抜きで返す、呼び出し側で詰める）。 */
function statusFromServerImage(
  imageStatus: "uploading" | "processing" | "available" | "failed",
  imageId: string,
): TileStatus {
  switch (imageStatus) {
    case "uploading":
      // server に image record はあるが未 complete。並行 tab の最中など稀ケース。
      // UI は「送信中」表示で扱う。
      return { kind: "uploading" };
    case "processing":
      return { kind: "processing", imageId };
    case "available":
      return { kind: "available", imageId };
    case "failed":
      // raw failure_reason は外に出さず "processing_failed" に倒す。
      return { kind: "failed", reason: "processing_failed", imageId };
  }
}

/** server image を queue に統合するための情報。 */
export type ServerImageForMerge = {
  imageId: string;
  status: "uploading" | "processing" | "available" | "failed";
  originalByteSize: number;
  createdAt: string;
};

/**
 * server edit-view の images を queue とマージし、reload 復元を行う（β-3 P0-c）。
 *
 *   - 既に attach 済（placedImageIds 含む）の image: queue から除外（/edit ページに移動済）
 *   - 既に queue に対応 tile がある image（status.imageId 一致）: status を server 値で更新
 *   - 未対応の server image: 新規 server-restored tile として追加
 *   - upload chain 中の local-only tile（imageId 未割り当て）: そのまま保持
 *
 * displayLabel は labelLookup（localStorage 等）から取得、無ければ generic 文言。
 * raw imageId を displayLabel に流入させない。
 *
 * tiles の順序は createdAt ASC（server 提供順）を尊重しつつ、既存 tile は先に出す。
 */
export function mergeServerImages(
  s: QueueState,
  serverImages: ReadonlyArray<ServerImageForMerge>,
  placedImageIds: ReadonlySet<string>,
  labelLookup: (imageId: string) => string | null,
  idGen: () => string,
): QueueState {
  const tilesByImageId = new Map<string, QueueTile>();
  const localTiles: QueueTile[] = [];
  for (const t of s.tiles) {
    const imgId = imageIdOf(t);
    if (imgId !== null) {
      tilesByImageId.set(imgId, t);
    } else {
      localTiles.push(t);
    }
  }

  // server image を imageId base で統合
  const serverImageById = new Map<string, ServerImageForMerge>();
  for (const img of serverImages) serverImageById.set(img.imageId, img);

  // 1. 既存 tile（imageId 持ち）を server 状態で更新 / 削除判定
  const updatedTiles: QueueTile[] = [];
  for (const [imgId, t] of tilesByImageId) {
    if (placedImageIds.has(imgId)) {
      // /edit に移動済、queue からは除外
      continue;
    }
    const serverImg = serverImageById.get(imgId);
    if (serverImg === undefined) {
      // server から見えない（極稀、削除等）→ そのまま保持
      updatedTiles.push(t);
      continue;
    }
    const newStatus = statusFromServerImage(serverImg.status, imgId);
    updatedTiles.push({
      ...t,
      status: newStatus,
    });
  }

  // 2. 新規 server image（queue に未存在 + 未配置）を追加
  const sorted = [...serverImages].sort((a, b) => a.createdAt.localeCompare(b.createdAt));
  const newRestored: QueueTile[] = [];
  for (const img of sorted) {
    if (placedImageIds.has(img.imageId)) continue;
    if (tilesByImageId.has(img.imageId)) continue;
    const label = labelLookup(img.imageId);
    newRestored.push({
      id: idGen(),
      displayLabel: label ?? "復元された画像",
      byteSize: img.originalByteSize,
      origin: "server",
      status: statusFromServerImage(img.status, img.imageId),
    });
  }

  return {
    tiles: [...localTiles, ...updatedTiles, ...newRestored],
  };
}

/** tile から imageId を抽出（無ければ null）。pollOnce / merge で利用。 */
export function imageIdOf(t: QueueTile): string | null {
  switch (t.status.kind) {
    case "processing":
    case "available":
      return t.status.imageId;
    case "failed":
      return t.status.imageId ?? null;
    default:
      return null;
  }
}
