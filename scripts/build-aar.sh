#!/usr/bin/env bash
#
# build-aar.sh — compile Volt's shared Go core to an Android library (.aar).
#
# This script runs `gomobile bind` against the gomobile-bindable facade in
# ./mobile (package `mobile`, type `Bridge`; see mobile/bind.go), producing
# `volt.aar`. The Capacitor Android project's custom plugin (VoltBridge) links
# this AAR and forwards JS bridge calls to `mobile.Bridge`, so Android requests
# run through the exact same Go httpcore/store logic as the desktop build
# (Req 15.1), bypass WebView CORS (Req 15.2), preserve engine errors (Req 15.4),
# and persist on-device (Req 15.5).
#
# This is a MANUAL / CI step. It is NOT part of `npm run build`/`check`/`test`
# and requires a provisioned environment that the web/desktop build does not:
#
#   Prerequisites
#   -------------
#   1. Go (the version in go.mod) on PATH.
#   2. The Android SDK + NDK, with environment variables set:
#        export ANDROID_HOME=/path/to/Android/sdk          # or ANDROID_SDK_ROOT
#        export ANDROID_NDK_HOME=$ANDROID_HOME/ndk/<version>
#      gomobile requires the NDK to cross-compile the Go runtime for Android.
#   3. The gomobile toolchain installed and initialised:
#        go install golang.org/x/mobile/cmd/gomobile@latest
#        go install golang.org/x/mobile/cmd/gobind@latest
#        export PATH="$PATH:$(go env GOPATH)/bin"
#        gomobile init        # one-time; downloads the OpenJDK/NDK glue
#
# Usage
# -----
#   scripts/build-aar.sh [--api <androidapi>] [--out <dir>]
#
#   --api   Android API level to target (default: 24). Must be >= the project's
#           minSdk.
#   --out   Output directory for volt.aar. Default places the AAR where the
#           Capacitor Android app can consume it:
#             frontend/android/app/libs/volt.aar
#           (the android/ project is generated on demand by `npx cap add
#           android`; the script creates the libs/ dir if missing).
#
# Example
# -------
#   ANDROID_HOME=$HOME/Android/Sdk \
#   ANDROID_NDK_HOME=$HOME/Android/Sdk/ndk/26.1.10909125 \
#   scripts/build-aar.sh --api 24
#
set -euo pipefail

# Resolve the Go module root (this script lives in <module>/scripts).
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MODULE_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

ANDROID_API=24
OUT_DIR="${MODULE_ROOT}/frontend/android/app/libs"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --api)
      ANDROID_API="${2:?--api requires a value}"; shift 2 ;;
    --out)
      OUT_DIR="${2:?--out requires a value}"; shift 2 ;;
    -h|--help)
      grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *)
      echo "build-aar.sh: unknown argument: $1" >&2; exit 2 ;;
  esac
done

OUT_AAR="${OUT_DIR}/volt.aar"

echo "==> Volt gomobile AAR build"
echo "    module root : ${MODULE_ROOT}"
echo "    facade pkg  : ./mobile (mobile.Bridge)"
echo "    android api : ${ANDROID_API}"
echo "    output      : ${OUT_AAR}"

# --- Preflight checks ------------------------------------------------------
command -v go >/dev/null 2>&1 || { echo "error: 'go' not found on PATH" >&2; exit 1; }

if ! command -v gomobile >/dev/null 2>&1; then
  echo "error: 'gomobile' not found on PATH." >&2
  echo "       Install it with:" >&2
  echo "         go install golang.org/x/mobile/cmd/gomobile@latest" >&2
  echo "         go install golang.org/x/mobile/cmd/gobind@latest" >&2
  echo "         export PATH=\"\$PATH:\$(go env GOPATH)/bin\"" >&2
  exit 1
fi

if [[ -z "${ANDROID_HOME:-}" && -z "${ANDROID_SDK_ROOT:-}" ]]; then
  echo "error: ANDROID_HOME (or ANDROID_SDK_ROOT) is not set." >&2
  echo "       gomobile needs the Android SDK + NDK to cross-compile." >&2
  exit 1
fi

if [[ -z "${ANDROID_NDK_HOME:-}" ]]; then
  echo "warning: ANDROID_NDK_HOME is not set; gomobile will try to locate the" >&2
  echo "         NDK under the SDK. If the build fails, set it explicitly." >&2
fi

# Ensure gomobile is initialised (idempotent; safe to re-run).
echo "==> gomobile init (one-time toolchain setup; idempotent)"
gomobile init

# --- Build -----------------------------------------------------------------
mkdir -p "${OUT_DIR}"

echo "==> gomobile bind"
cd "${MODULE_ROOT}"
gomobile bind \
  -target=android \
  -androidapi "${ANDROID_API}" \
  -javapkg=dev.volt.apiclient.bridge \
  -o "${OUT_AAR}" \
  ./mobile

echo "==> Done. Wrote ${OUT_AAR}"
echo "    The Capacitor VoltBridge plugin (Kotlin) imports this AAR's"
echo "    'mobile' package (dev.volt.apiclient.bridge.mobile.Bridge) and forwards"
echo "    JS calls to it. See frontend/native-android/VoltBridgePlugin.kt and"
echo "    frontend/native-android/README.md for the wiring steps."
