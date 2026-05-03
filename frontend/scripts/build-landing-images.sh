#!/usr/bin/env bash
#
# m2-design-refresh STOP β-2c: design/usephot/ raw PNG →
# frontend/public/img/landing/ の WebP/JPEG variants を生成する pipeline。
#
# 入力: repo-root の design/usephot/ にある 7 枚の VRChat raw PNG
#       (`.gitignore` で除外済、user-local 入力。git に絶対入れない)
# 出力: repo-root の frontend/public/img/landing/ に 7 stable name × 2 format =
#       14 file (hero / mock-cover / sample-01..05 × .webp/.jpg)
#       これらは git に含める static asset (LP / MockBook で参照)
#
# 使い方 (repo root から):
#   bash frontend/scripts/build-landing-images.sh
#
# 必須 tool (PATH 上):
#   - cwebp     (libwebp; PNG/JPEG → WebP)
#   - cjpeg     (libjpeg-turbo; PPM → JPEG)
#   - convert   (ImageMagick 6+; PNG → PPM 中間 / リサイズ)
#   - identify  (ImageMagick 6+; 寸法確認)
# 不在の場合は明確に fail する。
# install (Ubuntu): sudo apt-get install -y webp libjpeg-turbo-progs imagemagick
#
# raw → stable name の確定 mapping は本 script 内の MAPPING 配列で管理 (Q-2c-1 確定)。
# raw filename は React component 側に書かない (mapping は本 script 内に閉じ込める)。
#
# 生成寸法:
#   - hero      : 1600px wide  (LP hero 表示、横長 16:9)
#   - mock-cover: 720px  wide  (MockBook 左 cover、縦長 9:16)
#   - sample-*  : 640px  wide  (LP sample strip、Mobile 1:1 / PC 4:3)
# 品質:
#   - WebP q70-72  (subjective threshold、ファイルサイズ抑制)
#   - JPEG q78     (WebP 非対応 browser fallback)
# 全 variant で `-strip` / `-metadata none` で EXIF / XMP 等メタデータを除去する。
#
# 設計参照:
#   - docs/plan/m2-design-refresh-stop-beta-2-plan.md §3 (β-2c)

set -euo pipefail

# 1) repo root を script 位置から決定 (frontend/scripts/.. /..)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
INPUT_DIR="$REPO_ROOT/design/usephot"
OUTPUT_DIR="$REPO_ROOT/frontend/public/img/landing"

# 2) 必須 tool の存在確認 (不在で即 fail、install 手順を提示)
for tool in cwebp cjpeg convert identify; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo "ERROR: required tool '$tool' not found in PATH" >&2
    echo "       install: sudo apt-get install -y webp libjpeg-turbo-progs imagemagick" >&2
    exit 1
  fi
done

# 3) 入力 dir 確認
if [ ! -d "$INPUT_DIR" ]; then
  echo "ERROR: input directory not found: $INPUT_DIR" >&2
  echo "       design/usephot/ に raw PNG 7 枚を user-local で配置してから実行してください" >&2
  echo "       (raw PNG は .gitignore 除外済、git に commit しない)" >&2
  exit 1
fi

# 4) raw → stable name mapping (β-2c Q-2c-1 確定。raw filename はここに閉じ込める)
#    形式: "STABLE_NAME=RAW_BASENAME"
MAPPING=(
  "hero=VRChat_2026-03-13_14-03-24.992_3840x2160.png"
  "mock-cover=82E37915-2A9D-4B69-B890-5427DB9BFC71VRChat_2026-04-06_23-37-30.108_2160x3840.png"
  "sample-01=VRChat_2026-03-22_23-48-33.324_2160x3840.png"
  "sample-02=VRChat_2026-04-14_15-59-36.459_3840x2160.png"
  "sample-03=VRChat_2026-03-03_22-55-45.806_2160x3840.png"
  "sample-04=VRChat_2026-03-27_00-07-06.943_2160x3840.png"
  "sample-05=VRChat_2026-03-27_00-02-57.153_2160x3840.png"
)

