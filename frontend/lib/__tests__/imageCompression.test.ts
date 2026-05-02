// imageCompression のユニットテスト。
//
// 観点:
//   - planCompression（pure）: skip / 縮小 / 据え置き
//   - renameToJpg: 拡張子置換 / 拡張子無し
//   - compressImageForUpload: 既に小さい JPEG は no-op / VRChat PNG は縮小 + JPEG 化 /
//                              入力過大は input_too_large / fallback quality / still_too_large
//
// 注意:
//   - vitest の environment は node のため、createImageBitmap / OffscreenCanvas はネイティブに無い
//   - 本テストは DI（decode + canvasFactory mock）でブラウザ依存を排除
//   - 実ブラウザ動作は STOP δ（Workers redeploy）後の Safari/Chrome 実機確認で担保

import { describe, expect, it } from "vitest";

import {
  CompressionError,
  compressImageForUpload,
  planCompression,
  renameToJpg,
  type CanvasFactory,
  type DecodeFn,
} from "@/lib/imageCompression";

// ---------- 共通の mock ----------

function fakeDecode(width: number, height: number): DecodeFn {
  return async () => ({
    width,
    height,
    drawTo: () => undefined,
    close: () => undefined,
  });
}

/**
 * outputBytes を順番に返す canvas mock。
 * primaryQuality 1 回目で blobBytes[0]、fallback 2 回目で blobBytes[1] を返す。
 */
function fakeCanvasFactory(blobBytes: number[]): CanvasFactory {
  let idx = 0;
  return (w, h) => ({
    width: w,
    height: h,
    getContext2D: () =>
      ({
        drawImage: () => undefined,
      }) as unknown as CanvasRenderingContext2D,
    toBlob: async () => {
      const size = blobBytes[Math.min(idx, blobBytes.length - 1)] ?? 1;
      idx += 1;
      // Node 環境で File は globalThis.File 経由で利用可能（Node 20+）
      return new Blob([new Uint8Array(size)], { type: "image/jpeg" });
    },
  });
}

function makeFile(name: string, bytes: number, mime: string): File {
  return new File([new Uint8Array(bytes)], name, { type: mime });
}

// ---------- planCompression（pure） ----------

describe("planCompression", () => {
  const base = {
    targetMaxBytes: 10 * 1024 * 1024,
    maxLongEdge: 2400,
  };

  it("正常_既にtarget以下のJPEGはskip", () => {
    const plan = planCompression({
      inputBytes: 1_000_000,
      inputMime: "image/jpeg",
      sourceWidth: 2160,
      sourceHeight: 3840,
      ...base,
    });
    expect(plan.skip).toBe(true);
    expect(plan.resized).toBe(false);
  });

  it("正常_PNGは常に再エンコード対象（skip=false）", () => {
    const plan = planCompression({
      inputBytes: 1_000_000,
      inputMime: "image/png",
      sourceWidth: 2160,
      sourceHeight: 3840,
      ...base,
    });
    expect(plan.skip).toBe(false);
  });

  it("正常_target超のJPEGも再エンコード対象", () => {
    const plan = planCompression({
      inputBytes: 11 * 1024 * 1024,
      inputMime: "image/jpeg",
      sourceWidth: 2160,
      sourceHeight: 3840,
      ...base,
    });
    expect(plan.skip).toBe(false);
  });

  it("正常_long edge >maxLongEdgeで縮小（VRChat 2160x3840 → 1350x2400）", () => {
    const plan = planCompression({
      inputBytes: 14_000_000,
      inputMime: "image/png",
      sourceWidth: 2160,
      sourceHeight: 3840,
      ...base,
    });
    expect(plan.skip).toBe(false);
    expect(plan.resized).toBe(true);
    expect(plan.targetHeight).toBe(2400);
    expect(plan.targetWidth).toBe(1350);
  });

  it("正常_long edge <=maxLongEdgeなら据え置き", () => {
    const plan = planCompression({
      inputBytes: 11_000_000,
      inputMime: "image/png",
      sourceWidth: 1920,
      sourceHeight: 1080,
      ...base,
    });
    expect(plan.skip).toBe(false);
    expect(plan.resized).toBe(false);
    expect(plan.targetWidth).toBe(1920);
    expect(plan.targetHeight).toBe(1080);
  });

  it("正常_横長画像も同じロジックで縮小（3840x2160 → 2400x1350）", () => {
    const plan = planCompression({
      inputBytes: 14_000_000,
      inputMime: "image/png",
      sourceWidth: 3840,
      sourceHeight: 2160,
      ...base,
    });
    expect(plan.targetWidth).toBe(2400);
    expect(plan.targetHeight).toBe(1350);
    expect(plan.resized).toBe(true);
  });
});

