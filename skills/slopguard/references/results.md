# Results

Results go to stdout, progress to stderr; terminal and JSON carry the same
report. Each finding has priority, confidence, category, and a location in
the frozen target.

Exit 2 failure classes include `authentication`, `capability`, `timeout`, and
`source_changed`. slopguard retries once, only for a malformed protocol
response; everything else is reported as is.

Fields and enums: [result contract](https://github.com/uinaf/ffss/blob/main/cli/slopguard/docs/RESULT_SCHEMA.md).
