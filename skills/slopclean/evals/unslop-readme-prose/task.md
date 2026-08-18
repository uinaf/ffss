# Clean an AI-Written README

## Problem/Feature Description

The README below was drafted by a code assistant and it shows. Clean it up so
it reads like a person wrote it on purpose. Do not change what it says: the
commands, numbers, and supported platforms must survive exactly. Edit
`README.md` in place.

## Input Files

=============== FILE: README.md ===============
# streamlog

streamlog is a robust, cutting-edge log shipper — designed to seamlessly
aggregate logs from all your services. In today's fast-paced world of
distributed systems, observability is more crucial than ever, and streamlog
delves into this challenge head-on.

## Key Features

- **Blazing Fast:** streamlog leverages a highly optimized pipeline to
  process up to 120,000 lines per second on a single core.
- **Seamless Integration:** it's worth noting that streamlog integrates
  effortlessly with your existing stack.
- **Robust Reliability:** studies show that at-least-once delivery matters —
  streamlog delivers.

## Getting Started

Getting started is incredibly simple — just run:

```
curl -fsSL https://get.streamlog.dev | sh
streamlog init --port 5140
```

It should be noted that streamlog supports Linux and macOS. Windows support
may potentially be considered in the future, but there are currently no
concrete plans at this time.

## Conclusion

In conclusion, streamlog represents a paradigm shift in the log-shipping
landscape. Whether you're a startup or an enterprise, streamlog empowers you
to unlock the full potential of your logs.
=============== END FILE ===============
