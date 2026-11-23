#!/usr/bin/env bash
# Decide whether main can be released automatically.
#
# A release is automatic only when every commit since the latest vX.Y.Z tag
# is a dependency/base-image patch, i.e. authored by Dependabot. Merge commits
# ("Merge pull request #N ...") are neutral: they carry no changes of their
# own, and the commits they bring in are judged individually. Anything else
# needs a human to cut the release.
#
# Writes these key=value lines to $GITHUB_OUTPUT (or stdout):
#   status        none | patch-only | needs-manual | skip
#   last_tag      latest vX.Y.Z tag reachable from HEAD_REF
#   next_version  last_tag with the patch component bumped (no "v")
# and a Markdown list of the commits in the range to $COMMITS_FILE.
#
# Optional env:
#   HEAD_REF      ref to evaluate (default HEAD)
#   PUSH_BEFORE   on push events, the previous tip; if the pushed commits
#                 contain no Dependabot commit the run reports "skip" so that
#                 ordinary pushes (feature work, manual release bumps) never
#                 trigger a notification on their own.
set -euo pipefail

HEAD_REF=${HEAD_REF:-HEAD}
COMMITS_FILE=${COMMITS_FILE:-/dev/null}
OUT=${GITHUB_OUTPUT:-/dev/stdout}
REPO_URL=${GITHUB_SERVER_URL:-https://github.com}/${GITHUB_REPOSITORY:-akarnani/avweather_cache}

BOT_EMAIL_RE='dependabot\[bot\]@users\.noreply\.github\.com$'

emit() { echo "$1=$2" >> "$OUT"; }

# Prints "merge", "patch" or "other".
classify_commit() {
  local sha=$1 parents
  parents=$(git log -1 --format='%p' "$sha" | wc -w)
  if [[ $parents -gt 1 ]]; then
    echo merge
  elif [[ $(git log -1 --format='%ae' "$sha") =~ $BOT_EMAIL_RE ]]; then
    echo patch
  else
    echo other
  fi
}

if [[ -n ${PUSH_BEFORE:-} && ! ${PUSH_BEFORE} =~ ^0+$ ]] && git cat-file -e "${PUSH_BEFORE}^{commit}" 2>/dev/null; then
  if ! git log --no-merges --format='%ae' "${PUSH_BEFORE}..${HEAD_REF}" | grep -Eq "$BOT_EMAIL_RE"; then
    echo "Push contains no Dependabot commits; nothing to do."
    emit status skip
    exit 0
  fi
fi

last_tag=$(git tag --list 'v[0-9]*.[0-9]*.[0-9]*' --merged "$HEAD_REF" --sort=-v:refname | head -n1)
if [[ -z $last_tag ]]; then
  echo "No vX.Y.Z tag found; a manual release is needed to set a baseline."
  emit status needs-manual
  emit last_tag ""
  emit next_version ""
  exit 0
fi

ver=${last_tag#v}
IFS=. read -r major minor patch <<< "$ver"
next_version="${major}.${minor}.$((patch + 1))"
emit last_tag "$last_tag"
emit next_version "$next_version"

mapfile -t shas < <(git rev-list --reverse "${last_tag}..${HEAD_REF}")
if [[ ${#shas[@]} -eq 0 ]]; then
  echo "No commits since ${last_tag}."
  emit status none
  : > "$COMMITS_FILE"
  exit 0
fi

status="patch-only"
: > "$COMMITS_FILE"
for sha in "${shas[@]}"; do
  kind=$(classify_commit "$sha")
  desc=$(git log -1 --format='%s (%an)' "$sha")
  link="[\`${sha:0:7}\`](${REPO_URL}/commit/${sha})"
  printf '  %-6s %s %s\n' "$kind" "${sha:0:7}" "$desc"
  case $kind in
    patch) echo "- [x] ${link} ${desc}" >> "$COMMITS_FILE" ;;
    merge) echo "- [x] ${link} ${desc} _(merge)_" >> "$COMMITS_FILE" ;;
    *)     echo "- [ ] ${link} ${desc} **(not a patch)**" >> "$COMMITS_FILE"
           status=needs-manual ;;
  esac
done

echo "Since ${last_tag}: ${#shas[@]} commit(s), status=${status}, next=v${next_version}"
emit status "$status"
