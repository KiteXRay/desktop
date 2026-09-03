#!/usr/bin/env bash
set -e

echo "Building for Linux (amd64)..."
wails build -platform linux/amd64 -tags webkit2_41

echo "Building for Linux (arm64)..."
wails build -platform linux/arm64 -tags webkit2_41

echo "Building for macOS (universal)..."
wails build -platform darwin/universal

echo "Build complete! Output files are located in build/bin"
