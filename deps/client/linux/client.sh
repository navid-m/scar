#!/usr/bin/env bash
set -e

echo "Updating package lists..."
if command -v apt-get >/dev/null 2>&1; then
    PKG_MANAGER="apt-get"
elif command -v yum >/dev/null 2>&1; then
    PKG_MANAGER="yum"
elif command -v dnf >/dev/null 2>&1; then
    PKG_MANAGER="dnf"
else
    echo "No supported package manager found (apt, yum, dnf)"
    exit 1
fi

echo "Installing build-essential / GCC..."
if [ "$PKG_MANAGER" = "apt-get" ]; then
    sudo apt-get update
    sudo apt-get install -y build-essential curl libcurl4-openssl-dev libpcre3-dev libjansson-dev
elif [ "$PKG_MANAGER" = "yum" ] || [ "$PKG_MANAGER" = "dnf" ]; then
    sudo $PKG_MANAGER install -y gcc gcc-c++ make curl libcurl-devel pcre-devel jansson-devel
fi
