# Plan a Resumable Event Feed

Draft the agreed event-feed work for this repository's GitHub Issues. Do not
publish or implement. Put the proposed issue bodies and blocking links in
`planning-result.md`; this report is a draft, not a second backlog.

## Input Files

=============== FILE: agreement.md ===============
Users must recover all events after a 72-hour disconnection. The planned
implementation uses the existing provider's replay cursor. No one has verified
its retention window; the available API reference does not specify it. If
72-hour replay is unsupported, the team must choose another persistence design
before implementing resumable delivery.

The team approved a bounded investigation using the existing local provider
fixture and maintained contract docs, with no paid or production calls. Those
may leave the question unresolved; report that honestly.

After feasibility is established, implement cursor recovery and its reconnect
tests as one behavior slice. Separately fix the event settings page's keyboard
focus order; it uses existing controls and needs no replay-provider changes.
The keyboard issue can ship independently.
=============== END FILE ===============
