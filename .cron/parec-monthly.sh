#!/usr/bin/env bash
#
# Wrapper invoked by /etc/cron.d/parec on the 1st of each month. Pure
# system-level launcher: has no knowledge of the source repo. Four host
# paths only (first three overridable via env for testing), plus an
# msmtp configuration somewhere msmtp can find it.
#
#   /usr/local/bin/parec   — the binary           (PAREC_BIN)
#   /etc/parec/parec.env   — credentials          (PAREC_ENV)
#   /var/lib/parec/        — state dir            (PAREC_STATE)
#   /var/log/parec.log     — stderr log
#   ~/.msmtprc OR /etc/msmtprc — SMTP delivery config
#
# The binary reads/writes its cookie jar and cache under ./data/, so we
# cd into the state dir and let it manage that subtree itself. The log
# file must be owned by the cron user (created by `make install-cron`);
# /var/log itself is not writable by non-root, so log rotation here is
# in-place truncation rather than a rename. The monthly report is
# composed in-memory and piped through `msmtp -t` (recipient comes from
# the To: header) — no local MTA required.

set -euo pipefail

BIN="${PAREC_BIN:-/usr/local/bin/parec}"
ENV_FILE="${PAREC_ENV:-/etc/parec/parec.env}"
STATE_DIR="${PAREC_STATE:-/var/lib/parec}"
LOG_FILE="/var/log/parec.log"
# PAREC_RECIPIENT is required; it's expected to come from $ENV_FILE,
# so the variable is resolved AFTER that file is sourced below.
# Multiple addresses are allowed — separate with commas, e.g.
#     PAREC_RECIPIENT="a@x.com, b@y.com"
# They all go into the To: header; msmtp -t walks the header list.

if [[ ! -x "$BIN" ]]; then
    echo "ERROR: $BIN not found or not executable — \`make install-cron\` from the repo." >&2
    exit 1
fi
if [[ ! -f "$ENV_FILE" ]]; then
    echo "ERROR: $ENV_FILE missing — \`make install-cron\` from the repo." >&2
    exit 1
fi
if [[ ! -d "$STATE_DIR" || ! -w "$STATE_DIR" ]]; then
    echo "ERROR: $STATE_DIR missing or not writable by $(id -un)." >&2
    exit 1
fi
if [[ ! -w "$LOG_FILE" ]]; then
    echo "ERROR: $LOG_FILE missing or not writable by $(id -un) — \`make install-cron\` from the repo." >&2
    exit 1
fi
if ! command -v msmtp >/dev/null 2>&1; then
    echo "ERROR: msmtp not on PATH — install it and configure ~/.msmtprc (template in repo at .cron/msmtprc.example)." >&2
    exit 1
fi

set -a
# shellcheck disable=SC1090
. "$ENV_FILE"
set +a

# Resolve recipients now that the env file has been sourced. Accept
# comma-separated values; normalize whitespace so the To: header has
# exactly one ", " between addresses. Reject if every entry was blank.
if [[ -z "${PAREC_RECIPIENT:-}" ]]; then
    echo "ERROR: PAREC_RECIPIENT not set — add it to $ENV_FILE." >&2
    exit 1
fi
RECIPIENTS=$(printf '%s' "$PAREC_RECIPIENT" \
    | sed -e 's/[[:space:]]*,[[:space:]]*/, /g' \
          -e 's/^[[:space:]]*//' \
          -e 's/[[:space:]]*$//')
if [[ -z "$RECIPIENTS" || "$RECIPIENTS" == "," ]]; then
    echo "ERROR: PAREC_RECIPIENT contains no usable addresses." >&2
    exit 1
fi

cd "$STATE_DIR"

# Trim log in-place if it's grown past ~1 MB. We can't `mv` a sibling
# into /var/log/ (the dir is root-owned), so stage the tail in /tmp and
# truncate-write the existing log file (write permission on the file is
# enough — directory write is not required).
if [[ -f "$LOG_FILE" && $(stat -c%s "$LOG_FILE" 2>/dev/null || echo 0) -gt 1048576 ]]; then
    TRIM=$(mktemp)
    tail -c 524288 "$LOG_FILE" > "$TRIM"
    cat "$TRIM" > "$LOG_FILE"
    rm -f "$TRIM"
fi

# Run parec; capture stdout (the report) into a tempfile, stream stderr
# to the log. Don't let a non-zero exit kill the wrapper before we send
# email — failures should be delivered too.
BODY=$(mktemp)
trap 'rm -f "$BODY"' EXIT

EXIT=0
{
    echo "== parec monthly report — $(date -Iseconds) =="
    echo
    "$BIN" --months=3 2>>"$LOG_FILE"
} > "$BODY" || EXIT=$?

SUBJECT="parec monthly report — $(date '+%Y-%m')"
if [[ $EXIT -ne 0 ]]; then
    SUBJECT="$SUBJECT [FAILED exit=$EXIT]"
    {
        echo
        echo "---"
        echo "parec exited with status $EXIT. Tail of $LOG_FILE:"
        tail -n 50 "$LOG_FILE" 2>/dev/null || true
    } >> "$BODY"
fi

# Compose RFC-822 message and hand it to msmtp. `msmtp -t` reads
# recipients from the To: header; envelope From is taken from the
# msmtp account's `from` setting, so we don't hardcode it here.
{
    printf 'To: %s\r\n' "$RECIPIENTS"
    printf 'Subject: %s\r\n' "$SUBJECT"
    printf 'Date: %s\r\n' "$(date -R)"
    printf 'MIME-Version: 1.0\r\n'
    printf 'Content-Type: text/plain; charset=UTF-8\r\n'
    printf '\r\n'
    cat "$BODY"
} | msmtp -t

exit "$EXIT"
