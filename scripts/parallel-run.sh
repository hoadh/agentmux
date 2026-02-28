#!/bin/sh
# parallel-run.sh — Run multiple agentmux pipelines in parallel
# with isolated output directories, job limiting, and summary reporting.
set -e

PROG="$(basename "$0")"
CONFIG="agentmux.yaml"
MANIFEST=""
MAX_JOBS=4
BASE_OUT=".agentmux-out"

# --- Help ---

usage() {
    cat <<EOF
Usage: $PROG -c <config> [options] [-- --var k=v ...] [-- --var k=v ...]

Run multiple agentmux pipelines in parallel with isolated output directories.

Options:
  -c <config>   Config file (default: agentmux.yaml)
  -f <manifest> Manifest file: one pipeline's args per line
  -j <N>        Max parallel jobs (default: 4)
  -o <dir>      Base output directory (default: .agentmux-out)
  -h            Show this help

Inline mode (-- separates each pipeline):
  $PROG -c pipeline.yaml \\
    -- --var topic="dog grooming" \\
    -- --var topic="cat nutrition"

Manifest mode (one line per pipeline):
  echo '--var topic="dog grooming"' > pipelines.txt
  echo '--var topic="cat nutrition"' >> pipelines.txt
  $PROG -c pipeline.yaml -f pipelines.txt
EOF
    exit 0
}

# --- Parse flags (stop at first --) ---

while [ $# -gt 0 ]; do
    case "$1" in
        -c) CONFIG="$2"; shift 2 ;;
        -f) MANIFEST="$2"; shift 2 ;;
        -j) MAX_JOBS="$2"; shift 2 ;;
        -o) BASE_OUT="$2"; shift 2 ;;
        -h|--help) usage ;;
        --) shift; break ;;
        -*) echo "Error: unknown option: $1" >&2; exit 1 ;;
        *)  echo "Error: unexpected argument: $1" >&2; exit 1 ;;
    esac
done

# --- Validate MAX_JOBS ---

case "$MAX_JOBS" in
    ''|*[!0-9]*) echo "Error: -j requires a positive integer, got: $MAX_JOBS" >&2; exit 1 ;;
esac
if [ "$MAX_JOBS" -lt 1 ]; then
    echo "Error: -j must be at least 1, got: $MAX_JOBS" >&2
    exit 1
fi

# --- Build pipeline list into a temp file (avoids pipe-subshell issues) ---

TMPFILE=$(mktemp)
PIPELINE_COUNT=0

if [ -n "$MANIFEST" ]; then
    if [ ! -f "$MANIFEST" ]; then
        echo "Error: manifest file not found: $MANIFEST" >&2
        exit 1
    fi
    while IFS= read -r line || [ -n "$line" ]; do
        line="$(echo "$line" | sed 's/^[[:space:]]*//' | sed 's/[[:space:]]*$//')"
        case "$line" in "#"*|"") continue ;; esac
        PIPELINE_COUNT=$((PIPELINE_COUNT + 1))
        echo "$line" >> "$TMPFILE"
    done < "$MANIFEST"
