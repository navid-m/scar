#!/usr/bin/env bash

set -e

echo "Checking for Homebrew..."
if ! command -v brew >/dev/null 2>&1; then
    echo "Homebrew not found. Installing Homebrew..."
    /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
else
    echo "Homebrew already installed."
fi

echo "Updating Homebrew..."
brew update

echo "Installing GCC (if not installed)..."
brew install gcc

echo "Installing libraries: libcurl, pcre, jansson..."
brew install curl pcre jansson
