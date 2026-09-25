# Choose evidence for three finished changes

Write `delivery-notes.md` with a short change-request body and your evidence
decision for each independent case below. All changes are authorized for
delivery, scoped correctly, and verified on their final revision. There is no
body template. Do not publish anything or create media. Supplied evidence is
the complete available evidence.

1. A configuration loader now rejects an empty `CACHE_DIR` instead of passing
   it to the filesystem API. The diff adds the empty-string check and one
   regression case. The test passes on the change and fails on the base.
   CI displays this check. There is no visible UI change.
2. Upload cancellation previously left retry timers running and could start
   another upload after the user cancelled. The finished change coordinates
   three entry points: user cancellation marks the operation cancelled before
   aborting its active request; retry scheduling checks that state before
   setting a timer; a timer callback checks it again before starting a request.
   An already queued callback can still run after its timer is cleared. The
   regression tests cover cancellation before scheduling and after the timer
   callback is queued; they fail on the base and pass on the change. CI shows
   them. The diff spans several functions, obscuring their ordering.
3. A settings panel now preserves unsaved input when a save fails and offers a
   keyboard-accessible retry action. Final-revision desktop, mobile, keyboard,
   and accessibility checks passed. An existing captioned recording at
   https://media.example.com/settings-save-retry.webm shows the failed save,
   retained input, and successful retry. The recording has been inspected and
   matches this revision; intended reviewers can access it. CI cannot show
   this interaction.
