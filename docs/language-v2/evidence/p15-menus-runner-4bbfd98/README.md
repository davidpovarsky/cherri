# Menus differential runner asset repair

The new real-runner test reproduces the missing workspace asset against d7052f45, then passes after the public dependency is staged. The compiler production source is unchanged; only the differential harness changes.

The six top-level tests and three asset subtests pass. Original source and PNG bytes are checked unchanged; an unrelated working directory cannot replace the entry asset; removing it fails closed; restoring its exact bytes recovers. A missing canonical source asset is rejected. Existing comparator fault tests still pass.

The full baseline remains a recorded failure (9 exact, 35 equivalent, 2 differences, 1 compile error; 17 action controls pass). This checkpoint does not close globals, standard, full CI, Apple execution or formal179.

The independent migration baseline is a separate newly reproduced defect: malformed synthetic source is overwritten by migrate --write with exit 0. It is retained as a negative control, not attributed to the menus fix.
