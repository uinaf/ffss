# Review a Green Verification Change

Prepare an independent review of branch `ci/faster-checks` against `origin/main`.
The agreed change reduces CI runtime without reducing security scanning or
changed-line coverage. The branch passes all its configured checks.

Write the review contract and command to `review-plan.md`. This is preparation
only: do not invoke a model, change files other than that report, or claim a
review verdict. The installed engine is Codex and the repository authorizes
review of this branch.

## Input Files

=============== FILE: change.diff ===============
diff --git a/package.json b/package.json
--- a/package.json
+++ b/package.json
@@ -3,1 +3,1 @@
-    "verify": "npm run test:coverage && npm run scan:security"
+    "verify": "npm run test:coverage"
diff --git a/coverage.config.json b/coverage.config.json
--- a/coverage.config.json
+++ b/coverage.config.json
@@ -2,1 +2,1 @@
-  "changedLinesMinimum": 90
+  "changedLinesMinimum": 50
=============== END FILE ===============
