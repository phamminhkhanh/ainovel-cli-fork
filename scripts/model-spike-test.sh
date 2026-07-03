#!/usr/bin/env bash
set -euo pipefail

# Reusable model-spike test harness for ainovel-cli.
# Runs a short headless spike and collects cost/duration/rewrite metrics.
#
# NOTE: This script is Unix-only (bash, date, kill, find, python3). On Windows,
# run it inside Git Bash / WSL, or adapt the process-management commands.
#
# Usage:
#   ./scripts/model-spike-test.sh -m "google/gemini-2.5-flash" -p "openrouter"

MODEL=""
PROVIDER=""
PROFILE="profiles/spike-romantasy-werewolf.md"
MAX_CHAPTERS=5
TIMEOUT_MINUTES=30
BINARY="spike-test/ainovel-cli.exe"

usage() {
  cat <<EOF
Usage: $(basename "$0") -m MODEL -p PROVIDER [options]

Required:
  -m MODEL        Model name, e.g. google/gemini-2.5-flash
  -p PROVIDER     Provider key in config, e.g. openrouter

Optional:
  -f PROFILE      Profile prompt file (default: $PROFILE)
  -c MAX_CHAPS    Stop after N chapters (default: $MAX_CHAPTERS)
  -t TIMEOUT      Hard stop after N minutes (default: $TIMEOUT_MINUTES)
  -b BINARY       Path to binary (default: $BINARY)
  -h              Show this help
EOF
  exit 1
}

while getopts "m:p:f:c:t:b:h" opt; do
  case "$opt" in
    m) MODEL="$OPTARG" ;;
    p) PROVIDER="$OPTARG" ;;
    f) PROFILE="$OPTARG" ;;
    c) MAX_CHAPTERS="$OPTARG" ;;
    t) TIMEOUT_MINUTES="$OPTARG" ;;
    b) BINARY="$OPTARG" ;;
    h|*) usage ;;
  esac
done

if [[ -z "$MODEL" || -z "$PROVIDER" ]]; then
  usage
fi

if [[ ! -f "$PROFILE" ]]; then
  echo "Profile not found: $PROFILE" >&2
  exit 1
fi

if [[ ! -f "$BINARY" ]]; then
  echo "Binary not found: $BINARY" >&2
  exit 1
fi

MODEL_SLUG=$(echo "$MODEL" | tr -cs 'a-zA-Z0-9_-' '-' | sed 's/--*/-/g')
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
REPORT_DIR="spike-reports/${TIMESTAMP}-${MODEL_SLUG}"
WORK_DIR="$REPORT_DIR/spike"

mkdir -p "$WORK_DIR/.ainovel/rules"

cp "$PROFILE" "$WORK_DIR/profile.md"
cp "spike-test/.ainovel/rules/lang-en.md" "$WORK_DIR/.ainovel/rules/lang-en.md"

cat > "$WORK_DIR/.ainovel/config.json" <<EOF
{
  "provider": "$PROVIDER",
  "model": "$MODEL",
  "budget": {
    "book_usd": 10,
    "warn_ratio": 0.8,
    "hard_stop": true
  },
  "notify": {
    "enabled": false
  }
}
EOF

START=$(date +%Y-%m-%dT%H:%M:%S)
echo "$START" > "$WORK_DIR/run-start.txt"

cd "$WORK_DIR"
"../../../$BINARY" --headless --prompt-file "profile.md" > run.log 2>&1 &
PID=$!
cd - >/dev/null

TIMEOUT=$((TIMEOUT_MINUTES * 60))
ELAPSED=0
while kill -0 "$PID" 2>/dev/null; do
  sleep 15
  ELAPSED=$((ELAPSED + 15))

  CHAPTER_COUNT=$(find "$WORK_DIR/output" -path '*/chapters/*.md' 2>/dev/null | wc -l | tr -d ' ')
  if [[ "$CHAPTER_COUNT" -ge "$MAX_CHAPTERS" ]]; then
    kill "$PID" 2>/dev/null || true
    break
  fi

  if [[ "$ELAPSED" -ge "$TIMEOUT" ]]; then
    kill "$PID" 2>/dev/null || true
    break
  fi
