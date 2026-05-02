// Browser-side 画像圧縮モジュール。
//
// 設計意図:
//   - VRChat の典型的な PNG 撮影（2160×3840, 13〜18 MB）を upload 上限 10 MB に収める
//   - 画質劣化を最小化（quality 0.85 / long edge 2400px、必要時 quality 0.7 fallback）
//   - 元 file が既に 10 MB 以下の JPEG なら no-op（再エンコードによる劣化を避ける）
//   - 巨大 file（既定 50 MB 超）はメモリ保護のため decode 前に拒否
//   - browser-native API（createImageBitmap / OffscreenCanvas / canvas.toBlob）依存
//   - test では DI で decode / encode / canvas factory を差し替え可能（vitest=node 環境用）
//
// セキュリティ:
//   - file 名 / MIME 以外の中身（バイト列）を log / console に出さない
//   - 圧縮後の File は元 file 名から拡張子を変えるのみ。新規 unique 名を付けない
//
// 参照:
//   - frontend/lib/upload.ts MAX_UPLOAD_BYTE_SIZE（10 MB、validateFile）
//   - backend/internal/imageupload/internal/usecase/issue_upload_intent.go MaxUploadByteSize
//   - .agents/rules/security-guard.md

/** 圧縮結果。 */
export type CompressResult = {
  /** 圧縮後（または no-op の元）File。常に upload に使える。 */
  file: File;
  /** 解像度を縮小したか。 */
  resized: boolean;
  /** 再エンコードしたか（false = 元 file をそのまま採用）。 */
  recompressed: boolean;
  /** 元サイズ（bytes）。 */
  originalBytes: number;
  /** 圧縮後サイズ（bytes）。 */
  outputBytes: number;
  /** 採用した quality（recompressed=true のみ意味あり）。 */
  appliedQuality?: number;
};

/** 圧縮失敗種別。 */
export type CompressErrorKind =
  | "input_too_large"
  | "decode_failed"
  | "encode_failed"
  | "still_too_large";

export class CompressionError extends Error {
  readonly kind: CompressErrorKind;
  constructor(kind: CompressErrorKind, message?: string) {
    super(message ?? kind);
    this.kind = kind;
  }
}

/** 圧縮計画（pure）。 */
export type CompressionPlan = {
  /** decode をスキップする（既に小さい JPEG など）。 */
  skip: boolean;
  /** 出力解像度（pixel）。skip=true なら 0,0。 */
  targetWidth: number;
  targetHeight: number;
  /** 縮小したか。 */
  resized: boolean;
};

export type PlanInput = {
  inputBytes: number;
  inputMime: string;
  sourceWidth: number;
  sourceHeight: number;
  /** 圧縮目標 byte（既定 10 MB）。 */
  targetMaxBytes: number;
  /** 縮小後の長辺最大 px（既定 2400）。 */
  maxLongEdge: number;
};

/**
 * pure 関数: 入力サイズ/解像度から圧縮 plan を立てる。
 *
 * - 既に target 以下の JPEG → skip=true（再エンコードしない）
 * - それ以外: maxLongEdge を超えていれば縮小、出力 mime は image/jpeg を想定
 */
export function planCompression(input: PlanInput): CompressionPlan {
  if (
    input.inputMime === "image/jpeg" &&
    input.inputBytes > 0 &&
    input.inputBytes <= input.targetMaxBytes
  ) {
    return { skip: true, targetWidth: 0, targetHeight: 0, resized: false };
  }
  const longEdge = Math.max(input.sourceWidth, input.sourceHeight);
  if (longEdge <= 0) {
    // decode が成功しているはずなのでここには来ない想定
    return {
      skip: false,
      targetWidth: input.sourceWidth,
      targetHeight: input.sourceHeight,
      resized: false,
    };
  }
  if (longEdge > input.maxLongEdge) {
    const scale = input.maxLongEdge / longEdge;
    return {
      skip: false,
      targetWidth: Math.max(1, Math.round(input.sourceWidth * scale)),
      targetHeight: Math.max(1, Math.round(input.sourceHeight * scale)),
      resized: true,
    };
  }
  return {
    skip: false,
    targetWidth: input.sourceWidth,
    targetHeight: input.sourceHeight,
    resized: false,
  };
}

