#!/usr/bin/env bash

set -euo pipefail

if [ ! -t 0 ] || [ ! -t 1 ]; then
  echo "commit helper requires an interactive terminal"
  exit 1
fi

if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  echo "not inside a git repository"
  exit 1
fi

branch="$(git branch --show-current)"
if [ -z "$branch" ]; then
  echo "could not determine current branch"
  exit 1
fi

if git diff --quiet && git diff --cached --quiet; then
  echo "no changes to commit"
  exit 0
fi

echo "current branch: $branch"
echo
git status --short
echo

read -r -p "stage all changes with 'git add .'? [y/N] " stage_all
case "$stage_all" in
  y|Y) git add . ;;
  *) echo "aborted"; exit 1 ;;
esac

if git diff --cached --quiet; then
  echo "no staged changes to commit"
  exit 1
fi

echo
echo "select commit type:"
types=(feat fix chore docs refactor test ci perf build revert)
for i in "${!types[@]}"; do
  printf "  %d) %s\n" "$((i + 1))" "${types[$i]}"
done

read -r -p "type [1-${#types[@]}] (default: chore): " type_choice
if [ -z "$type_choice" ]; then
  commit_type="chore"
elif [[ "$type_choice" =~ ^[0-9]+$ ]] && [ "$type_choice" -ge 1 ] && [ "$type_choice" -le "${#types[@]}" ]; then
  commit_type="${types[$((type_choice - 1))]}"
else
  echo "invalid commit type selection"
  exit 1
fi

read -r -p "optional scope (leave blank for none): " commit_scope

while :; do
  read -r -p "commit summary: " commit_summary
  if [ -n "${commit_summary// }" ]; then
    break
  fi
  echo "summary is required"
done

commit_message="$commit_type"
if [ -n "${commit_scope// }" ]; then
  commit_message="$commit_message($commit_scope)"
fi
commit_message="$commit_message: $commit_summary"

echo
echo "commit message: $commit_message"
read -r -p "create commit? [Y/n] " confirm_commit
case "$confirm_commit" in
  n|N) echo "aborted"; exit 1 ;;
esac

git commit -m "$commit_message"

echo
read -r -p "push branch '$branch' to origin? [Y/n] " confirm_push
case "$confirm_push" in
  n|N)
    echo "commit created locally; push skipped"
    exit 0
    ;;
esac

if git rev-parse --verify "origin/$branch" >/dev/null 2>&1; then
  git push origin "$branch"
else
  git push -u origin "$branch"
fi
