# Prose

Keep the author's voice. Every cut keeps its fact; every rewrite names
something the reader can do, check, or look up.

- **Copied config:** replace prose tables of options, defaults, inventories,
  or commands with a link to the owning file and its key or symbol. Read the
  source first; if you can't, flag the suspected copy. Keep rationale,
  constraints, recovery steps, and examples the source can't hold.
- **Vague attribution:** name the source only if the materials do; otherwise
  flag it. Keep uncertainty the evidence supports.
- **Puffery:** cut promotional words, AI vocabulary, intensifiers, and filler
  (seamless, robust, crucial, pivotal, delve, underscore, testament,
  incredibly, "it is important to note", "In conclusion") without rephrasing
  them into new filler. Where the materials give a number, the number
  replaces the praise.
- **Feeling over mechanism:** "SQL you can read" or "types that follow your
  schema" names a feeling. Name the mechanism instead: "`.toSQL()` returns
  the exact query string", "a column rename fails the build". A sentence that
  could sit unchanged in another project's docs says nothing about this one;
  cut it.
- **Stand-in words:** write the real symbol, file, flag, or command, not a
  description of it. Use the plain verb: use, help, is, has, not leverage,
  facilitate, serves as, boasts.
- **Mannered prose:** metaphor nouns (substrate, wedge, vector, primitive as
  a noun, north star, flywheel, ratchet, endgame), personified code ("the
  plan holds it"), figurative verbs ("rides along"), and aphorisms become the
  literal phrase. A named pattern the doc defines on first use stays.
- **Template shapes:** "not just X, but Y"; ideas forced into threes;
  trailing "-ing" clauses that claim significance (", ensuring
  reliability"); false ranges ("from startups to enterprises"); a bold
  inline header the line repeats ("**Speed:** Speed improved"). State the
  point once, in its natural shape.
- **One name per thing:** pick one term for each concept and repeat it.
  Synonym cycling teaches the reader several things where there is one.
- **Hedge stacks:** "could potentially possibly" becomes the single hedge the
  evidence supports.
- **Chat residue:** "Great question", "Certainly!", "I hope this helps",
  "Let me know if", and decorative emoji go.
- **Over-compression:** arrows, symbol-speak, undefined abbreviations, and
  sentences missing their articles or verbs. "Bad date → exit 2, no write"
  becomes "The parser rejects a bad date, exits with code 2, and writes
  nothing."
- **Two readings:** untangle noun stacks ("the proto import budget check
  script" becomes "the script that checks the proto-import budget"); point
  "this" and "which" at one noun, not a whole clause; keep "only" and "not"
  beside the word they change; name the actor a passive hides; replace "a/b"
  and "(s)" with words.
- **Structure:** keep heading case, useful headings, lists, and emphasis.
  Em dashes, colons, and short sentences are fine; cut punctuation only where
  it's repeated scaffolding.
- Add no jokes, opinions, or forced variety.