// ===== DI 用の型（test 容易化のため） =====

/** 復号した画像のハンドル。drawTo で 2D context に転写し、close で解放する。 */
export type DecodedImage = {
  width: number;
  height: number;
  drawTo: (
    ctx: CanvasRenderingContext2D | OffscreenCanvasRenderingContext2D,
    targetWidth: number,
    targetHeight: number,
  ) => void;
  close: () => void;
};

export type DecodeFn = (file: Blob) => Promise<DecodedImage>;

/** Canvas factory。同 size の 2D 描画コンテキストを得るための抽象。 */
export type CanvasHandle = {
  width: number;
  height: number;
  getContext2D: () =>
    | CanvasRenderingContext2D
    | OffscreenCanvasRenderingContext2D
    | null;
  toBlob: (mime: string, quality: number) => Promise<Blob>;
};

export type CanvasFactory = (width: number, height: number) => CanvasHandle;

// ===== Browser default 実装 =====

const browserDecode: DecodeFn = async (blob) => {
  // createImageBitmap はモダンブラウザで広くサポート（Chrome/FF/Safari/Edge）。
  const bitmap = await createImageBitmap(blob);
  return {
    width: bitmap.width,
    height: bitmap.height,
    drawTo: (ctx, w, h) => {
      ctx.drawImage(bitmap, 0, 0, w, h);
    },
    close: () => bitmap.close(),
  };
};

const browserCanvasFactory: CanvasFactory = (width, height) => {
  // OffscreenCanvas が使える環境（Worker / 多くのブラウザ）優先
  if (typeof OffscreenCanvas !== "undefined") {
    const oc = new OffscreenCanvas(width, height);
    return {
      width,
      height,
      getContext2D: () => oc.getContext("2d"),
      toBlob: async (mime, quality) => oc.convertToBlob({ type: mime, quality }),
    };
  }
  // HTMLCanvasElement fallback（document アクセスが必要、SSR/Worker では使えない）
  if (typeof document !== "undefined") {
    const c = document.createElement("canvas");
    c.width = width;
    c.height = height;
    return {
      width,
      height,
      getContext2D: () => c.getContext("2d"),
      toBlob: (mime, quality) =>
        new Promise<Blob>((resolve, reject) => {
          c.toBlob(
            (b) =>
              b
                ? resolve(b)
                : reject(new CompressionError("encode_failed", "toBlob returned null")),
            mime,
            quality,
          );
        }),
    };
  }
  throw new CompressionError("encode_failed", "no canvas factory available");
};

// ===== 公開: compressImageForUpload =====

export type CompressOptions = {
  /** 圧縮目標 byte 上限（既定 10 MB）。 */
  targetMaxBytes?: number;
  /** decode する前に弾く入力 byte 上限（既定 50 MB、メモリ保護）。 */
  maxInputBytes?: number;
  /** 縮小後の長辺最大 px（既定 2400）。 */
  maxLongEdge?: number;
  /** 一次 quality（既定 0.85）。 */
  primaryQuality?: number;
  /** target に収まらない時の fallback quality（既定 0.7）。 */
  fallbackQuality?: number;
  /** test 用 DI: decode / canvas factory を差し替え可能。 */
  decode?: DecodeFn;
  canvasFactory?: CanvasFactory;
};

const DEFAULTS = {
  targetMaxBytes: 10 * 1024 * 1024,
  maxInputBytes: 50 * 1024 * 1024,
  maxLongEdge: 2400,
  primaryQuality: 0.85,
  fallbackQuality: 0.7,
} as const;

const OUTPUT_MIME = "image/jpeg";

