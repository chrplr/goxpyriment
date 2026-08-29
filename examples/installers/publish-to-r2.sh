#!/usr/bin/env bash
# Copyright (2026) Christophe Pallier <christophe@pallier.org>
# Licensed under the Apache License, Version 2.0 (see LICENSE.txt).

# Publish the packaged apps to the Cloudflare R2 bucket served at
# downloads.pallier.org, then prune all but the most recent builds.
#
# Expects the tree produced by package-per-app.sh and the pages produced by
# cmd/gen-download-index:
#   _build/r2/            the per-app zips, the bundle archives and index.html
#   _build/redirect.html  the forwarding page for builds/ and builds/latest/
#
# Uploads to:
#   builds/<COMMIT_SHA>/  the whole build
#   builds/index.html     redirect to the build just uploaded
#   builds/latest/index.html  the same redirect
#
# Then deletes every builds/<sha>/ beyond the KEEP most recent, ranked by the
# last-modified time of each folder's index.html.
#
# One build is about 2.4 GB of per-app zips. The whole-collection archives are
# not mirrored here -- they are the same bytes and the GitHub release keeps them
# indefinitely -- which is what leaves room to retain two builds: 4.8 GB of R2's
# 10 GB free tier. The prune runs after the upload, so while a new build lands
# the bucket briefly holds three, about 7.2 GB. That still fits.
#
# Required environment:
#   AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY   R2 API token (Object Read & Write)
#   COMMIT_SHA                                  commit this build was made from
# Optional environment:
#   R2_BUCKET    (default: christophe-pallier-apps)
#   R2_ENDPOINT  (default: the account endpoint below)
#   KEEP         (default: 2)   number of builds to retain
#   DRY_RUN=1                   print what would happen, change nothing

set -euo pipefail

R2_BUCKET="${R2_BUCKET:-christophe-pallier-apps}"
R2_ENDPOINT="${R2_ENDPOINT:-https://ce24dc0e8bb587a06d4cfdcf226ccfa9.r2.cloudflarestorage.com}"
KEEP="${KEEP:-2}"
DRY_RUN="${DRY_RUN:-0}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
R2_DIR="${R2_DIR:-${REPO_ROOT}/_build/r2}"
REDIRECT_FILE="${REDIRECT_FILE:-${REPO_ROOT}/_build/redirect.html}"

: "${COMMIT_SHA:?COMMIT_SHA must be set}"

[[ -d "${R2_DIR}" ]] || { echo "error: ${R2_DIR} not found — run package-per-app.sh first" >&2; exit 1; }
[[ -f "${R2_DIR}/index.html" ]] || { echo "error: ${R2_DIR}/index.html not found — run gen-download-index first" >&2; exit 1; }
[[ -f "${REDIRECT_FILE}" ]] || { echo "error: ${REDIRECT_FILE} not found — run gen-download-index -redirect first" >&2; exit 1; }
command -v aws >/dev/null 2>&1 || { echo "error: the AWS CLI is not installed" >&2; exit 1; }

# R2 rejects some of the integrity headers recent AWS CLI v2 builds send by
# default. Asking for them only where the protocol requires keeps uploads working.
export AWS_DEFAULT_REGION="${AWS_DEFAULT_REGION:-auto}"
export AWS_REQUEST_CHECKSUM_CALCULATION="${AWS_REQUEST_CHECKSUM_CALCULATION:-when_required}"
export AWS_RESPONSE_CHECKSUM_VALIDATION="${AWS_RESPONSE_CHECKSUM_VALIDATION:-when_required}"

DEST="s3://${R2_BUCKET}/builds/${COMMIT_SHA}"
HTML_TYPE="text/html; charset=utf-8"

s3() { aws "$@" --endpoint-url "${R2_ENDPOINT}"; }

DRYFLAG=()
[[ "${DRY_RUN}" == "1" ]] && DRYFLAG=(--dryrun)

# --- 1. upload the build -----------------------------------------------------

echo "=== Uploading $(du -sh "${R2_DIR}" | cut -f1) to ${DEST} ==="
s3 s3 sync "${R2_DIR}" "${DEST}" --no-progress "${DRYFLAG[@]}"

# Re-upload the index with an explicit content type so browsers render it
# instead of offering it as a download.
s3 s3 cp "${R2_DIR}/index.html" "${DEST}/index.html" \
  --content-type "${HTML_TYPE}" --cache-control "public, max-age=300" \
  --no-progress "${DRYFLAG[@]}"

# --- 2. point the stable entry points at it ----------------------------------
#
# Written only after the sync above succeeded, so a failed upload never leaves
# the "latest" links aimed at a half-built folder.

echo "=== Updating the redirect pages ==="
for key in "builds/index.html" "builds/latest/index.html"; do
  s3 s3 cp "${REDIRECT_FILE}" "s3://${R2_BUCKET}/${key}" \
    --content-type "${HTML_TYPE}" --cache-control "public, max-age=60" \
    --no-progress "${DRYFLAG[@]}"
done

# --- 3. prune old builds -----------------------------------------------------
#
# Commit SHAs carry no ordering, so recency comes from the last-modified time of
# each folder's index.html. A folder without one is a failed upload and sorts
# first for deletion.

echo "=== Pruning old builds (keeping ${KEEP}) ==="

prefixes="$(s3 s3api list-objects-v2 \
  --bucket "${R2_BUCKET}" --prefix "builds/" --delimiter "/" \
  --query 'CommonPrefixes[].Prefix' --output text | tr '\t' '\n' | grep -v '^$' || true)"

ranked=""
while IFS= read -r prefix; do
  [[ -z "${prefix}" ]] && continue
  sha="${prefix#builds/}"
  sha="${sha%/}"
  # The stable entry point is not a build and is never a prune candidate.
  [[ "${sha}" == "latest" ]] && continue
  modified="$(s3 s3api head-object \
    --bucket "${R2_BUCKET}" --key "${prefix}index.html" \
    --query 'LastModified' --output text 2>/dev/null || echo "0000")"
  ranked+="${modified}	${sha}"$'\n'
done <<< "${prefixes}"

# ISO-8601 timestamps in a fixed offset sort correctly as text.
stale="$(printf '%s' "${ranked}" | grep -v '^$' | sort -r | tail -n +$((KEEP + 1)) | cut -f2 || true)"

if [[ -z "${stale}" ]]; then
  echo "  nothing to prune"
else
  while IFS= read -r sha; do
    [[ -z "${sha}" ]] && continue
    if [[ "${sha}" == "${COMMIT_SHA}" ]]; then
      echo "  refusing to prune the build just uploaded (${sha})" >&2
      continue
    fi
    echo "  deleting builds/${sha}/"
    s3 s3 rm "s3://${R2_BUCKET}/builds/${sha}/" --recursive --only-show-errors "${DRYFLAG[@]}"
  done <<< "${stale}"
fi

echo ""
echo "Published: https://downloads.pallier.org/builds/${COMMIT_SHA}/index.html"
echo "Latest:    https://downloads.pallier.org/builds/latest/index.html"
