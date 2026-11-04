#!/usr/bin/env bash
# Prints the version the next release gets, such as v0.2.0, or nothing
# when the commits since the last release make none. The release
# workflow and `make next-version` run it; the rules are in
# docs/conventions.md ("Releases"). A Release-As trailer sets the
# version outright (below).
#
# cog bump --auto takes the version from the Conventional Commits since
# the last tag (cog.toml). Before 1.0.0 it raises nothing for a breaking
# change unless it is a feat, so here any breaking change raises the
# minor version then. Output from cog that is neither a version nor its
# "nothing to bump" message is an error: a broken setup must not pass
# for a week with nothing to release.
set -euo pipefail
cd "$(dirname "$0")/.."

semver='^v[0-9]+\.[0-9]+\.[0-9]+$'

# A commit that is in a release already makes no other: a re-run of a
# release, or of an older run after a newer release, stops here.
if [[ -n $(git tag --list 'v[0-9]*' --contains HEAD) ]]; then
  exit 0
fi

# --skip-untracked: cog refuses a dry run beside untracked files, which
# a working copy often has.
out=$(cog bump --auto --dry-run --skip-untracked)
if [[ $out =~ $semver ]]; then
  next=$out
elif [[ $out == 'No conventional commits for your repository that required a bump.'* ]]; then
  next=
else
  printf 'next-version: unexpected output from cog bump:\n%s\n' "$out" >&2
  exit 1
fi

last=$(git describe --tags --abbrev=0 --match 'v[0-9]*' 2>/dev/null || true)
if [[ -z $last || $last == v0.* ]]; then
  # cog's own parser decides what is breaking: a "!" or a BREAKING
  # CHANGE footer, in any commit since the last tag.
  template=$(mktemp)
  trap 'rm -f "$template"' EXIT
  printf '%s' '{% for commit in commits %}{% if commit.breaking_change %}breaking {% endif %}{% endfor %}' >"$template"
  # Assigned first: a failure inside [[ ]] would not stop the script.
  breaking=$(cog changelog --template "$template" "$last..HEAD")
  if [[ $breaking == *breaking* ]]; then
    next=$(cog bump --minor --dry-run --skip-untracked)
    if [[ ! $next =~ $semver ]]; then
      printf 'next-version: unexpected output from cog bump --minor:\n%s\n' "$next" >&2
      exit 1
    fi
  fi
fi

# A "Release-As: vX.Y.Z" line in a commit since the last tag (the squash
# merge's body, from its pull request's description) sets the
# version outright, such as v1.0.0, which the rules above never reach
# from v0.x. It must be past the last tag and the version computed; a
# commit of any type makes that release.
range=HEAD
if [[ -n $last ]]; then
  range=$last..HEAD
fi
# A line of its own, not git's trailers: those miss it after a BREAKING
# CHANGE footer, whose token has a space.
wanted=$(git log --format=%B "$range" | sed -n 's/^Release-As:[[:space:]]*//p' | sort -uV | tail -n 1)
if [[ -n $wanted ]]; then
  wanted=${wanted//[[:space:]]/}
  if [[ ! $wanted =~ $semver ]]; then
    printf 'next-version: Release-As %s: want vX.Y.Z\n' "$wanted" >&2
    exit 1
  fi
  for floor in $last $next; do
    if [[ $(printf '%s\n%s\n' "$floor" "$wanted" | sort -V | tail -n 1) != "$wanted" || $floor == "$wanted" ]]; then
      printf 'next-version: Release-As %s is not past %s\n' "$wanted" "$floor" >&2
      exit 1
    fi
  done
  next=$wanted
fi

if [[ -n $next ]]; then
  echo "$next"
fi
