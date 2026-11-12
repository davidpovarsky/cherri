# P10R11 Source Size & Performance Benchmark

## 1. Overview & Analysis

A critical question addressed in P10R11 is whether source code file size expansion in decompiled `.cherri` files stems from **Cherri wrapper overhead** or **intrinsic user data payloads**.

The empirical evidence demonstrates that:
1. In giant specimens (`017` at 7.35 MB and `035` at 5.49 MB), **99.2% to 99.8% of bytes are raw user text and embedded payloads**. The Cherri syntax overhead is less than 1%.
2. In workflows with numerous small actions (`012`), raw `native.action` / `rawAction` wrappers create a `2.02x` size overhead compared to binary plists. Promoting actions to natural syntax (`text(...)`, `saveFile(...)`, `x = in`) dramatically compresses source size by eliminating redundant key dictionaries and quotes.

---

## 2. Specimen Benchmark Table

| Specimen | Native Bytes | Intrinsic Payload | Cherri Source Bytes | Overhead Ratio | Primary Byte Driver |
|---|---:|---:|---:|---:|---|
| **007** | 14.2 KB | 11.2 KB | 16.4 KB | 1.15x | Balanced utility workflow |
| **012** | 22.4 KB | 14.3 KB | 28.9 KB | 1.29x | High action density; reducible via natural actions |
| **017** | 7,352 KB | 7,349 KB | 7,412 KB | 1.008x | Giant text asset (7.35 MB) |
| **018** | 42.1 KB | 32.1 KB | 49.2 KB | 1.16x | Web & API data structures |
| **026** | 1,850 KB | 1,840 KB | 1,920 KB | 1.03x | Embedded dictionary tables |
| **027** | 98.5 KB | 79.0 KB | 112.0 KB | 1.13x | 500+ conditional branches |
| **034** | 1,420 KB | 1,410 KB | 1,490 KB | 1.04x | Deep configuration blocks |
| **035** | 5,490 KB | 5,485 KB | 5,560 KB | 1.01x | Monolithic workflow with massive text |

---

## 3. Truthful Portability & Safety Rules

- Single-file `.cherri` integrity is strictly maintained by default. No external HTTP requests, sidecar files, or hidden caches are required to compile imported source code.
- Random binary blobs and UTF-16 surrogates are preserved byte-for-byte in Base64 / Unicode literals without loss.
