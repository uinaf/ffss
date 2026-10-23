## Replies

Applies to everything a person reads: chat and thread replies, Slack posts,
automation summaries, PR and issue bodies, comments, and commits.

Write about 80% of the way to ASD-STE100 Simplified Technical English.

- One idea per sentence, under 20 words. Active voice, simple verbs, no
  stacked clauses.
- One fact per line. Bullets and short labels beat paragraphs.
- Whole reply: 4 short lines or fewer unless asked for detail.
- Lead with the answer. No preamble, no recap of what you just did.
- Leave out mechanics, background, reasoning, and caveats unless they change
  what the reader does next.
- No filler, apologies, or closing offers. No updates for routine progress.
- Link every reference; backticks for literals only; link long output.
- After a change: status, links, and what's unverified or waiting on the
  reader.
- Ask only for decisions that belong to the reader; act on the rest.

## Example

Bad:
> I looked into the staging issue and it turns out the transfers page is
> failing because `/v2/transfers` started returning 500s after yesterday's
> deploy, which seems to be caused by the new pagination parameter the
> frontend sends but the API doesn't accept yet, so we either need to roll
> back the frontend change or ship the API fix that's already in review.

Good:
> Transfers page is broken on staging.
> `/v2/transfers` returns 500 since [deploy #412](link).
> Cause: the frontend sends a `cursor` param the API rejects.
> Fix: merge [API #91](link) or revert [web #88](link).
