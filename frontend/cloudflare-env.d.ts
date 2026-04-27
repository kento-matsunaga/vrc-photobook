// PR33c: CloudflareEnv 拡張型。OGP 配信用 R2 binding を追加。
// wrangler.jsonc の r2_buckets と一致させる（binding=OGP_BUCKET、bucket=vrcpb-images）。
//
// 注意:
//   `@cloudflare/workers-types` を別途 install していないため、
//   ここで R2 関連の最小 interface も local に declare する。
//   `wrangler deploy` 時には workerd 側の実 type が使われるため runtime 影響はない。

declare global {
  interface OgpR2Object {
    key: string;
    uploaded: Date;
    size: number;
  }

  interface OgpR2ObjectBody {
    body: ReadableStream<Uint8Array>;
  }

  interface OgpR2Bucket {
    list(opts: { prefix?: string; limit?: number }): Promise<{
      objects: OgpR2Object[];
    }>;
    get(key: string): Promise<OgpR2ObjectBody | null>;
  }

  interface CloudflareEnv {
    /**
     * OGP 画像配信用 R2 binding。bucket は `vrcpb-images`（public OFF）。
     * `app/ogp/[photobookId]/route.ts` で `env.OGP_BUCKET.get(key)` を呼ぶ。
     */
    OGP_BUCKET?: OgpR2Bucket;
  }
}

export {};
