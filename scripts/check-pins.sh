#!/bin/sh
# check-pins.sh [--fix]: go.mod's frp and yamux replaces must be the ones the
# required fortressedge release has, since a replace is not inherited from a
# dependency. --fix copies them over.
set -eu

pins() {
	grep -E '^replace github.com/(hashicorp/yamux|fatedier/frp) ' "$1" | sort
}

version=$(go list -m -f '{{.Version}}' github.com/Sebiee/fortressedge)
go mod download github.com/Sebiee/fortressedge
theirs=$(go list -m -f '{{.Dir}}' github.com/Sebiee/fortressedge)/go.mod
want=$(pins "$theirs")
have=$(pins go.mod)
[ "$want" != "$have" ] || exit 0

if [ "${1:-}" = --fix ]; then
	echo "$want" | while read -r _ module _ _; do
		line=$(grep -E "^replace $module " "$theirs")
		sed -i "s|^replace $module => .*|$line|" go.mod
	done
	go mod tidy
	echo "replaces now match fortressedge $version"
	exit 0
fi

echo "go.mod's frp and yamux replaces differ from fortressedge $version's; run scripts/check-pins.sh --fix"
echo "fortressedge $version:"
echo "$want"
echo "go.mod:"
echo "$have"
exit 1
