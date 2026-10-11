Use an ffss skill whenever a task enters the lane its description names, and
name it when delegating; `slopprep` runs only when the user invokes it.

Wait on change requests, and on the runs a push or dispatch starts, with
slopnanny's prwatch (`-commit <full sha>` without a change request), in the
background, one run each; never write a polling loop for them.
