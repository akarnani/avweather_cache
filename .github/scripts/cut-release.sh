#!/usr/bin/env bash
# Cut release v$VERSION from the current checkout of main: bump the chart's
# version/appVersion, commit, tag, push, and (when pushing with GITHUB_TOKEN)
# start ci.yml on the tag, which publishes the image and the Helm chart.
#
# Env:
#   VERSION        new version, X.Y.Z (no "v")
#   LAST_TAG       previous tag, for the commit message (may be empty)
#   COMMITS_FILE   Markdown commit list from release-check.sh (may be empty)
#   RELEASE_KIND   short label for the commit/tag message, e.g. "automated patch release"
#   HAS_BOT_TOKEN  "true" if pushing with RELEASE_BOT_TOKEN (the tag push then
#                  triggers CI by itself)
#   DRY_RUN        "true" to stop before committing
set -euo pipefail

CHART=${CHART:-deploy/charts/avweather-cache/Chart.yaml}
RELEASE_KIND=${RELEASE_KIND:-release}
TAG="v${VERSION}"

if [[ ! $VERSION =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "::error::Version '${VERSION}' is not X.Y.Z"; exit 1
fi
if git rev-parse -q --verify "refs/tags/${TAG}" >/dev/null; then
  echo "::error::${TAG} already exists"; exit 1
fi

sed -i -E "s/^version: .*/version: ${VERSION}/; s/^appVersion: .*/appVersion: \"${VERSION}\"/" "$CHART"
git diff

{
  echo "Release ${TAG}"
  echo
  echo "${RELEASE_KIND^}. Changes since ${LAST_TAG:-the beginning}:"
  echo
  if [[ -s ${COMMITS_FILE:-} ]]; then
    # shellcheck disable=SC2016
    sed -E 's/^- \[[x ]\] \[`([0-9a-f]+)`\]\([^)]*\) /- \1 /' "$COMMITS_FILE"
  fi
} > "${RUNNER_TEMP:-/tmp}/release-msg.txt"
cat "${RUNNER_TEMP:-/tmp}/release-msg.txt"

if [[ ${DRY_RUN:-false} == true ]]; then
  echo "Dry run: not committing, tagging or pushing ${TAG}."
  git checkout -- "$CHART"
  exit 0
fi

git config user.name "github-actions[bot]"
git config user.email "41898282+github-actions[bot]@users.noreply.github.com"
git commit -a -F "${RUNNER_TEMP:-/tmp}/release-msg.txt"
git tag -a "$TAG" -m "Release ${TAG} (${RELEASE_KIND})"
git push --atomic origin HEAD:main "refs/tags/${TAG}"

if [[ ${HAS_BOT_TOKEN:-false} != true ]]; then
  # Pushes made with GITHUB_TOKEN don't trigger workflows, so start the
  # image + chart release on the new tag explicitly.
  gh workflow run ci.yml --ref "$TAG"
fi

if [[ -n ${GITHUB_STEP_SUMMARY:-} ]]; then
  {
    echo "### Released ${TAG}"
    echo
    echo "Image and chart are being published by [CI on ${TAG}](${GITHUB_SERVER_URL}/${GITHUB_REPOSITORY}/actions/workflows/ci.yml?query=branch%3A${TAG})."
    echo
    [[ -s ${COMMITS_FILE:-} ]] && cat "$COMMITS_FILE"
  } >> "$GITHUB_STEP_SUMMARY"
fi
