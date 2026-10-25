Fuzzing found an off-by-one in `RingBuffer` last quarter: `push` on a full
buffer overwrote the oldest item instead of returning false (the old code
checked `len > cap` instead of `len == cap`). It's fixed, but
fuzzing only samples. I want proof that `push` and `pop` never index out of
bounds and never lose or duplicate an item, for every capacity up to 8, and I
want it to stay proven in CI. Set this crate up for that.

======= FILE: Cargo.toml =======
[package]
name = "ringbuf-lite"
version = "0.3.1"
edition = "2021"

[dependencies]
======= END FILE =======

======= FILE: src/lib.rs =======
pub struct RingBuffer<T: Copy + Default> {
    buf: Vec<T>,
    head: usize,
    len: usize,
}

impl<T: Copy + Default> RingBuffer<T> {
    pub fn with_capacity(cap: usize) -> Self {
        assert!(cap > 0);
        Self { buf: vec![T::default(); cap], head: 0, len: 0 }
    }

    pub fn capacity(&self) -> usize {
        self.buf.len()
    }

    pub fn len(&self) -> usize {
        self.len
    }

    /// Returns false when full.
    pub fn push(&mut self, value: T) -> bool {
        if self.len == self.capacity() {
            return false;
        }
        let tail = (self.head + self.len) % self.capacity();
        self.buf[tail] = value;
        self.len += 1;
        true
    }

    pub fn pop(&mut self) -> Option<T> {
        if self.len == 0 {
            return None;
        }
        let value = self.buf[self.head];
        self.head = (self.head + 1) % self.capacity();
        self.len -= 1;
        Some(value)
    }
}
======= END FILE =======

======= FILE: .github/workflows/ci.yml =======
name: CI
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v5
      - run: cargo test
======= END FILE =======