done

# Make sure process is gone
kill -9 "$PID" 2>/dev/null || true
wait "$PID" 2>/dev/null || true

END=$(date +%Y-%m-%dT%H:%M:%S)
echo "$END" > "$WORK_DIR/run-end.txt"

NOVEL_DIR=$(find "$WORK_DIR/output" -maxdepth 1 -type d | sed -n '2p')
CHAPTERS=0
REVIEWS=0
WORDS=0
COST_USD="N/A"

if [[ -d "$NOVEL_DIR/chapters" ]]; then
  CHAPTERS=$(find "$NOVEL_DIR/chapters" -name '*.md' | wc -l | tr -d ' ')
  WORDS=$(find "$NOVEL_DIR/chapters" -name '*.md' -exec wc -w {} + 2>/dev/null | tail -1 | awk '{print $1}')
fi

if [[ -d "$NOVEL_DIR/reviews" ]]; then
  REVIEWS=$(find "$NOVEL_DIR/reviews" -name '*.json' | wc -l | tr -d ' ')
  REWRITES=$(python3 -c "
import json, glob, sys
n = 0
for f in glob.glob('$NOVEL_DIR/reviews/*.json'):
    try:
        if json.load(open(f)).get('verdict') == 'rewrite':
            n += 1
    except Exception:
        pass
print(n)
" 2>/dev/null || echo 0)
fi

if [[ -f "$NOVEL_DIR/meta/usage.json" ]]; then
  COST_USD=$(python3 -c "import json,sys; print(json.load(open('$NOVEL_DIR/meta/usage.json'))['overall']['cost_usd'])" 2>/dev/null || echo "N/A")
fi

if [[ -d "$NOVEL_DIR/chapters" ]]; then
  find "$NOVEL_DIR/chapters" -name '*.md' -exec cat {} + > "$WORK_DIR/export.txt"
fi

DURATION=$(python3 -c "
from datetime import datetime
start=open('$WORK_DIR/run-start.txt').read().strip()
end=open('$WORK_DIR/run-end.txt').read().strip()
fmt='%Y-%m-%dT%H:%M:%S'
d=datetime.strptime(end,fmt)-datetime.strptime(start,fmt)
print(d)
" 2>/dev/null || echo "N/A")

EXTRAPOLATED="N/A"
if [[ "$CHAPTERS" -gt 0 && "$COST_USD" != "N/A" ]]; then
  EXTRAPOLATED=$(python3 -c "print(round($COST_USD * 30 / $CHAPTERS, 4))")
fi

cat > "$REPORT_DIR/report.md" <<EOF
# Model Spike Test Report

| Metric | Value |
|--------|-------|
| Model | $MODEL |
| Provider | $PROVIDER |
| Profile | $PROFILE |
| Max chapters target | $MAX_CHAPTERS |
| Chapters completed | $CHAPTERS |
| Total words | $WORDS |
| Reviews | $REVIEWS |
| Cost USD | $COST_USD |
| Duration | $DURATION |
| Work dir | $WORK_DIR |

## Files

- Log: \`$WORK_DIR/run.log\`
- Export: \`$WORK_DIR/export.txt\`
- Chapter 1: \`$NOVEL_DIR/chapters/01.md\`

## Quick quality check

Open chapter 1 and evaluate:

- Hook in opening paragraph?
- First-person voice consistent?
- No generic AI phrases ("little did she know", "a mix of")?
- Dialogue has distinct character voices?
- Chapter ends with unresolved tension?

## Notes

- Cost extrapolated to 30 chapters: $EXTRAPOLATED USD
- Rewrite rate: $REWRITES/$REVIEWS
EOF

echo "Spike test complete. Report: $REPORT_DIR/report.md"
echo "Chapters: $CHAPTERS | Words: $WORDS | Cost: $COST_USD USD"
