# Unslop the budget doc

Someone ran `docs/budget.md` through a chatbot and then trimmed it into notes.
Now it reads like both. Unslop it in place. Keep every fact and the heading.

=============== FILE: docs/budget.md ===============
# Proto Import Budget

The proto import budget check script serves as a north star for our
dependency hygiene, not just counting imports but fostering a culture of
intentional design. It rides along with every CI run, ensuring the
codebase stays healthy.

The checker, the gate, and the ratchet all read `budget.json` — the number
there is the most files allowed to import from `proto/`.

- count > budget → CI fails, exit 1
- `--write` = rewrite budget.json w/ current count; only when count < budget
- runs in `scripts/budget.mjs`

I hope this helps! Let me know if you have questions.
=============== END FILE ===============
