We agreed on the cache invalidation change in AGREED_PLAN.md. Make this a
durable plan for later so whoever picks it up next doesn't need this
conversation. Don't create anything on GitHub yourself, and don't start on the
code.

=============== FILE: .git/config ===============
[remote "origin"]
  url = git@github.com:acme/cache-service.git
=============== END FILE ===============

=============== FILE: README.md ===============
# cache-service

Read-through cache in front of the accounts API.

    npm ci && npm test
=============== END FILE ===============

=============== FILE: AGREED_PLAN.md ===============
Outcome: stale cache entries are invalidated after account deletion.
Constraints: preserve current event ordering and add integration coverage.
Size: one reviewable change.
=============== END FILE ===============
