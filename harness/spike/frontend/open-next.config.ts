/**
 * OpenNext for Cloudflare 設定
 *
 * 検証目的:
 *  - @opennextjs/cloudflare による build / preview / deploy が成立するか
 *  - Cloudflare Workers + Static Assets バインディング構成下で
 *    SSR / Cookie / redirect / ヘッダ制御が next-on-pages 版と同等に動くか
 *
 * 参考: https://opennext.js.org/cloudflare
 */
import { defineCloudflareConfig } from "@opennextjs/cloudflare";

export default defineCloudflareConfig({
  // M1 PoC では incremental cache 等の追加バインディングは設定しない（最小構成）
});
