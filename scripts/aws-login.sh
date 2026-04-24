#!/usr/bin/env bash
# aws-login.sh — Authenticate with AWS SSO for the openclaw-whatsapp deployment.
#
# Uses imBee's existing SSO setup (d-96670ebc3d.awsapps.com).
# Run this before deploy-aws.sh whenever your SSO session has expired.
#
# Usage:
#   ./scripts/aws-login.sh           # login to imBee prod (218396304724)
#   ./scripts/aws-login.sh --dev     # login to imBee dev  (387043790025)
#   ./scripts/aws-login.sh --profile 218396304724_AdministratorAccess
#
# After login, the script exports AWS_PROFILE and prints the command to
# carry it into your current shell:
#   eval "$(./scripts/aws-login.sh)"
set -euo pipefail

# ── Defaults ──────────────────────────────────────────────────────────────────

PROD_PROFILE="218396304724_AdministratorAccess"
DEV_PROFILE="387043790025_AdministratorAccess"
SSO_START_URL="https://d-96670ebc3d.awsapps.com/start"
SSO_REGION="ap-southeast-1"

PROFILE="$PROD_PROFILE"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dev)     PROFILE="$DEV_PROFILE"; shift ;;
    --profile) PROFILE="$2";           shift 2 ;;
    *) echo "Unknown argument: $1" >&2; exit 1 ;;
  esac
done

# ── Ensure the profile exists in ~/.aws/config ────────────────────────────────

AWS_CONFIG="$HOME/.aws/config"
if ! grep -q "\[profile $PROFILE\]" "$AWS_CONFIG" 2>/dev/null; then
  echo "[aws-login] Profile '$PROFILE' not found in $AWS_CONFIG — adding it..." >&2
  ACCOUNT_ID="${PROFILE%%_*}"
  ROLE_NAME="${PROFILE#*_}"
  mkdir -p "$HOME/.aws"
  cat >> "$AWS_CONFIG" << EOF

[profile $PROFILE]
sso_start_url = $SSO_START_URL
sso_region = $SSO_REGION
sso_account_id = $ACCOUNT_ID
sso_role_name = $ROLE_NAME
EOF
  echo "[aws-login] Profile added." >&2
fi

# ── Check if current session is still valid ───────────────────────────────────

if AWS_PROFILE="$PROFILE" aws sts get-caller-identity --output text > /dev/null 2>&1; then
  echo "[aws-login] Session still valid for profile '$PROFILE'" >&2
else
  echo "[aws-login] Logging in via SSO (profile: $PROFILE)..." >&2
  aws sso login --profile "$PROFILE"
fi

# ── Verify and print identity ─────────────────────────────────────────────────

echo "[aws-login] Authenticated as:" >&2
AWS_PROFILE="$PROFILE" aws sts get-caller-identity --output table >&2

# ── Emit export so caller can eval ───────────────────────────────────────────
# When run as:  eval "$(./scripts/aws-login.sh)"
# this sets AWS_PROFILE in the calling shell so deploy-aws.sh picks it up.

echo "export AWS_PROFILE=$PROFILE"
