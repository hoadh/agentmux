#!/bin/sh
set -e

PROG="$(basename "$0")"

usage() {
    echo "Usage: $PROG <executable> [--name <name>]"
    echo ""
    echo "Install an executable to a system path."
    echo ""
    echo "Options:"
    echo "  --name <name>  Override the installed binary name"
    echo "  --help         Show this help message"
    exit 1
}

# --- Parse CLI arguments ---

SOURCE=""
BIN_NAME=""

while [ $# -gt 0 ]; do
    case "$1" in
        --name)
            [ -z "${2:-}" ] && { echo "Error: --name requires a value"; exit 1; }
            BIN_NAME="$2"
            shift 2
            ;;
        --help|-h)
            usage
            ;;
        -*)
            echo "Error: unknown option: $1"
            exit 1
            ;;
        *)
            if [ -z "$SOURCE" ]; then
                SOURCE="$1"
            else
                echo "Error: unexpected argument: $1"
                exit 1
            fi
            shift
            ;;
    esac
done

if [ -z "$SOURCE" ]; then
    usage
fi

BIN_NAME="${BIN_NAME:-$(basename "$SOURCE")}"

# --- Validate source file ---

if [ ! -f "$SOURCE" ]; then
    echo "Error: file not found: $SOURCE"
    exit 1
fi

if [ ! -x "$SOURCE" ]; then
    printf "Warning: %s is not executable. Make it executable? [y/N] " "$SOURCE"
    read -r answer
    case "$answer" in
        [yY]*) chmod +x "$SOURCE" ;;
        *) echo "Aborted."; exit 1 ;;
    esac
fi

# --- Destination menu ---

echo ""
echo "Where would you like to install '$BIN_NAME'?"
echo ""
echo "  1) /usr/local/bin  (system-wide, may need sudo)"
echo "  2) ~/.local/bin    (user-only)"
echo "  3) Custom path"
echo ""
printf "Choice [1-3]: "
read -r choice

case "$choice" in
    1) TARGET_DIR="/usr/local/bin" ;;
    2) TARGET_DIR="$HOME/.local/bin" ;;
    3)
        printf "Enter target directory: "
        read -r TARGET_DIR
        ;;
    *) echo "Invalid choice."; exit 1 ;;
esac

if [ -z "$TARGET_DIR" ]; then
    echo "Error: target directory cannot be empty"
    exit 1
fi

# --- Create target directory if missing ---

if [ ! -d "$TARGET_DIR" ]; then
    echo "Creating directory: $TARGET_DIR"
    mkdir -p "$TARGET_DIR" 2>/dev/null || sudo mkdir -p "$TARGET_DIR"
fi

# --- Install ---

TARGET="$TARGET_DIR/$BIN_NAME"

if [ -w "$TARGET_DIR" ]; then
    install -m 755 "$SOURCE" "$TARGET"
else
    echo "Elevated permissions required for $TARGET_DIR"
    sudo install -m 755 "$SOURCE" "$TARGET"
fi

# --- Verify ---

if [ -x "$TARGET" ]; then
    echo "Successfully installed: $TARGET"
else
    echo "Error: installation failed"
    exit 1
fi

# --- PATH hint ---

case ":$PATH:" in
    *":$TARGET_DIR:"*) ;;
    *)
        echo ""
        echo "Note: $TARGET_DIR is not in your PATH."
        echo "Add it with:"
        echo ""
        echo "  export PATH=\"$TARGET_DIR:\$PATH\""
        echo ""
        echo "Add the above line to your ~/.bashrc, ~/.zshrc, or ~/.profile to persist."
        ;;
esac
