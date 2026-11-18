# Exact source verification: Distance and native reference operands

Verified source: 1dc091734194491d030e5a0761536725cfeb55f2.

The clean run job_299382919f63 completed on 2026-11-18 between 23:15:26 and
23:16:05 UTC. It observed 217 production and 76 internal passing test events,
zero failed or skipped events in the positive commands, canonical schema
verification, and actual capabilities CLI success. Source was clean before
and after.

The production set includes the unchanged public CF05 regression, all 22
public control-region families, exact native reference metadata, opaque raw
result controls, T04/T09 strict authored typing, and prior Ask Number evidence.
The raw logs record actual named executions rather than inferred coverage.

The source preserves AnyContent for native.ref. Static shape is validated by
the same extracted kind/UUID/name and aggrandizement validators used in
lowering. The eight malformed service cases fail under the isolated unchecked
marker counterfactual (nine failed Go events including the parent), as required.
See counterfactual.json. These expected negative failures are separate from
positive test command results.

The actual CLI reported:

- Native schema fingerprint:
  7bfff2b8c3fddf3892e943ef45b79bbbc4cc984ad051a15099a5da0fc6fccc0b
- Compiler macro fingerprint:
  1fe7f7993e2473aec591341712f42c521f913974b8e84091a5c2950232230dec
- Codec fingerprint:
  3e7e3555eefe1e030d254df0a22c62fdc3a139f92e39da34f6d841f8bfa1689e
- 505 native actions.

The only generated registry change is getDistance's output from Unknown to
Number with raw output type number. The independently captured macOS 27 Apple
metadata identifies a single NSNumber output; its exact path/hash is in the
receipt. No iOS availability or runtime execution is inferred from that file.

The attempts.json file separates the original production failures, rejected
.number projection, disk/linker failure, fixture-setup corrections, reporter
failure, and final clean result. Original raw public logs are retained without
rewriting their bytes. Full capabilities output remains in the local artifact
directory; its measured SHA-256 is recorded in receipt.json rather than
duplicating the catalog here.

The independent read-only review found no field-dropping or validation
relaxation in the shared extraction. It identified the supported native.plist
value alias; the final source includes that alias and a passing service/compiler
control. The reviewer did not run these tests.

Scope: local compiler, analyzer, service, schema, and exact serialized native
graphs. No private corpus replay or Apple runtime test is claimed. The
integration owner performs the next complete 38-input replay on combined source.
