// /ogp/[photobookId]?v=<version> Workers proxy（PR33c）。
//
// 設計参照:
//   - docs/plan/m2-ogp-generation-plan.md §5 採用案 A（Cloudflare Workers proxy）
//
// 流れ:
//   1. Backend `/api/public/photobooks/<id>/ogp` を fetch
//   2. status='generated' なら R2 binding (`env.OGP_BUCKET`) で GetObject
//   3. image/png + Cache-Control: public, max-age=86400 で返す
//   4. status != generated / R2 miss / 例外 → /og/default.png に 302 redirect
//
// セキュリティ:
//   - storage_key 完全値はレスポンスにもログにも出さない（R2 binding 経由でのみ使用）
//   - R2 credentials は binding で隠蔽（環境変数として露出しない）
//   - SNS crawler / 第三者からの直接 GET でも認可不要（OGP は公開情報）
//   - bucket は public OFF を維持（PR33c では bucket 設定変更なし）
import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

import { getCloudflareContext } from "@opennextjs/cloudflare";

// Backend の OGP lookup レスポンス schema（public_handler.go と一致）。
type OgpLookupResponse = {
  status: string;
  version: number;
  image_url_path: string;
};

const DEFAULT_OG_PATH = "/og/default.png";
const CACHE_HEADERS_OK: Record<string, string> = {
  "Cache-Control": "public, max-age=86400, s-maxage=86400",
  "Content-Type": "image/png",
  "X-Robots-Tag": "noindex, nofollow",
};

export async function GET(
  req: NextRequest,
  { params }: { params: Promise<{ photobookId: string }> },
): Promise<Response> {
  const { photobookId } = await params;

  // photobookId は uuid と仮定。完全な validation は Backend 側で実施するが、
  // ここでも基本的な形式チェックでショートサーキット可能。
  if (!/^[0-9a-f-]{36}$/i.test(photobookId)) {
    return NextResponse.redirect(new URL(DEFAULT_OG_PATH, req.url), 302);
  }

  // Backend lookup
  const apiBase = process.env.NEXT_PUBLIC_API_BASE_URL ?? "https://api.vrc-photobook.com";
  let lookup: OgpLookupResponse | null = null;
  try {
    const res = await fetch(`${apiBase}/api/public/photobooks/${photobookId}/ogp`, {
      // SNS crawler 経由のリクエストでも edge cache は短く保つ
      cache: "no-store",
    });
    if (res.ok) {
      lookup = (await res.json()) as OgpLookupResponse;
    }
  } catch {
    lookup = null;
  }
  if (!lookup || lookup.status !== "generated") {
    return NextResponse.redirect(new URL(DEFAULT_OG_PATH, req.url), 302);
  }

  // R2 binding で GetObject。key は **Backend 側で生成済**だが、Workers / Frontend
  // に直接返さない設計のため、photobook_id ベースで R2 を ListObjects + 最新を選ぶ
  // か、または公開可能な「version + photobook_id」での deterministic key 解決が
  // 必要。PR33c の Backend では image_url_path に `<photobook_id>?v=<version>` 形式
  // を返しているため、Backend に追加で `storage_key` を返してもらうか、Workers が
  // ListObjects で prefix `photobooks/<photobook_id>/ogp/` を 1 件取り出す。
  //
  // 採用方針: Workers が R2 List で prefix の最新オブジェクトを返す（OGP は generated
  // 時に 1 アクティブ object のみが論理的に存在する想定。stale → 再生成で旧 object は
  // GC で回収される）。
  let env: CloudflareEnv | null = null;
  try {
    env = getCloudflareContext().env;
  } catch {
    env = null;
  }
  if (!env || !env.OGP_BUCKET) {
    return NextResponse.redirect(new URL(DEFAULT_OG_PATH, req.url), 302);
  }
  const prefix = `photobooks/${photobookId}/ogp/`;
  type Listing = Awaited<ReturnType<OgpR2Bucket["list"]>>;
  let listing: Listing | null = null;
  try {
    listing = await env.OGP_BUCKET.list({ prefix, limit: 50 });
  } catch {
    listing = null;
  }
  if (!listing || listing.objects.length === 0) {
    return NextResponse.redirect(new URL(DEFAULT_OG_PATH, req.url), 302);
  }
  // 最新の uploaded を選ぶ（key 内の <ogp_id> が UUIDv7 で時系列順、または uploaded 時刻順）
  let latest = listing.objects[0];
  for (const o of listing.objects) {
    if (o.uploaded.getTime() > latest.uploaded.getTime()) {
      latest = o;
    }
  }

  type R2Body = Awaited<ReturnType<OgpR2Bucket["get"]>>;
  let obj: R2Body = null;
  try {
    obj = await env.OGP_BUCKET.get(latest.key);
  } catch {
    obj = null;
  }
  if (!obj) {
    return NextResponse.redirect(new URL(DEFAULT_OG_PATH, req.url), 302);
  }

  return new Response(obj.body, {
    status: 200,
    headers: CACHE_HEADERS_OK,
  });
}
