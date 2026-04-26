// OpenNext for Cloudflare Workers の最小設定。
//
// PR5 段階では incremental cache / queue / D1 / KV / R2 のような追加バインディングは設定しない。
// 後続 PR で必要が生じたタイミング（Image アップロード等）で個別に追加する。
//
// 詳細: https://opennext.js.org/cloudflare
import { defineCloudflareConfig } from "@opennextjs/cloudflare";

export default defineCloudflareConfig({
  // 最小構成。
});
