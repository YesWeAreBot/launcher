#!/bin/sh
set -eu

REPO='YesWeAreBot/launcher'
CHANNEL='nightly'
INSTALL_DIR="${HOME:-}/.local/bin"

usage() {
    cat <<'EOF'
Usage: install.sh [--channel CHANNEL] [--install-dir PATH]

Download and install the YesImBot Launcher from a GitHub Release.
EOF
}

fail() {
    printf 'error: %s\n' "$1" >&2
    exit 1
}

while [ "$#" -gt 0 ]; do
    case "$1" in
        --channel)
            [ "$#" -ge 2 ] || fail '--channel requires a value'
            CHANNEL=$2
            shift 2
            ;;
        --install-dir)
            [ "$#" -ge 2 ] || fail '--install-dir requires a value'
            INSTALL_DIR=$2
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            fail "unknown argument: $1"
            ;;
    esac
done

[ -n "${HOME:-}" ] || fail 'HOME is not set'
case "$CHANNEL" in
    ''|*[!A-Za-z0-9._-]*)
        fail "invalid channel: $CHANNEL"
        ;;
esac

if [ -n "${GITHUB_MIRROR:-}" ]; then
    MIRROR=$GITHUB_MIRROR
else
    MIRROR=https://github.com
fi
while :; do
    case "$MIRROR" in
        */) MIRROR=${MIRROR%/} ;;
        *) break ;;
    esac
done
case "$MIRROR" in
    http://*|https://*) ;;
    *) fail "invalid GITHUB_MIRROR: $MIRROR (expected an http(s) URL)" ;;
esac

OS=$(uname -s 2>/dev/null) || fail 'cannot detect operating system'
MACHINE=$(uname -m 2>/dev/null) || fail 'cannot detect CPU architecture'
case "$OS:$MACHINE" in
    Linux:x86_64|Linux:amd64)
        PLATFORM=linux
        ARCH=amd64
        ;;
    Linux:aarch64|Linux:arm64)
        PLATFORM=linux
        ARCH=arm64
        ;;
    Darwin:x86_64|Darwin:amd64)
        PLATFORM=darwin
        ARCH=amd64
        ;;
    Darwin:arm64|Darwin:aarch64)
        PLATFORM=darwin
        ARCH=arm64
        ;;
    *)
        fail "unsupported platform or architecture: $OS $MACHINE"
        ;;
esac

[ -n "$INSTALL_DIR" ] || fail 'install directory must not be empty'
mkdir -p "$INSTALL_DIR" || fail "cannot create install directory: $INSTALL_DIR"
INSTALL_DIR=$(CDPATH= cd -- "$INSTALL_DIR" && pwd -P) || fail "cannot resolve install directory: $INSTALL_DIR"
TARGET="$INSTALL_DIR/yesimbot-cli"
ASSET="yesimbot-cli-$PLATFORM-$ARCH"
URL="$MIRROR/$REPO/releases/download/$CHANNEL/$ASSET"

if command -v curl >/dev/null 2>&1; then
    DOWNLOADER=curl
elif command -v wget >/dev/null 2>&1; then
    DOWNLOADER=wget
else
    fail 'curl or wget is required'
fi

TMP_FILE=$(mktemp "$INSTALL_DIR/.yesimbot-cli.XXXXXX") || fail "cannot create temporary file in $INSTALL_DIR"
cleanup() {
    rm -f "$TMP_FILE"
}
trap cleanup EXIT HUP INT TERM

printf 'Downloading %s (%s/%s) from %s\n' "$CHANNEL" "$PLATFORM" "$ARCH" "$MIRROR"
case "$DOWNLOADER" in
    curl)
        curl --fail --location --silent --show-error --retry 3 --connect-timeout 15 --max-time 60 --output "$TMP_FILE" "$URL" || fail 'download failed'
        ;;
    wget)
        wget --quiet --timeout=30 --tries=3 --output-document="$TMP_FILE" "$URL" || fail 'download failed'
        ;;
esac
chmod 755 "$TMP_FILE" || fail "cannot make $TARGET executable"
mv -f "$TMP_FILE" "$TARGET" || fail "cannot install $TARGET"

path_has_dir() {
    case ":${PATH:-}:" in
        *":$INSTALL_DIR:"*) return 0 ;;
    esac
    return 1
}

path_file_has_dir() {
    [ -f "$1" ] || return 1
    while IFS= read -r line || [ -n "$line" ]; do
        case "$line" in
            *"YesImBot Launcher installer"*|*"$INSTALL_DIR"*) return 0 ;;
        esac
    done < "$1"
    return 1
}

if path_has_dir; then
    PATH_STATUS='PATH already contains the install directory'
else
    SHELL_NAME=${SHELL##*/}
    case "$SHELL_NAME" in
        bash) PROFILE="$HOME/.bashrc" ;;
        zsh) PROFILE="$HOME/.zshrc" ;;
        *) PROFILE="$HOME/.profile" ;;
    esac
    if path_file_has_dir "$PROFILE"; then
        PATH_STATUS="PATH entry already configured in $PROFILE"
    else
        QUOTED_DIR=$(printf '%s' "$INSTALL_DIR" | sed "s/'/'\\''/g; 1s/^/'/; \$s/\$/'/")
        {
            printf '\n# Added by YesImBot Launcher installer\n'
            printf 'export PATH=%s:"$PATH"\n' "$QUOTED_DIR"
        } >> "$PROFILE" || fail "cannot update PATH in $PROFILE"
        PATH_STATUS="added $INSTALL_DIR to $PROFILE"
    fi
fi

printf 'Installed %s to %s\n' "$CHANNEL" "$TARGET"
printf '%s\n' "$PATH_STATUS"
printf '\n--- yesimbot-cli --help ---\n'
"$TARGET" --help