else
    # Inline mode: remaining args split by -- separators
    current=""
    while [ $# -gt 0 ]; do
        if [ "$1" = "--" ]; then
            if [ -n "$current" ]; then
                PIPELINE_COUNT=$((PIPELINE_COUNT + 1))
                echo "$current" >> "$TMPFILE"
            fi
            current=""
        else
            current="${current}${current:+ }$1"
        fi
        shift
    done
    if [ -n "$current" ]; then
        PIPELINE_COUNT=$((PIPELINE_COUNT + 1))
        echo "$current" >> "$TMPFILE"
    fi
fi

if [ "$PIPELINE_COUNT" -eq 0 ]; then
    echo "Error: no pipelines specified. Use -- separators or -f manifest." >&2
    echo "Run '$PROG -h' for help." >&2
    exit 1
fi

if [ ! -f "$CONFIG" ]; then
    echo "Error: config file not found: $CONFIG" >&2
    exit 1
fi

mkdir -p "$BASE_OUT"

# --- Cleanup: remove temp file and kill children on exit/interrupt ---

# shellcheck disable=SC2329
cleanup() {
    rm -f "$TMPFILE"
    # Kill any remaining background children
    # shellcheck disable=SC2086
    for p in $PIDS; do
        kill "$p" 2>/dev/null || true
    done
}
trap cleanup EXIT INT TERM

echo "=== Parallel Pipeline Runner ==="
echo ""
echo "  Config:     $CONFIG"
echo "  Pipelines:  $PIPELINE_COUNT"
echo "  Max jobs:   $MAX_JOBS"
echo "  Output dir: $BASE_OUT"
echo ""

# --- Derive slug from pipeline args ---

derive_slug() {
    index="$2"
    # Extract first --var value for the slug.
    # Handles both quoted (manifest) and unquoted (inline) forms:
    #   --var topic="dog grooming" --output-dir ...
    #   --var topic=dog grooming --output-dir ...
    # Try quoted value first, then unquoted (up to next -- flag or end of line)
    slug=$(echo "$1" | sed -n 's/.*--var [^=]*="\([^"]*\)".*/\1/p' | head -1)
    if [ -z "$slug" ]; then
        slug=$(echo "$1" | sed -n 's/.*--var [^=]*=\([^-][^-]*\).*/\1/p' | head -1)
        slug=$(echo "$slug" | sed 's/[[:space:]]*$//')
    fi
    if [ -n "$slug" ]; then
        echo "$slug" | tr '[:upper:]' '[:lower:]' | sed 's/[^a-z0-9]/-/g' | sed 's/--*/-/g' | sed 's/^-//' | sed 's/-$//'
    else
        echo "pipeline-$index"
    fi
}

# --- Count running PIDs (POSIX-compatible) ---

count_running() {
    count=0
    # shellcheck disable=SC2086
    for p in $PIDS; do
        if kill -0 "$p" 2>/dev/null; then
            count=$((count + 1))
        fi
    done
    echo "$count"
}

# --- Launch pipelines ---

PIDS=""
SLUGS=""
START_TIMES=""
END_TIMES=""
INDEX=0

echo "--- Launching pipelines ---"
echo ""

while IFS= read -r line; do
    [ -z "$line" ] && continue
    INDEX=$((INDEX + 1))

    slug=$(derive_slug "$line" "$INDEX")

    # Add --output-dir if not already specified
    case "$line" in
        *--output-dir*) out_args="$line" ;;
        *) out_args="$line --output-dir ${BASE_OUT}/${slug}" ;;
    esac

    # Job limiting: poll until a slot opens
    while [ "$(count_running)" -ge "$MAX_JOBS" ]; do
        sleep 1
    done

    start_time=$(date +%s)

    # Launch without eval to prevent shell injection.
    # Globbing disabled; word splitting on $out_args is intentional.
    set -f
    # shellcheck disable=SC2086
    agentmux run -c "$CONFIG" --headless $out_args &
    set +f
    pid=$!

    PIDS="${PIDS}${pid} "
    SLUGS="${SLUGS}${slug} "
    START_TIMES="${START_TIMES}${start_time} "

    echo "  [#$INDEX] Started: $slug (PID $pid)"
done < "$TMPFILE"

echo ""
echo "--- Waiting for all pipelines to complete ---"
echo ""

# --- Wait and collect exit codes + per-pipeline end times ---
# Disable set -e so we can capture non-zero exit codes from wait
set +e

EXIT_CODES=""
# shellcheck disable=SC2086
for pid in $PIDS; do
    wait "$pid" 2>/dev/null
    EXIT_CODES="${EXIT_CODES}$? "
    END_TIMES="${END_TIMES}$(date +%s) "
done

set -e

# --- Print summary ---

echo ""
echo "=== Parallel Run Summary ==="
echo ""
printf "  %-4s %-24s %-10s %-10s %s\n" "#" "Pipeline" "Status" "Duration" "Output"

passed=0
failed=0

# Intentional word splitting to expand space-separated lists
# shellcheck disable=SC2086
set -- $SLUGS
slugs_list="$*"
# shellcheck disable=SC2086
set -- $EXIT_CODES
codes_list="$*"
# shellcheck disable=SC2086
set -- $START_TIMES
times_list="$*"
# shellcheck disable=SC2086
set -- $END_TIMES
etimes_list="$*"

i=1
for slug in $slugs_list; do
    code=$(echo "$codes_list" | cut -d' ' -f"$i")
    stime=$(echo "$times_list" | cut -d' ' -f"$i")
    etime=$(echo "$etimes_list" | cut -d' ' -f"$i")
    duration=$((etime - stime))

    if [ "$code" -eq 0 ]; then
        status_icon="✓ pass"
        passed=$((passed + 1))
    else
        status_icon="✗ fail"
        failed=$((failed + 1))
    fi

    out_dir="${BASE_OUT}/${slug}/"
    printf "  %-4s %-24s %-10s %-10s %s\n" "$i" "$slug" "$status_icon" "${duration}s" "$out_dir"
    i=$((i + 1))
done

total=$((passed + failed))
echo ""
echo "Results: $passed passed, $failed failed ($total total)"
echo ""

if [ "$failed" -gt 0 ]; then
    exit 1
fi
exit 0
