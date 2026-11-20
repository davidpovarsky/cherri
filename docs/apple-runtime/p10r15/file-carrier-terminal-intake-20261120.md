# Terminal File carrier controls — no native type/byte acceptance

Source b29d071e44a9a8be5f3c22e7b09d5fe3965f8716, original run 44644166680.

Text-to-public.data failed target signing. Its archived targetProvenance is null. The first strict intake crashed while dereferencing that absent object; the original crash log is retained. Reader fix b6b7256062880e9ab9a5b03071687662698e9765 adds a type/presence guard before dereference. The same unchanged archive now produces structured FAIL_EVIDENCE_VALIDATION, exit 3, with no runtime credit.

File-coercion obtained a signed AEA1 artifact but failed import. Pre-import file routes and the localhost import request timed out. The app was foreground at editor entry and libraryLostBeforeQuery was false; this result cannot be attributed to the prior lost-foreground condition. No target callback was observed. The actual strict reader preserves FAIL_RUNTIME_PRESERVED, exit 2, with all 21 published hashes verified. No carrier type, byte, count or name result is available.

Dictionary-to-public.data ended cancelled without a publication artifact. The actor and cancellation cause remain unverified. The original log and separate bounded recovery follow-up are retained.

The JSON proof binds both original ZIPs, genuine terminal metadata, strict reader results and log digests. Clean replay of the new reader reproduced both expected failures. It does not normalize or repair the native evidence and grants no requirement or macro completion.
