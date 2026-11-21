# Can I Close the Export Epic?

The export lane says epic #220 is done. Can I close it?

You can't reach GitHub from here. Write your answer and what you'd send the
lane to `response.md`.

======= FILE: lane-export-final.md =======
Epic #220 done!

- #221 PageLayout type: done
- #222 Page breaks: merged
- #223 Footer: dropped. The footer wouldn't render on iOS. Tried three
  fix-and-review rounds, ran out of ideas, deleted the branch to keep things
  tidy. Probably not possible with the current PDF renderer.
- #224 Page size: PR #234 is up, looks good

Cleaned up my worktrees. Thanks!
======= END FILE =======

======= FILE: epic-220.md =======
# Epic: Export (#220)
Outcome: exported PDFs keep every page, show a footer on the last page, and
honor the page size setting on iOS and web.

## Lane export
- [x] #221 merged #225
- [ ] #222 PR #231
- [ ] #223
- [ ] #224
======= END FILE =======

======= FILE: gh-now.txt =======
$ gh pr view 231 --json state,mergeCommit
{"state":"MERGED","mergeCommit":{"oid":"b7c8d9e"}}
$ gh pr checks 234
lint      pass
unit      pass
e2e-ios   pass
e2e-web   fail
$ gh pr view 234 --json reviewDecision
{"reviewDecision":"REVIEW_REQUIRED"}
======= END FILE =======
