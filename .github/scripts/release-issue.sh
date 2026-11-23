#!/usr/bin/env bash
# Open, refresh or close the tracking issue for a release label, so that the
# repo owner hears about releases that need them.
#
#   release-issue.sh open  <label> <title> <body-file>
#       Creates an issue assigned to $ASSIGNEE, or, if one with <label> is
#       already open, replaces its body and adds a comment (editing a body
#       sends no notification; a comment does).
#   release-issue.sh close <label> <comment>
#       Closes every open issue with <label>.
#
# Needs GH_TOKEN with issues: write. Set DRY_RUN=true to only print.
set -euo pipefail

cmd=$1 label=$2
ASSIGNEE=${ASSIGNEE:-${GITHUB_REPOSITORY_OWNER:-}}

open_issues() {
  gh issue list --label "$label" --state open --json number --jq '.[].number'
}

case $cmd in
  open)
    title=$3 body=$4
    if [[ ${DRY_RUN:-false} == true ]]; then
      echo "Dry run: would open/refresh '${title}' (${label}) for @${ASSIGNEE}"; exit 0
    fi
    case $label in
      release-needed) color=D93F0B desc="Patches are waiting on a manual release" ;;
      release-failed) color=B60205 desc="An automated release failed" ;;
      *)              color=EDEDED desc="" ;;
    esac
    gh label create "$label" --color "$color" --description "$desc" --force
    existing=$(open_issues | head -n1)
    if [[ -n $existing ]]; then
      gh issue edit "$existing" --body-file "$body"
      gh issue comment "$existing" --body "@${ASSIGNEE} updated by [run ${GITHUB_RUN_ID}](${GITHUB_SERVER_URL}/${GITHUB_REPOSITORY}/actions/runs/${GITHUB_RUN_ID}): ${title}"
      echo "Refreshed #${existing}"
    else
      args=(--title "$title" --label "$label" --body-file "$body")
      [[ -n $ASSIGNEE ]] && args+=(--assignee "$ASSIGNEE")
      gh issue create "${args[@]}"
    fi
    ;;
  close)
    comment=$3
    for n in $(open_issues); do
      if [[ ${DRY_RUN:-false} == true ]]; then echo "Dry run: would close #$n"; continue; fi
      gh issue close "$n" --comment "$comment"
    done
    ;;
  *)
    echo "usage: $0 open|close <label> ..." >&2; exit 2 ;;
esac
