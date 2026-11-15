# P10R12 Control Region Coverage (Whole Balanced Regions vs Native Markers)

**Generated:** `2026-11-15T05:11:58Z`  
**Compiler Binary SHA-256:** `7407548dd6a768c86414ff1495a14d3cfb2ee669bd1e6bd72a08e59a01fe8bad`  
**Dialect Parity:** Legacy and V2 control-region counts and classifications are **100% identical**.

---

## 1. Whole Balanced Regions vs Native Action Markers

| Family | Whole Regions Natural (Top / Nested) | Whole Regions Fallback (Top / Nested) | Mixed Internal | Natural Markers (`m0 + m1 + m2`) | Fallback Markers | Source Invalid |
|---|---:|---:|---:|---:|---:|---:|
| `conditional` | **3,300** (382 / 2,918) | 226 (17 / 209) | 1 (top, in `035`) | **8,296** (`3,300+1,696+3,300`) | 584 (`227+130+227`) | 1 (`035:723`) |
| `choosefrommenu` | **163** (2 / 161) | 24 (0 / 24) | 0 | **868** (`163+542+163`) | 108 (`24+60+24`) | 0 |
| `repeat.each` | **230** (9 / 221) | 0 | 0 | **460** (`230+0+230`) | 0 | 0 |
| `repeat.count` | **127** (12 / 115) | 0 | 0 | **254** (`127+0+127`) | 0 | 0 |
| **TOTAL** | **3,820 / 4,071 (93.83%)** | **250** | **1** | **9,878** | **692** | **1** |

- **`totalUnclosedRegions = 0`**, **`totalOrphanMarkers = 0`**.
- All **`repeat.each` (`230/230`)** and **`repeat.count` (`127/127`)** complete regions across all 38 shortcuts are promoted atomically with zero remaining loop fallbacks.
- `CONTROL_REGION` action markers dropped from **`10,160`** in P10R11 to **`692`** in P10R12 (`-93.19%`).
- Specimen `035` preserves orphan `End If` marker `035:723` as `SOURCE_INVALID` (`1` action) and records `mixedRegions = 1` where one inner conditional fallback pair inside an outer region stays raw without corrupting region balance (`0` unclosed regions).

---

## 2. Remaining `CONTROL_REGION` Fallback Reasons (250 Complete Regions + 1 Mixed Pair = 692 Markers)

| Rejection Reason | Whole Regions |
|---|---:|
| `conditional|CONTROL_REGION:cond(100)` | 72 |
| `conditional|CONTROL_REGION:cond(4)` | 56 |
| `conditional|CONTROL_REGION:cond(101)` | 36 |
| `choosefrommenu|CONTROL_REGION:non_string_menu_items` | 23 |
| `conditional|CONTROL_REGION:cond(2)` | 17 |
| `conditional|CONTROL_REGION:cond(99)` | 15 |
| `conditional|CONTROL_REGION:cond(3)` | 7 |
| `conditional|CONTROL_REGION:cond(999)` | 7 |
| `conditional|CONTROL_REGION:cond(5)` | 5 |
| `conditional|CONTROL_REGION:cond(0)` | 3 |
| `conditional|CONTROL_REGION:cond(1001)` | 3 |
| `conditional|CONTROL_REGION:multi_cond(2)` | 3 |
| `conditional|CONTROL_REGION:cond(1)` | 2 |
| `choosefrommenu|CONTROL_REGION:items(4)` | 1 |
