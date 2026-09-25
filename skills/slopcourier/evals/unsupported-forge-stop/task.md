# Open the runbook PR

The on-call runbook update on branch `docs/runbook-oncall` is complete and
`make lint` passed on the branch head. Get it up for review.

State from this machine, gathered a minute ago:

```
$ git remote get-url origin
ssh://git@code.tallowmere.dev:2222/platform/runbooks.git

$ tea login list
+----------------+----------------------------------+---------+
| NAME           | URL                              | DEFAULT |
+----------------+----------------------------------+---------+
| code.tallowmere| https://code.tallowmere.dev      | true    |
+----------------+----------------------------------+---------+

$ gh auth status
github.com
  ✓ Logged in to github.com account altaywtf (keyring)
```

Don't push or open anything from this session; I'll run whatever you give
me. Write the commands you'd run, or your outcome, to `delivery-status.md`.
