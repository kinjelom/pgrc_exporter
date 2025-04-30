#!/bin/bash

VERSION_LABEL="0.1.0"

DIST_DIR=".dist"

declare -a os_array=("linux") # "darwin" "windows")
declare -a arch_array=("amd64") # "arm64")

mkdir -p "$DIST_DIR"

for OS in "${os_array[@]}"; do
    for ARCH in "${arch_array[@]}"; do
        DIST_PATH="$DIST_DIR/pgrc_exporter-v$VERSION_LABEL-$OS-$ARCH"
        echo "build $DIST_PATH"
        if GOOS=$OS GOARCH=$ARCH go build -o "$DIST_PATH" -ldflags="-X 'main.ProgramVersion=${VERSION_LABEL}'" >> "$DIST_PATH.log"; then
            sha256sum "$DIST_PATH"  | awk '{print $1}' > "$DIST_PATH.sum"
        fi
    done
done
