the 2.4.0 changelog entry reads like chatgpt wrote it. fix it, keep the facts. edit CHANGELOG.md in place

=============== FILE: CHANGELOG.md ===============
# Changelog

## 2.4.0 (2026-09-28)

We're thrilled to announce streamlog 2.4.0, a game-changing release that
takes your logging experience to the next level! 🚀

In today's fast-paced world, reliability is more important than ever. That's
why we've completely reimagined how streamlog handles backpressure, ensuring a
seamless and robust experience for all your mission-critical workloads.

### ✨ Highlights

- **Smarter Backpressure:** streamlog now pauses reads when the outbound queue
  passes 80% of `queue.max`, instead of dropping batches. This empowers you to
  ship logs with confidence!
- **Blazing-Fast Compression:** zstd is now the default codec, cutting
  bandwidth by roughly 35% in our benchmarks compared to gzip.
- **Breaking:** the `--legacy-ack` flag has been removed. It's worth noting
  that this flag was deprecated in 2.1.0.

### 🙏 In Conclusion

We can't wait to see what you build with streamlog 2.4.0. As always, your
feedback is invaluable to us. Happy logging!

## 2.3.1 (2026-08-14)

- Fix a crash when `queue.max` is 0.
=============== END FILE ===============
