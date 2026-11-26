# Startup observer event-progress control

This checkpoint changes the standalone control's sampling contract. It does not
change the app observer, its queue, its timer cadence, or any UIKit/UI assertion.

The old control sampled after a nominal 120 ms caller sleep. The hosted bfe run
stopped at that control before Go, Xcode or XCTest. A setup barrier and then a
scoped Foundation precision request did not qualify the original assertion.
The retained precision03 observation is especially important: a nominal 120 ms
sleep actually blocked for 246.8 ms, and its two events occurred after 120 ms.
Those original failures remain failures.

The qualified control starts the same observer with its existing 300 ms
lifetime and 20 ms cadence, observes only real timer records, and blocks the
caller without pumping the main queue. It retains the original minimum of two
off-main heartbeats and rejects any accepted event outside the original
lifetime. The file reader has a bounded input and is drained before the final
immutable observation. The outer command still has its original 10 s limit.

Foundation04 passed the complete standalone control (83 actual assertions).
Its two genuine off-main events were at 171 and 258 ms after observer-opened;
the caller was actually blocked for 268.345 ms. This proves the declared
300 ms event-progress case for that execution, not a 120 ms timer-delivery
guarantee or a general host scheduling guarantee.

Four explicitly injected controls were rejected for their intended reasons:
no callbacks (zero), one actual callback (one), actual main-queue delivery
while the caller is blocked (zero), and a reported-main-thread field injection
(two records rejected by the unchanged off-main assertion). The last control
is a field injection, not a claim that those callbacks actually ran on main.
The isolated injected source is proof material and is not an app source file.

The 62 Foundation04 originals, all nine process receipts, prior failed
attempt archives, independent source review, exact source patch, and intake
are retained here. Every child was reaped and its owned process group absent.
The production observer remains SHA256
f25ac92a076db857e130abb39e391981949a285bd95cd781cdd92db6a843c09b.
The qualified standalone source is SHA256
8d5b9fe6aef52d95abc121943c1f77fa16f818a03380691b35a96cabdb176131.

This checkpoint gives no native/UI or final179 acceptance. The latest actual
fixed-three component run remains two passed and one failed; the next
source-bound diagnostic must still reach that failure. All original 89 UI/
component tests, the three new component methods, their assertions and
deadlines, the workflow and R23 review are unchanged by this checkpoint.

The original source patch is stored losslessly as source.original.patch.gz;
its decompressed SHA256 is 9d11e082669766e9061797f7942597afb88e8869c6f851e7cf02177053156273.