# 5) stable name 別の出力寸法・品質
declare -A WIDTH_SPEC=(
  [hero]=1600
  [mock-cover]=720
  [sample-01]=640
  [sample-02]=640
  [sample-03]=640
  [sample-04]=640
  [sample-05]=640
)
declare -A WEBP_QUALITY=(
  [hero]=72
  [mock-cover]=70
  [sample-01]=70
  [sample-02]=70
  [sample-03]=70
  [sample-04]=70
  [sample-05]=70
)
declare -A JPEG_QUALITY=(
  [hero]=78
  [mock-cover]=78
  [sample-01]=78
  [sample-02]=78
  [sample-03]=78
  [sample-04]=78
  [sample-05]=78
)

# 6) 出力 dir 準備
mkdir -p "$OUTPUT_DIR"

# 7) 中間 PPM 用の tmp dir (cleanup 保証)
TMP_DIR=$(mktemp -d -t beta-2c-XXXXXX)
trap 'rm -rf "$TMP_DIR"' EXIT

echo "build-landing-images.sh: STOP β-2c image asset pipeline"
echo "  INPUT_DIR  : $INPUT_DIR"
echo "  OUTPUT_DIR : $OUTPUT_DIR"
echo "  TMP_DIR    : $TMP_DIR (auto-cleanup on EXIT)"
echo ""

# 8) mapping を順次処理
for entry in "${MAPPING[@]}"; do
  stable="${entry%%=*}"
  raw="${entry#*=}"
  src="$INPUT_DIR/$raw"

  if [ ! -f "$src" ]; then
    echo "ERROR: input PNG not found for stable=${stable}: $src" >&2
    exit 1
  fi

  width="${WIDTH_SPEC[$stable]}"
  wq="${WEBP_QUALITY[$stable]}"
  jq="${JPEG_QUALITY[$stable]}"

  webp_out="$OUTPUT_DIR/$stable.webp"
  jpg_out="$OUTPUT_DIR/$stable.jpg"
  ppm_tmp="$TMP_DIR/$stable.ppm"

  printf '[%-10s] src=%s (%dpx wide / WebP q%d / JPEG q%d)\n' \
    "$stable" "$raw" "$width" "$wq" "$jq"

  # WebP: cwebp は PNG 直接読込み可。-resize WIDTH 0 で縦比率維持。-metadata none で EXIF 除去。
  cwebp -quiet -q "$wq" -resize "$width" 0 -metadata none "$src" -o "$webp_out"

  # JPEG: cjpeg は PNG 不対応のため、convert で PPM 中間 → cjpeg で encode。
  #       -strip でメタデータ除去、-resize は width 制限 (height 比率維持)。
  convert "$src" -resize "${width}x" -strip "$ppm_tmp"
  cjpeg -quality "$jq" -optimize -progressive -outfile "$jpg_out" "$ppm_tmp"

  rm -f "$ppm_tmp"
done

echo ""
echo "generated assets:"
du -h "$OUTPUT_DIR"/*.{webp,jpg} 2>/dev/null | sort -k2

echo ""
echo "total (webp + jpeg):"
du -ch "$OUTPUT_DIR"/*.{webp,jpg} 2>/dev/null | tail -1

# 9) raw PNG が出力 dir に混入していないこと (誤コピー防止 guard)
echo ""
echo "guard: raw PNG must not be present in OUTPUT_DIR"
if find "$OUTPUT_DIR" -maxdepth 1 -type f -name '*.png' | grep -q .; then
  echo "ERROR: raw PNG が OUTPUT_DIR に混入" >&2
  find "$OUTPUT_DIR" -maxdepth 1 -type f -name '*.png' >&2
  exit 1
fi
echo "  OK (no .png in $OUTPUT_DIR)"

echo ""
echo "build-landing-images.sh: done"
