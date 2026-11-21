#!/bin/bash
# Tự nhận diện nền tảng/ABI, tải binary MỚI NHẤT từ GitHub Release về thư mục
# chứa script, verify SHA-256 rồi chạy nó. Không cần gh/jq — chỉ cần curl.
#
#   ./run_release.sh            # tải rồi chạy bot
#   NO_RUN=1 ./run_release.sh   # chỉ tải/verify, không chạy (dùng để test)
#   DEST=/path ./run_release.sh # đổi thư mục chứa binary (.env phải ở đó)
set -eu

REPO="sang765/TMR-Discord-Bot"
API="https://api.github.com/repos/$REPO/releases/latest"
DIR="$(cd "$(dirname "$0")" && pwd)"
DEST="${DEST:-$DIR}"

command -v curl >/dev/null 2>&1 || { echo "Cần curl để tải release" >&2; exit 1; }
mkdir -p "$DEST"

# --- 1) Detect platform + ABI -> tên asset ---
os="$(uname -s)"
arch="$(uname -m)"
android=no
if [ -n "${PREFIX:-}" ] && [ -d "$PREFIX" ]; then android=yes; fi
if [ -f /system/build.prop ]; then android=yes; fi

case "$os" in
  Darwin) platform=macos ;;
  MINGW*|MSYS*|CYGWIN*) platform=windows ;;
  Linux)  platform=linux ;;
  *) echo "Hệ điều hành không hỗ trợ: $os" >&2; exit 1 ;;
esac

case "$arch" in
  aarch64|arm64) abi=arm64 ;;
  armv7l|armv7|armv6l) abi=armv7 ;;
  x86_64|amd64) abi=x86_64 ;;
  i386|i486|i586|i686) abi=x86 ;;
  *) echo "Kiến trúc không hỗ trợ: $arch" >&2; exit 1 ;;
esac

if [ "$platform" = linux ] && [ "$android" = yes ]; then
  platform=termux # Termux là userspace Linux trên kernel Android
fi

case "$platform:$abi" in
  termux:arm64)      asset=tmr-bot-termux-arm64-v8a ;;
  termux:armv7)      asset=tmr-bot-termux-armeabi-v7a ;;
  termux:x86)        asset=tmr-bot-termux-x86 ;;
  termux:x86_64)     asset=tmr-bot-termux-x86_64 ;;
  linux:arm64)       asset=tmr-bot-linux-arm64 ;;
  linux:x86_64)      asset=tmr-bot-linux-amd64 ;;
  macos:arm64)       asset=tmr-bot-macos-arm64 ;;
  macos:x86_64)      asset=tmr-bot-macos-amd64 ;;
  windows:x86_64)    asset=tmr-bot-windows-amd64.exe ;;
  *)
    echo "Không có binary cho $platform/$abi (arch=$arch)" >&2
    echo "Các asset có sẵn xem tại https://github.com/$REPO/releases" >&2
    exit 1 ;;
esac

echo "Nền tảng: $os ($arch) -> $asset"

# --- 2) Lấy metadata release mới nhất ---
if [ -n "${GH_TOKEN:-}" ]; then
  json="$(curl -fsSL -H "Authorization: Bearer $GH_TOKEN" "$API")"
else
  json="$(curl -fsSL "$API")"
fi
tag="$(printf '%s\n' "$json" | grep -o '"tag_name": *"[^"]*"' | head -1 | sed -E 's/.*"([^"]+)"/\1/')"
if [ -z "$tag" ]; then
  echo "Không đọc được tag release mới nhất" >&2; exit 1
fi
echo "Release mới nhất: $tag"

# Tìm browser_download_url của một asset theo tên chính xác.
asset_url() {
  printf '%s\n' "$json" \
    | grep -o '"browser_download_url": *"[^"]*"' \
    | grep -F "$1\"" \
    | head -1 \
    | sed -E 's/.*"([^"]+)"/\1/'
}

url="$(asset_url "$asset")"
sum_url="$(asset_url "$asset.sha256")"
if [ -z "$url" ] || [ -z "$sum_url" ]; then
  echo "Không tìm thấy asset $asset trong release $tag" >&2; exit 1
fi

# --- 3) Tải về file tạm + verify checksum ---
tmp="$DEST/$asset.download"
sum_file="$DEST/$asset.sha256"
curl -fsSL "$url" -o "$tmp"
curl -fsSL "$sum_url" -o "$sum_file"

expected="$(awk 'NR==1 {print $1}' "$sum_file")"
if command -v sha256sum >/dev/null 2>&1; then
  actual="$(sha256sum "$tmp" | awk '{print $1}')"
elif command -v shasum >/dev/null 2>&1; then
  actual="$(shasum -a 256 "$tmp" | awk '{print $1}')"
else
  echo "Không có sha256sum/shasum — bỏ qua verify checksum" >&2
  actual="$expected"
fi
if [ "$actual" != "$expected" ]; then
  echo "Checksum KHÔNG khớp ($actual != $expected)" >&2
  rm -f "$tmp"
  exit 1
fi

target="$DEST/$asset"
mv -f "$tmp" "$target"
chmod +x "$target"
echo "Đã tải và verify: $target ($tag)"

if [ "${NO_RUN:-0}" = 1 ]; then
  echo "NO_RUN=1 — bỏ qua khởi động bot"
  exit 0
fi

cd "$DIR" || exit 1
exec "$target" "$@"
