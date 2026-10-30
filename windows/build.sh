#!/usr/bin/env bash
# 編出 Windows 版 DesktopPet.exe(可以在 macOS / Linux 上交叉編譯,不需要 Windows)
#   用法:windows/build.sh            → windows/dist/DesktopPet.exe(64 位元)
#        GOARCH=arm64 windows/build.sh → ARM 版 Windows
set -euo pipefail
cd "$(dirname "$0")"

# 素材跟 macOS 版共用一份:複製進來,編進 exe
rm -rf pet
cp -R ../Sources/DesktopPet/Resources/Pet pet

go test ./...
mkdir -p dist
GOOS=windows GOARCH="${GOARCH:-amd64}" CGO_ENABLED=0 \
	go build -trimpath -ldflags "-H windowsgui -s -w" -o dist/DesktopPet.exe .
echo "dist/DesktopPet.exe"
