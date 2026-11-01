#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

version="$(go list -m -f '{{.Version}}' github.com/metruzanca/gothic)"
if [ -z "$version" ]; then
	echo "cannot find the pinned gothic version in go.mod" >&2
	exit 1
fi

src="${GOTHIC_SRC:-}"
if [ -z "$src" ]; then
	cache="${XDG_CACHE_HOME:-$HOME/.cache}/gothic-docs"
	src="${cache}/gothic-${version}"
	if [ ! -d "$src/.git" ]; then
		rm -rf "$src"
		mkdir -p "$cache"
		echo "cloning gothic ${version} into ${src}"
		git clone --depth 1 --branch "$version" https://github.com/metruzanca/gothic.git "$src"
	fi
fi

if ! git -C "$src" rev-parse --verify --quiet "${version}^{commit}" >/dev/null; then
	git -C "$src" fetch --tags --quiet --force origin "+refs/tags/${version}:refs/tags/${version}" || true
fi
if ! git -C "$src" rev-parse --verify --quiet "${version}^{commit}" >/dev/null; then
	echo "gothic ${version} not found in ${src}" >&2
	exit 1
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
git -C "$src" archive "$version" README.md docs | tar -x -C "$tmp"

rm -rf content
mkdir -p content
cp "$tmp/README.md" content/README.md
cp -R "$tmp/docs" content/docs

echo "synced gothic ${version} markdown into content/"
