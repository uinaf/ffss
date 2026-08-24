# ffss CLI library

Shared Go packages used by the ffss command-line tools. This module has no
standalone binary or release assets.

Module versions use repository tags such as `cli/lib/v0.1.0`. Consumers must
depend on a published version so `GOWORK=off` and downloaded-module
`go install` builds remain reproducible.

Run `mise run verify` before tagging or updating a consumer.