// ---------- renameToJpg ----------

describe("renameToJpg", () => {
  it("正常_PNGの拡張子を.jpgに置換", () => {
    expect(renameToJpg("vrc_photo.png")).toBe("vrc_photo.jpg");
  });

  it("正常_WebPの拡張子を.jpgに置換", () => {
    expect(renameToJpg("vrc_photo.webp")).toBe("vrc_photo.jpg");
  });

  it("正常_既に.jpgならそのまま", () => {
    expect(renameToJpg("vrc_photo.jpg")).toBe("vrc_photo.jpg");
  });

  it("正常_拡張子無しは末尾に.jpgを付与", () => {
    expect(renameToJpg("vrc_photo")).toBe("vrc_photo.jpg");
  });

  it("正常_先頭ドットは拡張子扱いしない", () => {
    expect(renameToJpg(".hidden")).toBe(".hidden.jpg");
  });

  it("正常_空文字はimage.jpg", () => {
    expect(renameToJpg("")).toBe("image.jpg");
  });
});

// ---------- compressImageForUpload ----------

describe("compressImageForUpload no-op path", () => {
  it("正常_既に10MB以下のJPEGはno-op（元File返却・recompressed=false）", async () => {
    const file = makeFile("small.jpg", 2 * 1024 * 1024, "image/jpeg");
    const out = await compressImageForUpload(file, {
      decode: fakeDecode(2160, 3840),
      canvasFactory: fakeCanvasFactory([1]),
    });
    expect(out.recompressed).toBe(false);
    expect(out.resized).toBe(false);
    expect(out.file).toBe(file); // 同 instance（再エンコードしない）
    expect(out.originalBytes).toBe(2 * 1024 * 1024);
    expect(out.outputBytes).toBe(2 * 1024 * 1024);
  });
});

describe("compressImageForUpload PNG → JPEG conversion", () => {
  it("正常_VRChat PNG 13.5MB → 縮小+JPEG q=0.85で2MB相当", async () => {
    const file = makeFile("vrc_2160x3840.png", 13_500_000, "image/png");
    // primary quality で 2 MB 相当を返す mock
    const out = await compressImageForUpload(file, {
      decode: fakeDecode(2160, 3840),
      canvasFactory: fakeCanvasFactory([2_000_000]),
    });
    expect(out.recompressed).toBe(true);
    expect(out.resized).toBe(true);
    expect(out.outputBytes).toBe(2_000_000);
    expect(out.appliedQuality).toBe(0.85);
    expect(out.file.type).toBe("image/jpeg");
    expect(out.file.name).toBe("vrc_2160x3840.jpg");
    expect(out.file.size).toBe(2_000_000);
  });

  it("正常_既存JPEGがtarget超なら再エンコード（10MB超JPEG → 縮小）", async () => {
    const file = makeFile("big.jpg", 12 * 1024 * 1024, "image/jpeg");
    const out = await compressImageForUpload(file, {
      decode: fakeDecode(2160, 3840),
      canvasFactory: fakeCanvasFactory([3_000_000]),
    });
    expect(out.recompressed).toBe(true);
    expect(out.resized).toBe(true);
    expect(out.outputBytes).toBe(3_000_000);
    expect(out.file.name).toBe("big.jpg");
    expect(out.file.type).toBe("image/jpeg");
  });

  it("正常_WebP入力もJPEGとして出力", async () => {
    const file = makeFile("vrc.webp", 11 * 1024 * 1024, "image/webp");
    const out = await compressImageForUpload(file, {
      decode: fakeDecode(2160, 3840),
      canvasFactory: fakeCanvasFactory([2_500_000]),
    });
    expect(out.recompressed).toBe(true);
    expect(out.file.type).toBe("image/jpeg");
    expect(out.file.name).toBe("vrc.jpg");
  });
});