/** 元 file 名の拡張子を .jpg に変える（拡張子無しの場合は末尾に付与）。 */
export function renameToJpg(name: string): string {
  if (!name) return "image.jpg";
  const dot = name.lastIndexOf(".");
  if (dot <= 0) return `${name}.jpg`;
  return `${name.slice(0, dot)}.jpg`;
}

/**
 * upload 用に画像を圧縮する。
 *
 * 仕様:
 *   - 元 file が既に target 以下の JPEG → no-op（recompressed=false）
 *   - それ以外:
 *     1. maxInputBytes 超過 → CompressionError("input_too_large")
 *     2. decode（createImageBitmap）
 *     3. maxLongEdge 超過なら縮小、それ以下なら同 size
 *     4. JPEG q=primaryQuality でエンコード
 *     5. それでも target を超えるなら q=fallbackQuality で再エンコード
 *     6. それでも超えるなら CompressionError("still_too_large")
 */
export async function compressImageForUpload(
  file: File,
  options: CompressOptions = {},
): Promise<CompressResult> {
  const targetMaxBytes = options.targetMaxBytes ?? DEFAULTS.targetMaxBytes;
  const maxInputBytes = options.maxInputBytes ?? DEFAULTS.maxInputBytes;
  const maxLongEdge = options.maxLongEdge ?? DEFAULTS.maxLongEdge;
  const primaryQuality = options.primaryQuality ?? DEFAULTS.primaryQuality;
  const fallbackQuality = options.fallbackQuality ?? DEFAULTS.fallbackQuality;
  const decode = options.decode ?? browserDecode;
  const canvasFactory = options.canvasFactory ?? browserCanvasFactory;

  if (file.size > maxInputBytes) {
    throw new CompressionError(
      "input_too_large",
      `input ${file.size} bytes exceeds limit ${maxInputBytes}`,
    );
  }

  // 既に target 以下の JPEG なら no-op（劣化させない、I/O 節約）
  if (file.type === "image/jpeg" && file.size <= targetMaxBytes) {
    return {
      file,
      resized: false,
      recompressed: false,
      originalBytes: file.size,
      outputBytes: file.size,
    };
  }

  let decoded: DecodedImage;
  try {
    decoded = await decode(file);
  } catch (e) {
    throw new CompressionError(
      "decode_failed",
      e instanceof Error ? e.message : "decode failed",
    );
  }

  try {
    const plan = planCompression({
      inputBytes: file.size,
      inputMime: file.type,
      sourceWidth: decoded.width,
      sourceHeight: decoded.height,
      targetMaxBytes,
      maxLongEdge,
    });

    // skip=true（同 mime + 同サイズ）はここまで来ない（上で early return 済）が念のため
    if (plan.skip) {
      return {
        file,
        resized: false,
        recompressed: false,
        originalBytes: file.size,
        outputBytes: file.size,
      };
    }

    const canvas = canvasFactory(plan.targetWidth, plan.targetHeight);
    const ctx = canvas.getContext2D();
    if (!ctx) {
      throw new CompressionError("encode_failed", "no 2d context");
    }
    decoded.drawTo(ctx, plan.targetWidth, plan.targetHeight);

    let blob = await canvas.toBlob(OUTPUT_MIME, primaryQuality);
    let usedQuality = primaryQuality;
    if (blob.size > targetMaxBytes) {
      blob = await canvas.toBlob(OUTPUT_MIME, fallbackQuality);
      usedQuality = fallbackQuality;
    }
    if (blob.size > targetMaxBytes) {
      throw new CompressionError(
        "still_too_large",
        `compressed ${blob.size} bytes still exceeds target ${targetMaxBytes}`,
      );
    }

    const newName = renameToJpg(file.name);
    const newFile = new File([blob], newName, {
      type: OUTPUT_MIME,
      lastModified: file.lastModified,
    });
    return {
      file: newFile,
      resized: plan.resized,
      recompressed: true,
      originalBytes: file.size,
      outputBytes: blob.size,
      appliedQuality: usedQuality,
    };
  } finally {
    try {
      decoded.close();
    } catch {
      // close 失敗は致命ではない
    }
  }
}
