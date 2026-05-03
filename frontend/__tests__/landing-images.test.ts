// β-2c landing image asset の existence + size guard。
//
// 検査:
//   - frontend/public/img/landing/ に 14 file (hero / mock-cover / sample-01..05 × .webp/.jpg) 存在
//   - 合計サイズ ≤ 4 MiB (4194304 bytes) — plan §3 / §3.6 目標
//   - 各 file > 1 KB (空ファイル・破損 guard)
//   - 公開 dir に raw .png が混入していない (build script 内 guard と二重化)
//
// build pipeline: frontend/scripts/build-landing-images.sh
// 設計参照: docs/plan/m2-design-refresh-stop-beta-2-plan.md §3
//
// vitest は repo-root を cwd に走るとは限らないため、このテストファイルからの相対 path で
// 解決する (`__dirname` 相当を ESM で再構築)。

import { describe, expect, it } from "vitest";
import { existsSync, readdirSync, statSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const __filename = fileURLToPath(import.meta.url);
const LANDING_DIR = resolve(dirname(__filename), "..", "public", "img", "landing");

const STABLE_NAMES = [
  "hero",
  "mock-cover",
  "sample-01",
  "sample-02",
  "sample-03",
  "sample-04",
  "sample-05",
] as const;

const MAX_TOTAL_BYTES = 4 * 1024 * 1024; // 4 MiB
const MIN_FILE_BYTES = 1024; // 1 KB - 空ファイル guard

describe("landing image asset (β-2c)", () => {
  it("正常_landing_dir_が存在する", () => {
    expect(existsSync(LANDING_DIR)).toBe(true);
  });

  it("正常_14_file_全て存在 (hero/mock-cover/sample-01..05 × webp/jpg)", () => {
    for (const slug of STABLE_NAMES) {
      const webp = resolve(LANDING_DIR, `${slug}.webp`);
      const jpg = resolve(LANDING_DIR, `${slug}.jpg`);
      expect(existsSync(webp), `missing: ${slug}.webp`).toBe(true);
      expect(existsSync(jpg), `missing: ${slug}.jpg`).toBe(true);
    }
  });

  it("正常_各_file_size_>_1_KB_かつ_合計_≤_4_MiB", () => {
    let total = 0;
    for (const slug of STABLE_NAMES) {
      for (const ext of ["webp", "jpg"] as const) {
        const f = resolve(LANDING_DIR, `${slug}.${ext}`);
        const size = statSync(f).size;
        expect(size, `${slug}.${ext} size > 1 KB`).toBeGreaterThan(MIN_FILE_BYTES);
        total += size;
      }
    }
    expect(total, `total ${total} bytes ≤ ${MAX_TOTAL_BYTES} (4 MiB)`).toBeLessThanOrEqual(
      MAX_TOTAL_BYTES,
    );
  });

  it("正常_公開_dir_に_raw_PNG_が混入していない", () => {
    const entries = readdirSync(LANDING_DIR);
    const pngs = entries.filter((e) => e.toLowerCase().endsWith(".png"));
    expect(pngs, `unexpected raw PNG in landing dir: ${pngs.join(", ")}`).toEqual([]);
  });

  it("正常_landing_dir_に_想定外の_file_は無い (14_file_only)", () => {
    const entries = readdirSync(LANDING_DIR).sort();
    const expected = STABLE_NAMES.flatMap((s) => [`${s}.jpg`, `${s}.webp`]).sort();
    expect(entries).toEqual(expected);
  });
});
