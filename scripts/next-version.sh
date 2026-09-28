#!/bin/sh
# next-version.sh <commit>: prints "tag=vX.Y.Z" when go.mod requires another
# fortressedge release than it did at <commit>, and nothing otherwise.
# X.Y is fortressedge's major.minor; Z is one more than the provider's last
# vX.Y.* tag, or 0 for the first provider of that minor.
set -eu

fortressedge() {
	awk '$1 == "github.com/Sebiee/fortressedge" { print $2 } $1 == "require" && $2 == "github.com/Sebiee/fortressedge" { print $3 }'
}

now=$(fortressedge < go.mod)
before=$(git show "$1:go.mod" 2>/dev/null | fortressedge || true)
[ "$now" != "$before" ] || exit 0
case $now in
*-* | '')
	echo "fortressedge $now is not a release: no tag" >&2
	exit 0
	;;
v[0-9]*.[0-9]*.[0-9]*) ;;
*)
	echo "fortressedge $now is not a release: no tag" >&2
	exit 0
	;;
esac

minor=${now%.*}
last=$(git tag -l "$minor.*" --sort=-v:refname | head -n 1)
if [ -z "$last" ]; then
	echo "tag=$minor.0"
else
	echo "tag=$minor.$((${last##*.} + 1))"
fi
