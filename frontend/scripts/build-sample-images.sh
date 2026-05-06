#!/usr/bin/env bash
#
# claude-opus47/viewer-photobook-feel-v2: TESTImage/ raw PNG →
# frontend/public/img/sample/ の WebP/JPEG variants を生成する pipeline。
#
# 入力: repo-root の TESTImage/ にある VRChat raw PNG
#       (`.gitignore` 同等で git 管理外、user-local 入力)
# 出力: repo-root の frontend/public/img/sample/ に
#       sample-cover / sample-01..07 × 2 size (display / thumb) × 2 format =
#       32 file (Lightbox / PageHero / Cover で参照)
#       本 sample は dev preview (`/p/__sample__`) のみで使う、production routes 不参照
#
# 使い方 (repo root から):
#   bash frontend/scripts/build-sample-images.sh
#
# 必須 tool (PATH 上):
#   - cwebp     (libwebp; PNG/JPEG → WebP)
#   - cjpeg     (libjpeg-turbo; PPM → JPEG)
#   - convert   (ImageMagick 6+; PNG → PPM 中間 / リサイズ)
#   - identify  (ImageMagick 6+; 寸法確認)
# install (Ubuntu): sudo apt-get install -y webp libjpeg-turbo-progs imagemagick
#
# raw → stable name の確定 mapping は本 script 内の MAPPING 配列で管理。
# raw filename は React component 側に書かない。
#
# 生成寸法:
#   - cover  display: 1200×1800 (portrait 縦長 cover)
#   - cover  thumb  : 300×450
#   - sample display: 1600×900 (landscape 16:9)
#   - sample thumb  : 480×270
# 品質:
#   - WebP q72 / JPEG q78
#   - -strip / -metadata none で EXIF / XMP 等メタデータ除去

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
INPUT_DIR="$REPO_ROOT/TESTImage"
OUTPUT_DIR="$REPO_ROOT/frontend/public/img/sample"

for tool in cwebp cjpeg convert identify; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo "ERROR: required tool '$tool' not found in PATH" >&2
    echo "       install: sudo apt-get install -y webp libjpeg-turbo-progs imagemagick" >&2
    exit 1
  fi
done

if [ ! -d "$INPUT_DIR" ]; then
  echo "ERROR: input dir not found: $INPUT_DIR" >&2
  exit 1
fi

mkdir -p "$OUTPUT_DIR"

# stable_name|raw_relative_path|orientation (P=portrait / L=landscape)
MAPPING=(
  "sample-cover|82E37915-2A9D-4B69-B890-5427DB9BFC71VRChat_2026-04-06_23-37-30.108_2160x3840.png|P"
  "sample-01|VRChat_2026-04-29_00-28-49.430_3840x2160.png|L"
  "sample-02|VRChat_2026-04-29_00-31-03.185_3840x2160.png|L"
  "sample-03|VRChat_2026-04-29_00-32-43.792_3840x2160.png|L"
  "sample-04|VRChat_2026-04-29_00-43-31.032_3840x2160.png|L"
  "sample-05|VRChat_2026-04-29_00-43-40.811_3840x2160.png|L"
  "sample-06|VRChat_2026-04-29_00-58-37.971_3840x2160.png|L"
  "sample-07|VRChat_2026-04-29_00-59-03.868_3840x2160.png|L"
)

PORTRAIT_DISP_W=1200
PORTRAIT_DISP_H=1800
PORTRAIT_THUMB_W=300
PORTRAIT_THUMB_H=450

LANDSCAPE_DISP_W=1600
LANDSCAPE_DISP_H=900
LANDSCAPE_THUMB_W=480
LANDSCAPE_THUMB_H=270

WEBP_Q=72
JPEG_Q=78

process_one() {
  local stable="$1"
  local raw_rel="$2"
  local orientation="$3"
  local input="$INPUT_DIR/$raw_rel"

  if [ ! -f "$input" ]; then
    echo "  SKIP (missing): $stable ← $raw_rel" >&2
    return 0
  fi

  local disp_w disp_h thumb_w thumb_h
  if [ "$orientation" = "P" ]; then
    disp_w=$PORTRAIT_DISP_W
    disp_h=$PORTRAIT_DISP_H
    thumb_w=$PORTRAIT_THUMB_W
    thumb_h=$PORTRAIT_THUMB_H
  else
    disp_w=$LANDSCAPE_DISP_W
    disp_h=$LANDSCAPE_DISP_H
    thumb_w=$LANDSCAPE_THUMB_W
    thumb_h=$LANDSCAPE_THUMB_H
  fi

  local tmp_disp_ppm="$OUTPUT_DIR/.tmp.${stable}.disp.ppm"
  local tmp_thumb_ppm="$OUTPUT_DIR/.tmp.${stable}.thumb.ppm"
  local out_disp_jpg="$OUTPUT_DIR/${stable}.jpg"
  local out_disp_webp="$OUTPUT_DIR/${stable}.webp"
  local out_thumb_jpg="$OUTPUT_DIR/${stable}.thumb.jpg"
  local out_thumb_webp="$OUTPUT_DIR/${stable}.thumb.webp"

  # display variant: cover-resize to exact disp_w×disp_h then center crop
  convert "$input" -strip -auto-orient \
    -resize "${disp_w}x${disp_h}^" -gravity center -extent "${disp_w}x${disp_h}" \
    "$tmp_disp_ppm"
  cjpeg -quality "$JPEG_Q" -optimize -progressive "$tmp_disp_ppm" > "$out_disp_jpg"
  cwebp -q "$WEBP_Q" -metadata none -quiet "$tmp_disp_ppm" -o "$out_disp_webp"
  rm -f "$tmp_disp_ppm"

  # thumbnail variant
  convert "$input" -strip -auto-orient \
    -resize "${thumb_w}x${thumb_h}^" -gravity center -extent "${thumb_w}x${thumb_h}" \
    "$tmp_thumb_ppm"
  cjpeg -quality "$JPEG_Q" -optimize -progressive "$tmp_thumb_ppm" > "$out_thumb_jpg"
  cwebp -q "$WEBP_Q" -metadata none -quiet "$tmp_thumb_ppm" -o "$out_thumb_webp"
  rm -f "$tmp_thumb_ppm"

  echo "  OK: $stable (${orientation} ${disp_w}x${disp_h} / ${thumb_w}x${thumb_h})"
}

echo "=> input : $INPUT_DIR"
echo "=> output: $OUTPUT_DIR"
echo

for entry in "${MAPPING[@]}"; do
  IFS='|' read -r stable raw_rel orientation <<< "$entry"
  process_one "$stable" "$raw_rel" "$orientation"
done

echo
echo "=> done. generated files:"
ls -lh "$OUTPUT_DIR" | tail -n +2
