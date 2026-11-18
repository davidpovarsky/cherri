# Inline native metadata regression repair

The integrated full-Go run at3b9b6d8 failed TestNATIVE_Ref_ChainedMultiLevelAggrandizements: a three-item metadata chain inserted3 Dictionary producers ahead of the explicit native action. The old unchecked positional assertion then panicked, so later root-package tests were not executed. Other packages and the Python suite passed; this is not a full-Go PASS.

The authored-List repair had accidentally been applied to direct native.ref aggrandizements and native.plist array literals. Causal tests reproduced4 emitted actions instead of1 for direct/wrapped aggrandizements,3 instead of1 for a nested raw plist array, and silent dropping of malformed aggrandizements. An existing map-wrapped control already passed.

The repair reuses lowerListExpression(...,false), the existing inline Dictionary-array path, for native.plist data and reference metadata. Ordinary authored Lists still materialize typed Dictionary producers. Unknown metadata keys, sequence, nested arrays, Boolean values and original OutputUUID are preserved. Malformed aggrandizement containers/items now fail explicitly instead of disappearing. No new serializer or invented native metadata is introduced. The old regression assertion now checks exact action count/identity before accessing parameters.

Focused production verification:122 pass events, zero failures,52.437s across native transport, authored Dictionary/List, import/hybrid and edit-isolation tests. Raw test log SHA256dfbf2365662dd8cc9c3de0aa0d16d531a0a487c56d327a305586cb448e619283. Full root suite and latest private corpus will be refreshed after integration; native metadata transport is not Apple action execution evidence.
