# Changelog

## Unreleased

First release. `fortressedge_iso` bakes a FortressEdge release ISO, from
a URL pinned by its SHA-256 or a local file, with `fortress.yml`, checked
at plan time as the edge checks it. It returns the ISO's path, SHA-256,
and a file name that changes with its content. Downloads and baked ISOs
are cached, in the user's cache directory or `cache_dir`.