describe("compressImageForUpload fallback quality", () => {
  it("正常_q=0.85でtarget超ならq=0.7にfallbackして成功", async () => {
    const file = makeFile("vrc.png", 14_000_000, "image/png");
    // 1 回目 11 MB（target=10 MB 超）, 2 回目 8 MB（target 以下）
    const out = await compressImageForUpload(file, {
      decode: fakeDecode(2160, 3840),
      canvasFactory: fakeCanvasFactory([11 * 1024 * 1024, 8 * 1024 * 1024]),
    });
    expect(out.recompressed).toBe(true);
    expect(out.appliedQuality).toBe(0.7);
    expect(out.outputBytes).toBe(8 * 1024 * 1024);
  });
});

describe("compressImageForUpload error path", () => {
  it("異常_input_too_large（既定 50MB 超）はdecode前に拒否", async () => {
    const file = makeFile("huge.png", 60 * 1024 * 1024, "image/png");
    let called = 0;
    await expect(
      compressImageForUpload(file, {
        decode: async () => {
          called += 1;
          return { width: 2160, height: 3840, drawTo: () => undefined, close: () => undefined };
        },
        canvasFactory: fakeCanvasFactory([1]),
      }),
    ).rejects.toBeInstanceOf(CompressionError);
    expect(called).toBe(0); // decode が呼ばれない
  });

  it("異常_decode失敗はdecode_failed", async () => {
    const file = makeFile("vrc.png", 13_000_000, "image/png");
    await expect(
      compressImageForUpload(file, {
        decode: async () => {
          throw new Error("createImageBitmap failed");
        },
        canvasFactory: fakeCanvasFactory([1]),
      }),
    ).rejects.toMatchObject({ kind: "decode_failed" });
  });

  it("異常_q=0.85もq=0.7も超なら still_too_large", async () => {
    const file = makeFile("vrc.png", 13_000_000, "image/png");
    await expect(
      compressImageForUpload(file, {
        decode: fakeDecode(2160, 3840),
        canvasFactory: fakeCanvasFactory([
          11 * 1024 * 1024,
          11 * 1024 * 1024,
        ]),
      }),
    ).rejects.toMatchObject({ kind: "still_too_large" });
  });

  it("異常_2D context取得失敗は encode_failed", async () => {
    const file = makeFile("vrc.png", 13_000_000, "image/png");
    const factory: CanvasFactory = (w, h) => ({
      width: w,
      height: h,
      getContext2D: () => null,
      toBlob: async () => new Blob([new Uint8Array(1)], { type: "image/jpeg" }),
    });
    await expect(
      compressImageForUpload(file, {
        decode: fakeDecode(2160, 3840),
        canvasFactory: factory,
      }),
    ).rejects.toMatchObject({ kind: "encode_failed" });
  });

  it("正常_decode handle の close が呼ばれる（リソース解放）", async () => {
    const file = makeFile("vrc.png", 13_000_000, "image/png");
    let closed = false;
    await compressImageForUpload(file, {
      decode: async () => ({
        width: 2160,
        height: 3840,
        drawTo: () => undefined,
        close: () => {
          closed = true;
        },
      }),
      canvasFactory: fakeCanvasFactory([2_000_000]),
    });
    expect(closed).toBe(true);
  });
});
