# RP29 resource authorization execution

At clean source 8d99dc4261e367017087ef591d4bd5c81f2e3837, two real production-path
tests passed all eight named events without failures or skips in 5.687 seconds.
This is resource security and service/compiler evidence, not complete RP29
acceptance: the independent strict large-asset benchmark remains required.

The scenarios cover allowed text and binary with exact emitted bytes, a missing
resource, an existing parent-directory escape, existing outside targets reached
through file and directory symlinks, and a formerly authorized cached path
replaced by an outside symlink. Each negative exercises both service analysis
and canonical compilation, checks the exact workspace diagnostic, rejects
output bytes and content reads/encoding, and recovers to an allowed resource.
The cached-path scenario then restores changed legitimate bytes. All outside
witnesses are public synthetic files created inside the test's owned temporary
parent; no private host resource is used.

The filesystem really created both kinds of symlink on the observed Windows
host. Unsupported hosts explicitly skip that unexercised case; formal intake
must reject any such skip, never treat it as proof of the symlink boundary.
Existing S03/E08 tests remain the source for real large text/binary cache
measurements and their raw monotonic samples. Related requirement IDs receive
no result from this standalone security proof.

See the source-bound process receipt and raw Go JSONL under
[evidence](evidence/p15-rp29-resource-security-20261119/).
