// /prepare 画面の filename 補助 cache（plan v2 §3.4 P0-c）。
//
// 役割:
//   - upload 開始時に「imageId → filename」の mapping を localStorage に保存
//   - reload 後 server から imageId を受け取った際、対応 filename を即時表示できる
//   - server に filename を送る経路を増やさず、UI 補助のみ
//
// セキュリティ:
//   - localStorage に格納するのは filename だけ。raw token / Cookie / storage_key は触らない
//   - UI 表示専用。imageId 自体は labelLookup の入力として渡るが、戻り値は filename のみ
//   - photobookId 単位で名前空間化（cross-photobook leak を防ぐ）
//   - 上限 50 entry / photobook + 30 日 TTL（古い entry は自動 GC）
//   - SSR / Worker 等 localStorage 不在環境では no-op フォールバック

const STORAGE_PREFIX = "vrcpb-prepare-labels:";
const MAX_ENTRIES = 50;
const TTL_MS = 30 * 24 * 60 * 60 * 1000;

type StoredEntry = {
  /** filename（user-friendly な表示名）。 */
  label: string;
  /** epoch ms。GC 用。 */
  ts: number;
};

type Stored = Record<string, StoredEntry>;

function storageKey(photobookId: string): string {
  return `${STORAGE_PREFIX}${photobookId}`;
}

function safeGetStorage(): Storage | null {
  if (typeof window === "undefined") return null;
  try {
    return window.localStorage;
  } catch {
    return null;
  }
}

function readAll(photobookId: string): Stored {
  const ls = safeGetStorage();
  if (ls === null) return {};
  let raw: string | null;
  try {
    raw = ls.getItem(storageKey(photobookId));
  } catch {
    return {};
  }
  if (raw === null || raw === "") return {};
  try {
    const parsed = JSON.parse(raw) as unknown;
    if (typeof parsed !== "object" || parsed === null) return {};
    return parsed as Stored;
  } catch {
    return {};
  }
}

function writeAll(photobookId: string, data: Stored): void {
  const ls = safeGetStorage();
  if (ls === null) return;
  try {
    ls.setItem(storageKey(photobookId), JSON.stringify(data));
  } catch {
    // QuotaExceeded / Safari private mode 等は無視（label 補助は best-effort）
  }
}

/** 古い / 上限超過 entry を削除した結果を返す（in place しない）。 */
function compact(data: Stored, now: number): Stored {
  const fresh: Stored = {};
  for (const [k, v] of Object.entries(data)) {
    if (typeof v !== "object" || v === null) continue;
    if (typeof v.label !== "string" || typeof v.ts !== "number") continue;
    if (now - v.ts > TTL_MS) continue;
    fresh[k] = v;
  }
  const entries = Object.entries(fresh);
  if (entries.length <= MAX_ENTRIES) return fresh;
  // 古い順に削る（ts 昇順）
  entries.sort((a, b) => a[1].ts - b[1].ts);
  const trimmed = entries.slice(entries.length - MAX_ENTRIES);
  return Object.fromEntries(trimmed);
}

/** imageId に対応する filename を localStorage に保存する。 */
export function rememberLabel(
  photobookId: string,
  imageId: string,
  filename: string,
  now: number = Date.now(),
): void {
  if (imageId === "" || filename === "") return;
  const all = readAll(photobookId);
  all[imageId] = { label: filename, ts: now };
  writeAll(photobookId, compact(all, now));
}

/** imageId に対応する filename を返す（無ければ null）。 */
export function lookupLabel(
  photobookId: string,
  imageId: string,
  now: number = Date.now(),
): string | null {
  if (imageId === "") return null;
  const all = readAll(photobookId);
  const entry = all[imageId];
  if (entry === undefined) return null;
  if (typeof entry.label !== "string") return null;
  if (typeof entry.ts !== "number") return null;
  if (now - entry.ts > TTL_MS) return null;
  return entry.label;
}

/** photobook 単位で全削除（debug / cleanup 用）。 */
export function clearLabels(photobookId: string): void {
  const ls = safeGetStorage();
  if (ls === null) return;
  try {
    ls.removeItem(storageKey(photobookId));
  } catch {
    // ignore
  }
}
