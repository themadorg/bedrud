#!/usr/bin/env bash
# Fail on links into this repository that cannot resolve.
#
# Every tree/ or blob/ link into bedrud names a ref and a path, and both can rot
# without anyone noticing: a ref that is not a branch here answers 404 exactly
# like a deleted file does, and neither shows up in a build. The checkout the
# links ship with is enough to settle both questions, so this runs offline —
# no rate limits, no flakes, and a pull request is told before it merges rather
# than a reader finding out later.
#
# What it does not check: links to other repositories, and anchors within a
# page. Those need the network, which is the part that makes a link checker
# unreliable enough to be ignored.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

readonly SLUG="themadorg/bedrud"
# The branch the published docs point at. Repository default; not "main".
readonly REF="${BEDRUD_LINK_REF:-master}"

# The pattern is assembled rather than written out so that this file does not
# match its own search.
readonly PATTERN="https://github\\.com/${SLUG}/(tree|blob)/[^)\"'\`<>[:space:]]+"

fail=0
checked=0

while IFS= read -r hit; do
	file="${hit%%:*}"
	rest="${hit#*:}"
	line="${rest%%:*}"
	link="${rest#*:}"

	# Trim what markdown leaves attached to the end of a bare link, then drop
	# any anchor: the fragment is a question about a rendered page, not about
	# whether the file exists.
	link="${link%%#*}"
	link="${link%%[.,;:]}"

	tail="${link#https://github.com/${SLUG}/}"
	kind="${tail%%/*}"          # tree or blob
	tail="${tail#*/}"
	ref="${tail%%/*}"
	path="${tail#*/}"
	if [ "$path" = "$ref" ]; then
		path=""                 # the link stops at the ref
	fi

	checked=$((checked + 1))

	if [ "$ref" != "$REF" ]; then
		printf '%s:%s: ref %q is not %q — %s\n' "$file" "$line" "$ref" "$REF" "$link"
		fail=$((fail + 1))
		continue
	fi

	if [ -n "$path" ] && [ ! -e "$path" ]; then
		printf '%s:%s: %s does not exist in the tree — %s\n' "$file" "$line" "$path" "$link"
		fail=$((fail + 1))
		continue
	fi

	# A blob link has to name a file and a tree link a directory, or the page
	# renders something other than what the text promises.
	if [ "$kind" = "blob" ] && [ -n "$path" ] && [ -d "$path" ]; then
		printf '%s:%s: blob link points at a directory — %s\n' "$file" "$line" "$link"
		fail=$((fail + 1))
	fi
done < <(git grep -EIno "$PATTERN" -- . ":(exclude)scripts/check-repo-links.sh" || true)

if [ "$fail" -gt 0 ]; then
	printf '\n%d of %d repository links do not resolve.\n' "$fail" "$checked" >&2
	exit 1
fi

printf '%d repository links resolve against this checkout.\n' "$checked"
