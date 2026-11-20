# T04 opaque Optional provenance — 2026-11-20

The old T04 positive source let value: AnyContent? = Clipboard, followed by show(value), contradicts the established compiler-owned Optional presence boundary. AnyContent describes opaque content; it supplies no independent presence fact. The immutable T04 contract requires Unknown, AnyContent and Void to remain distinct, with no false type guarantees. It does not authorize inventing presence for native content.

## Original and repaired evidence

The original clean fd691b96 T04 execution remains retained:40 named passes and one parent failure at the old positive. The original raw SHA256 is b3bbe3b8f671cd6dabb8f4561ec4ca28543e37aec20d9ad24e8b53004ae51ebe. Separate actual fd CLI check and compile observations reject that exact unchanged source with E_TARGET_VALUE_SHAPE at Clipboard1:26–1:35 and emit no shortcut.

Test-only2afeb8fbcb60cb44c03c2ae37feda74d8f057a4d moves that identical source into the shared named opaque_optional_binding negatives. Both production and service now require its exact code/span; production requires no native bytes and rejects a deliberately shifted diagnostic span. The old category-separation, plain AnyContent Clipboard, Unknown and Void checks are unchanged. Supported Optional positive cases use source-owned false,zero and empty Text; existing None remains covered.

The complete clean selection of TestP15T04OpaqueTypeProduction, TestP15T04OpaqueTypeService and TestP15OptionalForwardingRequiresOwnedPresence passed56 named events, no failures/skips, exit0 in10.453 seconds. Source was clean before/after. Draft56-event evidence is retained separately. No production guard, immutable contract or native expectation was weakened. No full179/BRG33 or Apple runtime completion is asserted from this component.

## Observation and transport qualifications

The11 CLI cases are exploratory observations, not an11-case asserted pass suite. Their original receipt's outputExists refers only to the requested result.shortcut path. Supported compilations actually wrote source-derived case.shortcut and case_unsigned.shortcut files; the append-only clarification records their exact bytes and hashes. No absence/failure verdict follows from that path mismatch. The exploratory Unknown? assumption was also corrected: current NewOptional intentionally keeps Unknown, giving no Optional presence proof.

A read-only notification request returned an internal transport error after the clean test had completed. Direct host state and original process-receipt reads recovered the result; the raw bytes were then reconciled without rerunning the test.
