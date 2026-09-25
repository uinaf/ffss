# Unslop the streamlog README

An assistant rewrote our README last week and it now reads like a brochure.
Unslop it. The install commands, the throughput figure, and the supported
platforms stay as they are. Edit `README.md` in place; `src/config.ts` is
there for reference, don't touch it.

=============== FILE: README.md ===============
# streamlog

streamlog is a robust, cutting-edge log shipper designed to seamlessly
aggregate logs from all your services. In today's fast-paced world of
distributed systems, observability is more crucial than ever.

## Key Features

- **Blazing Fast:** streamlog leverages a highly optimized pipeline to process
  up to 120,000 lines per second on a single core.
- **Robust Reliability:** delivery is at-least-once — a crash between read and
  ack replays the batch rather than dropping it.

## Getting Started

Getting started is incredibly simple! Just run:

```
curl -fsSL https://get.streamlog.dev | sh
streamlog init --port 5140
```

streamlog supports Linux and macOS. Windows support may potentially be
considered in the future, but there are currently no concrete plans.

## Configuration Options

streamlog offers a rich and flexible set of configuration options to empower
you to tailor it to your unique needs:

| Option          | Default | Description                                   |
| --------------- | ------- | --------------------------------------------- |
| `port`          | `5140`  | TCP port the collector listens on             |
| `batchSize`     | `500`   | Lines per outbound batch                      |
| `flushMs`       | `250`   | Maximum time a partial batch waits            |
| `maxRetries`    | `8`     | Delivery attempts before a batch is parked    |
| `spoolDir`      | `/var/lib/streamlog` | Where parked batches go          |

Keep `flushMs` below your downstream's idle timeout — otherwise the connection
drops between batches and every flush pays a reconnect.

## Conclusion

In conclusion, streamlog represents a paradigm shift in the log-shipping
landscape, empowering teams to unlock the full potential of their logs.
=============== END FILE ===============

=============== FILE: src/config.ts ===============
export interface Config {
  /** TCP port the collector listens on. */
  port: number;
  /** Lines per outbound batch. */
  batchSize: number;
  /** Maximum time a partial batch waits before it is flushed. */
  flushMs: number;
  /** Delivery attempts before a batch is parked in spoolDir. */
  maxRetries: number;
  /** Where parked batches are written. */
  spoolDir: string;
}

export const DEFAULTS: Config = {
  port: 5140,
  batchSize: 1000,
  flushMs: 250,
  maxRetries: 8,
  spoolDir: "/var/lib/streamlog",
};
=============== END FILE ===============
