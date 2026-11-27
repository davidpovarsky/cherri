"""One full regression on the exact current joined source; no native acceptance.
All original root/internal events and ordinary skips remain in complete streams.
The existing parallel-two scheduler is explicitly selected after the prior alarm.
"""
from pathlib import Path
import base64
import hashlib
import importlib.util
import json
import os
import platform
import re
import shutil
import stat
import sys
import traceback
import time

sys.dont_write_bytecode = True
SOURCE = "7abdf68d6717092f59a72319d39d087e33296dde"
TREE = "09abc4dd0b7fb7325122768708671569686ac6b8"
PARENTS = ("64924761ec203259bfde89983897e5977392149b", "a6d3b7bb785d8a0909fe325faf310e7bf20a8a7a")
BRANCH = "probe/cherri-p10r15-current-regression-7abdf6-20261127"
HELPER_SHA = "4672474ddb270a5857914041ebb279f20431f87e9da173e6cd8bf83b3fb11c85"
PREFIX = "tools/ci/distinct_functions/"
CONTROLLER_FILES = [".github/workflows/p10r15-distinct-functions.yml", PREFIX+"run.py", PREFIX+"owned_source_process.py"]
PACKAGE = "github.com/electrikmilk/cherri"
REGRESSION_COMMAND = ("go", "test", "-count=1", "-json", "-timeout=10m", "-parallel=2", ".", "./internal/...")
EXPECTED_PACKAGES = tuple(["github.com/electrikmilk/cherri","github.com/electrikmilk/cherri/internal/icloudshortcut","github.com/electrikmilk/cherri/internal/language/analysis","github.com/electrikmilk/cherri/internal/language/assets","github.com/electrikmilk/cherri/internal/language/backend","github.com/electrikmilk/cherri/internal/language/enums","github.com/electrikmilk/cherri/internal/language/ir","github.com/electrikmilk/cherri/internal/language/lower","github.com/electrikmilk/cherri/internal/language/lsp","github.com/electrikmilk/cherri/internal/language/migrate","github.com/electrikmilk/cherri/internal/language/modules","github.com/electrikmilk/cherri/internal/language/protocol","github.com/electrikmilk/cherri/internal/language/schema","github.com/electrikmilk/cherri/internal/language/service","github.com/electrikmilk/cherri/internal/language/source","github.com/electrikmilk/cherri/internal/language/syntax","github.com/electrikmilk/cherri/internal/language/types","github.com/electrikmilk/cherri/internal/language/workspace","github.com/electrikmilk/cherri/internal/shortcutcompare"])
NO_TEST_PACKAGES = frozenset(["github.com/electrikmilk/cherri/internal/language/enums"])
REQUIRED_PARENTS = tuple(tuple(x) for x in [["github.com/electrikmilk/cherri/internal/language/analysis","TestScopedBlockDiagnosticsAndBranchTypes"],["github.com/electrikmilk/cherri/internal/language/analysis","TestScopedBlockDoesNotReserveExistingFunctionName"],["github.com/electrikmilk/cherri/internal/language/analysis","TestScopedBlockInferredTypeAndLexicalScope"],["github.com/electrikmilk/cherri/internal/language/analysis","TestScopedMeasureStatementBodiesHaveExactVoidType"],["github.com/electrikmilk/cherri/internal/language/backend","TestScopedCachePreviousEmissionFailuresDoNotRunBody"],["github.com/electrikmilk/cherri/internal/language/backend","TestScopedCachePreviousSnapshotBoundary"],["github.com/electrikmilk/cherri/internal/language/backend","TestScopedMeasureInputCaptureRestoreOrderAndFailures"],["github.com/electrikmilk/cherri/internal/language/backend","TestScopedMeasurePreservesTypedIdentityAndStopsOnProducerFailure"],["github.com/electrikmilk/cherri/internal/language/backend","TestScopedMeasureRejectsIncompleteInputBoundaryBeforeEffects"],["github.com/electrikmilk/cherri/internal/language/backend","TestScopedMeasureVoidNestedBoundaryOrder"],["github.com/electrikmilk/cherri/internal/language/backend","TestScopedMeasureVoidRunsOnceAndStopsAtFailedStage"],["github.com/electrikmilk/cherri/internal/language/lower","TestScopedBlockRenamingPreservesSourceAndLiteralValues"],["github.com/electrikmilk/cherri/internal/language/lower","TestScopedLoopCellsRejectCallAndBodyReplacementBeforeEffects"],["github.com/electrikmilk/cherri/internal/language/lower","TestScopedSyntaxRejectsAbsoluteNativeNamesBeforeScope"],["github.com/electrikmilk/cherri/internal/language/schema","TestScopedMeasureExactLifecycleSchemaGuard"],["github.com/electrikmilk/cherri/internal/language/schema","TestScopedMeasureLoopPlanKeepsDeclarationAndBodyIdentity"],["github.com/electrikmilk/cherri/internal/language/schema","TestScopedMeasureLoopPlanRetainsClosedBoundaries"],["github.com/electrikmilk/cherri/internal/language/schema","TestScopedMeasureLoopPlanVisitsOrdinaryNestedExpressions"],["github.com/electrikmilk/cherri/internal/language/syntax","TestScopedBlockDoesNotConsumeControlBodies"],["github.com/electrikmilk/cherri/internal/language/syntax","TestScopedBlockNestedArgumentHasOwnBoundary"],["github.com/electrikmilk/cherri/internal/language/syntax","TestScopedBlockParseFormatAndUnicode"],["github.com/electrikmilk/cherri/internal/shortcutcompare","TestFunctionBodyMutation_DetectedByNormalizedComparator"],["github.com/electrikmilk/cherri/internal/shortcutcompare","TestFunctionNoContentEpilogueNormalization"],["github.com/electrikmilk/cherri","TestBackendRecovery14ValueBlockReturnNative"],["github.com/electrikmilk/cherri","TestBackendRecovery14ValueBlockReturnUnit"],["github.com/electrikmilk/cherri","TestP15_CollectTypedBodyHasSingleNativeProducer"],["github.com/electrikmilk/cherri","TestP15_CollectUsesExistingRepeatResultsWithoutAccumulator"],["github.com/electrikmilk/cherri","TestP15_LegacyMeasureActionArgumentsRunOnce"],["github.com/electrikmilk/cherri","TestP15_LegacyMeasureCommentsAndPropertyReferences"],["github.com/electrikmilk/cherri","TestP15_LegacyMeasureControlScopesAndCapturedMutation"],["github.com/electrikmilk/cherri","TestP15_LegacyMeasureImportedHintsDoNotRetargetLocalShadow"],["github.com/electrikmilk/cherri","TestP15_LegacyMeasureLexicalBindings"],["github.com/electrikmilk/cherri","TestP15_LegacyMeasureNestedOnceAndDateSnapshot"],["github.com/electrikmilk/cherri","TestP15_LegacyMeasureRejectsUnprovenForms"],["github.com/electrikmilk/cherri","TestP15_LegacyMeasureStateAndDiagnostics"],["github.com/electrikmilk/cherri","TestP15_LegacyMeasureTypedIdentityAndProfiles"],["github.com/electrikmilk/cherri","TestP15_MeasureAndCollectReusableFixturesCompile"],["github.com/electrikmilk/cherri","TestP15_MeasureFreezesCurrentDateBeforeStoppingTimer"],["github.com/electrikmilk/cherri","TestP15_MeasureNestedSameLabelRunsBodyOnce"],["github.com/electrikmilk/cherri","TestP15_MeasurePreservesOriginalDictionaryProducer"],["github.com/electrikmilk/cherri","TestP15_MeasureRejectsUnprovenForms"],["github.com/electrikmilk/cherri","TestP15_MeasureShadowedNativeStorageDoesNotOverwriteOuter"],["github.com/electrikmilk/cherri","TestP15_MeasureWaitAndNativeControlResults"],["github.com/electrikmilk/cherri","TestP15CacheCapturedWritesAndShadowing"],["github.com/electrikmilk/cherri","TestP15CacheExistingScopedValueControl"],["github.com/electrikmilk/cherri","TestP15CacheNativeConditionEncoding"],["github.com/electrikmilk/cherri","TestP15CachePreviousBaselineOracleFaults"],["github.com/electrikmilk/cherri","TestP15CachePreviousMatchesAuthoredBaseline"],["github.com/electrikmilk/cherri","TestP15CachePreviousSnapshotOracleFaults"],["github.com/electrikmilk/cherri","TestP15CacheRejectsUnqualifiedPersistence"],["github.com/electrikmilk/cherri","TestP15CacheScalarLazyGraphs"],["github.com/electrikmilk/cherri","TestP15CacheVersionAndLiteralBoundaries"],["github.com/electrikmilk/cherri","TestP15CacheZeroTTLBypassesPersistence"],["github.com/electrikmilk/cherri","TestP15F04ValueLoopBoundaryNative"],["github.com/electrikmilk/cherri","TestP15F04ValueLoopBoundaryUnit"],["github.com/electrikmilk/cherri","TestP15M02CompleteGraphCausalControls"],["github.com/electrikmilk/cherri","TestP15M02CompleteMainGraphs"],["github.com/electrikmilk/cherri","TestP15M02ExactNativeWorkflowBoundary"],["github.com/electrikmilk/cherri","TestP15M02ExplicitMainReturnBaseline"],["github.com/electrikmilk/cherri","TestP15M02MainVoidFallthrough"],["github.com/electrikmilk/cherri","TestP15M02MigratedEntryOutput"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousOracleRejectsCausalFaults"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations"],["github.com/electrikmilk/cherri","TestP15MeasureVoidLoopBoundaryGraphs"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs"],["github.com/electrikmilk/cherri","TestP15MeasureVoidPreservesPrevious"],["github.com/electrikmilk/cherri","TestP15MeasureVoidRejectsConsumptionAndImplicitControlValues"],["github.com/electrikmilk/cherri","TestP15MeasureVoidRetainsExistingValueControl"],["github.com/electrikmilk/cherri","TestP15NoneBindingsClearPreviousOutput"],["github.com/electrikmilk/cherri","TestP15NoneClearingPreservesPresentEmptyValues"],["github.com/electrikmilk/cherri","TestP15NoneClearOracleRejectsCorruption"],["github.com/electrikmilk/cherri","TestP15NoneReturnClearsPreviousOutput"],["github.com/electrikmilk/cherri","TestP15OptionalCallResultPredicateEvaluatesOnce"],["github.com/electrikmilk/cherri","TestP15OptionalNonePredicateOracleRejectsCorruption"],["github.com/electrikmilk/cherri","TestP15OptionalNonePredicatesUsePresence"],["github.com/electrikmilk/cherri","TestP15OptionalReturnCompleteness"],["github.com/electrikmilk/cherri","TestP15RP23ValueBlockLoopNative"],["github.com/electrikmilk/cherri","TestP15RP23ValueBlockLoopUnit"],["github.com/electrikmilk/cherri","TestP15RP26CallerInputVoidNative"],["github.com/electrikmilk/cherri","TestP15RP26PartialMarkerInputBoundaries"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportComposition"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportGraphFaults"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportLexicalStorage"],["github.com/electrikmilk/cherri","TestP15TaggedNoneResultRejectsStaleAndInventedValues"]])
REQUIRED_SCENARIO_CHILDREN = tuple(tuple(x) for x in [["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/date_input/legacy=false/debug"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/date_input/legacy=false/release"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/date_input/legacy=true/debug"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/date_input/legacy=true/release"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/entry_input/legacy=false/debug"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/entry_input/legacy=false/release"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/entry_input/legacy=true/debug"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/entry_input/legacy=true/release"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/debug_previous/legacy=false/debug"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/debug_previous/legacy=false/release"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/debug_previous/legacy=true/debug"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/debug_previous/legacy=true/release"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/body_owned_previous/legacy=false/debug"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/body_owned_previous/legacy=false/release"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/body_owned_previous/legacy=true/debug"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/body_owned_previous/legacy=true/release"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/nested_date_input/legacy=false/debug"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/nested_date_input/legacy=false/release"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/nested_date_input/legacy=true/debug"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/nested_date_input/legacy=true/release"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/nested_log/legacy=false/debug"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/nested_log/legacy=false/release"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/nested_log/legacy=true/debug"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/nested_log/legacy=true/release"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/empty_text/legacy=false/debug"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/empty_text/legacy=false/release"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/empty_text/legacy=true/debug"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/empty_text/legacy=true/release"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/zero_number/legacy=false/debug"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/zero_number/legacy=false/release"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/zero_number/legacy=true/debug"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/zero_number/legacy=true/release"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/no_content_date/legacy=false/debug"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/no_content_date/legacy=false/release"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/no_content_date/legacy=true/debug"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/no_content_date/legacy=true/release"],["github.com/electrikmilk/cherri","TestP15MeasureVoidPreservesPrevious/nested=false/legacy=false/debug"],["github.com/electrikmilk/cherri","TestP15MeasureVoidPreservesPrevious/nested=false/legacy=false/release"],["github.com/electrikmilk/cherri","TestP15MeasureVoidPreservesPrevious/nested=false/legacy=true/debug"],["github.com/electrikmilk/cherri","TestP15MeasureVoidPreservesPrevious/nested=false/legacy=true/release"],["github.com/electrikmilk/cherri","TestP15MeasureVoidPreservesPrevious/nested=true/legacy=false/debug"],["github.com/electrikmilk/cherri","TestP15MeasureVoidPreservesPrevious/nested=true/legacy=false/release"],["github.com/electrikmilk/cherri","TestP15MeasureVoidPreservesPrevious/nested=true/legacy=true/debug"],["github.com/electrikmilk/cherri","TestP15MeasureVoidPreservesPrevious/nested=true/legacy=true/release"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousOracleRejectsCausalFaults/legacy=false/explicit_capture"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousOracleRejectsCausalFaults/legacy=false/capture_after_timer"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousOracleRejectsCausalFaults/legacy=false/wrong_restore_slot"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousOracleRejectsCausalFaults/legacy=false/coerced_restore"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousOracleRejectsCausalFaults/legacy=false/removed_consumer"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousOracleRejectsCausalFaults/legacy=true/explicit_capture"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousOracleRejectsCausalFaults/legacy=true/capture_after_timer"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousOracleRejectsCausalFaults/legacy=true/wrong_restore_slot"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousOracleRejectsCausalFaults/legacy=true/coerced_restore"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousOracleRejectsCausalFaults/legacy=true/removed_consumer"],["github.com/electrikmilk/cherri/internal/language/backend","TestScopedMeasureInputCaptureRestoreOrderAndFailures/value/"],["github.com/electrikmilk/cherri/internal/language/backend","TestScopedMeasureInputCaptureRestoreOrderAndFailures/value/capture"],["github.com/electrikmilk/cherri/internal/language/backend","TestScopedMeasureInputCaptureRestoreOrderAndFailures/value/debugStartTimer"],["github.com/electrikmilk/cherri/internal/language/backend","TestScopedMeasureInputCaptureRestoreOrderAndFailures/value/restore"],["github.com/electrikmilk/cherri/internal/language/backend","TestScopedMeasureInputCaptureRestoreOrderAndFailures/value/body"],["github.com/electrikmilk/cherri/internal/language/backend","TestScopedMeasureInputCaptureRestoreOrderAndFailures/value/debugStopTimer"],["github.com/electrikmilk/cherri/internal/language/backend","TestScopedMeasureInputCaptureRestoreOrderAndFailures/void/"],["github.com/electrikmilk/cherri/internal/language/backend","TestScopedMeasureInputCaptureRestoreOrderAndFailures/void/capture"],["github.com/electrikmilk/cherri/internal/language/backend","TestScopedMeasureInputCaptureRestoreOrderAndFailures/void/debugStartTimer"],["github.com/electrikmilk/cherri/internal/language/backend","TestScopedMeasureInputCaptureRestoreOrderAndFailures/void/restore"],["github.com/electrikmilk/cherri/internal/language/backend","TestScopedMeasureInputCaptureRestoreOrderAndFailures/void/body"],["github.com/electrikmilk/cherri/internal/language/backend","TestScopedMeasureInputCaptureRestoreOrderAndFailures/void/debugStopTimer"],["github.com/electrikmilk/cherri/internal/shortcutcompare","TestFunctionNoContentEpilogueNormalization/omitted_parameters"],["github.com/electrikmilk/cherri/internal/shortcutcompare","TestFunctionNoContentEpilogueNormalization/empty_parameter_maps"],["github.com/electrikmilk/cherri/internal/shortcutcompare","TestFunctionNoContentEpilogueNormalization/unpaired_exit_is_observable"],["github.com/electrikmilk/cherri/internal/shortcutcompare","TestFunctionNoContentEpilogueNormalization/nonterminal_exit_is_observable"],["github.com/electrikmilk/cherri/internal/shortcutcompare","TestFunctionNoContentEpilogueNormalization/branch_local_exit_is_observable"],["github.com/electrikmilk/cherri/internal/shortcutcompare","TestFunctionNoContentEpilogueNormalization/main_frame_exit_is_observable"],["github.com/electrikmilk/cherri/internal/shortcutcompare","TestFunctionNoContentEpilogueNormalization/no_function_envelope_exit_is_observable"],["github.com/electrikmilk/cherri/internal/shortcutcompare","TestFunctionNoContentEpilogueNormalization/only_one_terminal_pair_is_removed"],["github.com/electrikmilk/cherri/internal/shortcutcompare","TestFunctionNoContentEpilogueNormalization/exit_null_parameters"],["github.com/electrikmilk/cherri/internal/shortcutcompare","TestFunctionNoContentEpilogueNormalization/exit_scalar_parameters"],["github.com/electrikmilk/cherri/internal/shortcutcompare","TestFunctionNoContentEpilogueNormalization/exit_list_parameters"],["github.com/electrikmilk/cherri/internal/shortcutcompare","TestFunctionNoContentEpilogueNormalization/exit_explicit_input"],["github.com/electrikmilk/cherri/internal/shortcutcompare","TestFunctionNoContentEpilogueNormalization/exit_explicit_empty_output"],["github.com/electrikmilk/cherri/internal/shortcutcompare","TestFunctionNoContentEpilogueNormalization/nothing_null_parameters"],["github.com/electrikmilk/cherri/internal/shortcutcompare","TestFunctionNoContentEpilogueNormalization/nothing_scalar_parameters"],["github.com/electrikmilk/cherri/internal/shortcutcompare","TestFunctionNoContentEpilogueNormalization/nothing_explicit_input"],["github.com/electrikmilk/cherri/internal/shortcutcompare","TestFunctionNoContentEpilogueNormalization/extra_action_metadata_is_observable"],["github.com/electrikmilk/cherri/internal/shortcutcompare","TestFunctionNoContentEpilogueNormalization/value_return_is_preserved"],["github.com/electrikmilk/cherri/internal/shortcutcompare","TestFunctionNoContentEpilogueNormalization/body_mutation_is_preserved"],["github.com/electrikmilk/cherri/internal/shortcutcompare","TestFunctionBodyMutation_DetectedByNormalizedComparator/greetDefault_default_value"],["github.com/electrikmilk/cherri/internal/shortcutcompare","TestFunctionBodyMutation_DetectedByNormalizedComparator/double_body_math_operator"],["github.com/electrikmilk/cherri/internal/shortcutcompare","TestFunctionBodyMutation_DetectedByNormalizedComparator/fibonacci_body_condition_threshold"]])
REQUIRED_IDENTITIES = REQUIRED_PARENTS + REQUIRED_SCENARIO_CHILDREN
SOURCE_PINS = {
    "build_profile.go": "c2fcfa5f7a46eda8a16ce3fe7ea8aa6ced74191b38bdb8019aa9623d53aa60ae",
    "internal/language/analysis/scoped_blocks_test.go": "caae50e35bbe840daa8e81a5b6c04282b14976bf73708c87d77c794cfc5d5783",
    "internal/language/analysis/scoped_blocks_void_test.go": "223848d594d2dfd67f9b9aa5d354307022ad1025a125d8b48dc19ac6f4a7e1ad",
    "internal/language/backend/contracts.go": "fde3e1c72b29b8d8bbf706b3c856f468f6be64c5d091d87321189a5130283e33",
    "internal/language/backend/scoped_cache_previous_test.go": "f44b843e3326bf6c964816acd295ffe896b1370d12ac7c3b2f10599facddef32",
    "internal/language/backend/scoped_measure.go": "d3a4dc25954bd147beb85a906a78aa04157e3bafba60748d6f7f9c71d2bb713c",
    "internal/language/backend/scoped_measure_input_test.go": "0f0856fa0a7fe72c064dbd2175f6179a7531e762a4578de1dec35be3c3661fad",
    "internal/language/backend/scoped_measure_test.go": "f454a81a1a16d3643e984e2fb894e5c459fb3eb928a795fe37b6db2c05193624",
    "internal/language/backend/scoped_measure_void_test.go": "f59c4291599de39d85b2a666647da4a67f5e7c9625e2c70e22035dbd3b945524",
    "internal/language/lower/lower.go": "4a28c73289d811282aac7be7bdea5d06f50e51486d951ea685f4064fe77c9eef",
    "internal/language/lower/lower_test.go": "1890a8692fab2b7a8609e449635c982a0d0330331cd75f739439494080e93579",
    "internal/language/lower/main_output.go": "41721975525ff359d3880c06d796aff98663c1b91f42e050fcb8625f50350921",
    "internal/language/lower/scoped_blocks.go": "0a7286724bc23b55236e779c82a6abf273f53330ed8a7963324a95cc4f02c7db",
    "internal/language/lower/scoped_blocks_test.go": "05838d0ed4b08707d126491e3391c3c561b18bde19b76242a140a35ed69e2fa2",
    "internal/language/lower/scoped_value_identity_test.go": "d9a8408b81d63a0171cb6aabf7557ca1cd28f809c2e4423cf9a154723756679d",
    "internal/language/schema/scoped_loop_result_test.go": "124bd195e6711af50b15e10a6dbedbc8fc9c0cc5f6e20ba9a7f3bb5f3163f7e3",
    "internal/language/syntax/scoped_blocks_test.go": "6fe2f1d013f63b60e86e16415c8e0dcccf14889f10ac932e19b5cb23f2815a7c",
    "internal/shortcutcompare/functions_regression_test.go": "b195b626e5593932170fdd194bcd5e550b460bd39c921e1c5dd8ec7ce42256fd",
    "internal/shortcutcompare/no_content_epilogue_test.go": "8689b26658751f4bdcf4e0d67b7ff25d98da36ea8f4e355050dcdf369bed7b30",
    "internal/shortcutcompare/normalize.go": "360ca60c0363381f314952e0e7c84ad049c722dd3420694985709fd2677cafd4",
    "ios/CherriApp/Diagnostics/EditorDeliveryObservation.swift": "6840649ef998ded3056d14767478255b4dc21c34d05731e9e771cb101035acd1",
    "ios/CherriUITests/CherriJoinedContractUITests.swift": "f83fe2b529d1fffb9900889b9cf5b8b84442ded509de6dec9f138bb4fe687b6f",
    "language_backend_adapter.go": "a095d66cce83cdae7815092932ba1b875af60d6b5f9893890f5233eb18a99d5f",
    "p15_cache_previous_faults_test.go": "77e4520b225285e0909e5991e162bb0c67c5b161dacb984a4a3051f4cbdd97d1",
    "p15_cache_previous_test.go": "6351f2e5f06b8709f85248af1aee15c03b34db216c0e72f08b6cbba844c10643",
    "p15_cache_ttl_captures_test.go": "822129f13e7a8c06aa6883c9e8ae1918830a2f516dc9130a463c5ba76ae5be04",
    "p15_cache_ttl_contract_test.go": "0a1de8d4855eb358297383b06da63617c026aac12511e2d739c875b5dbb2f9e5",
    "p15_cache_ttl_test.go": "eed602931c9c107fab194f3a206f154328a5b2f4396ce433add591306986dbd1",
    "p15_combined_value_blocks_test.go": "ed6f4ba43d546807e2ab9558163309a89cc30debd1f760611eb9a0bb0c64f112",
    "p15_function_optional_predicate_test.go": "3023245cabdc0088d962fc6dfae174c4a7a024e2df0697d83b0b71eb437c1f75",
    "p15_install_actual_tools_test.go": "f643c0ec301bd8bae257e0f0d0ff71cef2fb8b6fb0847f6fda0597ec9803ab58",
    "p15_main_output_graph_test.go": "af6dfa89af76970505122e47ee09a16851d49de3559ec441507aae90355bf0d3",
    "p15_main_void_return_test.go": "bb493466658026ce3378ae2d442daca96e9d261468210c3c2c886b49c91cab0f",
    "p15_measure_previous_test.go": "846e933ce55f8028fee12ff9df2e0f5c145e262e36f5c993cda3571e3dd2ad98",
    "p15_measure_void_controls_test.go": "b2a105fd4a2c915e768f977b1ea7b186902a4be9004205130423134547f6cadb",
    "p15_measure_void_test.go": "1a1360c5c12c4e48e48ff3e8001b6ca77689f11dc4dd5596dd9bec977f1c8523",
    "p15_none_clear_boundary_test.go": "2312222a2495c673d6053182a19c35c719bbcb3404c7fb7b3631457bd8bef0f6",
    "p15_rp26_caller_input_void_test.go": "982b9e94e4870fd40b08f378ba3b4a8e48f195259b449d0efe6a391069d9e363",
    "p15_scoped_blocks_legacy_test.go": "b255796452e49fc3854ffc9a24d9b70bca1f3b386583a3ba7fc63ec21374f7d8",
    "p15_scoped_blocks_test.go": "9eefec5a98309c2f3825087a14e0a7e905398ba0bf573f1f5af6afcb73800ca5",
    "p15_scoped_value_transport_test.go": "5a117f5b45d999b5b074431f3fab6f6e0d10f85d9c40d497f6f7b88f122b5ec9",
    "scoped_blocks_legacy.go": "4061b8f21a7aaffbae14c6f56f774586286a2a027235da1c0df6a03c3ff03d72",
    "tools/language-ai-eval/README.md": "b6e3121c69e3684b61a108e7f03068ab4d0ed16c2c73199b038dc9fd3f1c3fb1"
}
PRODUCT_ADMISSION_SECONDS = 800


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def validate_go(raw, packages, required):
    events = [json.loads(line) for line in raw.decode("utf-8").splitlines() if line.strip()]
    require(events and all(type(e) is dict for e in events), "ORIGINAL_GO_EVENTS")
    require(not any(e.get("Action") in ("fail", "skip") for e in events), "FAILED_OR_SKIPPED_GO_EVENT")
    passed_packages = [e["Package"] for e in events if e.get("Action") == "pass" and not e.get("Test")]
    require(sorted(passed_packages) == sorted(packages), "EXACT_COMPLETE_PACKAGE_TERMINALS")
    passed = [(e["Package"], e["Test"]) for e in events if e.get("Action") == "pass" and e.get("Test")]
    require(len(passed) == len(set(passed)), "DUPLICATE_NAMED_TERMINAL")
    for package, name in required:
        require(passed.count((package, name)) == 1, "MISSING_REQUIRED_PARENT:"+name)
    return {"packages": passed_packages, "namedPasses": len(passed), "requiredParents": required,
            "failed": 0, "skipped": 0, "originalStreamSHA256": sha(raw)}

def inspect_regression_stream(path, packages, no_test_packages, required, tick=lambda: None):
    """Read the entire authentic Go 1.24 JSONL without filtering or truncation.

    Ordinary original test skips remain visible. Required affected parents must
    pass; a package with no test files is distinct from a skipped named test.
    """
    path = Path(path)
    before = path.lstat()
    require(stat.S_ISREG(before.st_mode), "REGULAR_ORIGINAL_GO_STREAM")
    allowed = set(packages)
    require(len(allowed) == len(packages) and set(no_test_packages) <= allowed,
            "CLOSED_ORIGINAL_PACKAGE_SET")
    require(len(required) == len(set(required)) and all(p in allowed for p, _ in required),
            "CLOSED_AFFECTED_PARENT_SET")
    starts, package_terminals, named, builds = {}, {}, {}, {}
    output_events = 0
    output_bytes = 0
    digest = hashlib.sha256()
    size = lines = 0
    def unique(pairs):
        result = {}
        for key, value in pairs:
            require(key not in result, "DUPLICATE_JSON_KEY")
            result[key] = value
        return result
    with path.open("rb") as stream:
        for raw in stream:
            tick()
            size += len(raw)
            digest.update(raw)
            require(raw.endswith(b"\n") and raw.strip(), "COMPLETE_NONEMPTY_JSON_LINE")
            event = json.loads(raw.decode("utf-8"), object_pairs_hook=unique)
            lines += 1
            require(type(event) is dict and type(event.get("Action")) is str, "GO_EVENT_OBJECT")
            action = event["Action"]
            if action in ("build-output", "build-fail"):
                identity = event.get("ImportPath")
                require(type(identity) is str and identity and not event.get("Test"),
                        "GO_124_BUILD_IDENTITY")
                state = builds.setdefault(identity, {"outputEvents": 0, "failed": False})
                if action == "build-output":
                    require(type(event.get("Output")) is str, "ORIGINAL_BUILD_OUTPUT")
                    state["outputEvents"] += 1
                else:
                    require(not state["failed"], "DUPLICATE_BUILD_FAILURE")
                    state["failed"] = True
                continue
            require(action in ("start", "run", "pause", "cont", "pass", "fail", "skip", "output"),
                    "KNOWN_TEST_EVENT")
            package = event.get("Package")
            require(package in allowed, "UNEXPECTED_TEST_PACKAGE")
            name = event.get("Test")
            require(name is None or (type(name) is str and bool(name)), "NAMED_TEST_IDENTITY")
            if "Elapsed" in event:
                value = event["Elapsed"]
                require(type(value) in (int, float) and value == value and
                        value >= 0 and value < float("inf"), "FINITE_EVENT_ELAPSED")
            if action == "output":
                require(type(event.get("Output")) is str, "ORIGINAL_TEST_OUTPUT")
                output_events += 1
                output_bytes += len(event["Output"].encode("utf-8"))
                continue
            if action == "start":
                require(name is None and package not in starts and package not in package_terminals,
                        "ONE_PACKAGE_START")
                starts[package] = lines
                continue
            require(package in starts and package not in package_terminals,
                    "PACKAGE_LIFETIME")
            if name is None:
                require(action in ("pass", "fail", "skip"), "PACKAGE_TERMINAL_ACTION")
                package_terminals[package] = action
                continue
            key = (package, name)
            if action == "run":
                require(key not in named, "ONE_NAMED_START")
                named[key] = {"runLine": lines, "terminal": None, "paused": False}
            elif action in ("pause", "cont"):
                require(key in named and named[key]["terminal"] is None, "RUNNING_NAMED_EVENT")
                require(named[key]["paused"] is (action == "cont"), "PAUSE_CONT_ORDER")
                named[key]["paused"] = action == "pause"
            else:
                require(action in ("pass", "fail", "skip") and key in named and
                        named[key]["terminal"] is None, "ONE_NAMED_TERMINAL")
                named[key]["terminal"] = action
                named[key]["terminalLine"] = lines
    after = path.lstat()
    require(all(getattr(before, key) == getattr(after, key) for key in
                ("st_dev", "st_ino", "st_mode", "st_uid", "st_gid", "st_size", "st_mtime_ns", "st_ctime_ns")),
            "UNCHANGED_COMPLETE_ORIGINAL_STREAM")
    require(lines > 0 and size == before.st_size, "ENTIRE_ORIGINAL_STREAM_READ")
    missing_packages = sorted(allowed - set(package_terminals))
    invalid_packages = {p: status for p, status in package_terminals.items()
                        if status != "pass" and not (status == "skip" and p in no_test_packages)}
    incomplete = sorted(key for key, value in named.items() if value["terminal"] is None)
    failed = sorted(key for key, value in named.items() if value["terminal"] == "fail")
    skipped = sorted(key for key, value in named.items() if value["terminal"] == "skip")
    mandatory_missing = sorted(key for key in required
                               if key not in named or named[key]["terminal"] != "pass")
    build_failures = sorted(key for key, value in builds.items() if value["failed"])
    complete = (set(starts) == allowed and not missing_packages and not invalid_packages and
                not incomplete and not failed and not mandatory_missing and not build_failures)
    return {"schema": "cherri-complete-go-regression-stream/1", "passed": complete,
            "packageTerminals": package_terminals, "expectedPackages": sorted(allowed),
            "noTestFilePackages": sorted(no_test_packages), "missingPackages": missing_packages,
            "nonpassingPackages": invalid_packages, "requiredParents": list(required),
            "requiredParentsNotPassed": mandatory_missing, "namedRuns": len(named),
            "namedPasses": sum(v["terminal"] == "pass" for v in named.values()),
            "namedFailures": failed, "namedSkips": skipped, "incompleteNamedTests": incomplete,
            "buildEvents": builds, "buildFailures": build_failures,
            "eventCount": lines, "testOutputEvents": output_events,
            "testOutputUTF8Bytes": output_bytes, "originalStreamBytes": size,
            "originalStreamSHA256": digest.hexdigest(),
            "namedTerminals": [{"package": p, "test": n, **value}
                               for (p, n), value in sorted(named.items())]}


def main():
    require(len(sys.argv) == 1 and platform.system() == "Linux" and platform.machine() in ("aarch64", "arm64"),
            "FIXED_NATIVE_LINUX_ARM64_ROUTE")
    environ = dict(os.environ)
    event = json.loads(Path(environ["GITHUB_EVENT_PATH"]).read_bytes())
    require(environ["GITHUB_REPOSITORY"] == "davidpovarsky/cherri" and
            environ["GITHUB_EVENT_NAME"] == "create" and event.get("ref_type") == "branch" and
            event.get("ref") == BRANCH and environ["GITHUB_REF"] == "refs/heads/"+BRANCH and
            environ["GITHUB_RUN_ATTEMPT"] == "1", "FIRST_EXACT_CREATE_EVENT")
    controller_sha = environ["GITHUB_SHA"]
    require(re.fullmatch("[0-9a-f]{40}", controller_sha) is not None, "CONTROLLER_SHA")
    workspace = Path(environ["GITHUB_WORKSPACE"]).resolve(strict=True)
    controller, source = workspace/"controller", workspace/"source"
    for directory in (controller, source):
        require(directory.resolve(strict=True) == directory and (directory/".git").is_dir() and
                not (directory/".git/objects/info/alternates").exists() and
                not (directory/".git/info/grafts").exists(), "ORDINARY_SEPARATE_SOURCE")
    helper = controller/(PREFIX+"owned_source_process.py")
    require(sha(helper.read_bytes()) == HELPER_SHA, "UNCHANGED_QUALIFIED_PROCESS_OWNER")
    spec = importlib.util.spec_from_file_location("distinct_function_owned_process", helper)
    owned = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(owned)
    output = Path(environ["RUNNER_TEMP"]).resolve(strict=True)/"cherri-current-regression-7abdf6"
    output.mkdir(mode=0o700)
    commands = output/"commands"
    commands.mkdir(mode=0o700)
    temporary = output.parent/"cherri-current-regression-tmp-7abdf6"
    temporary.mkdir(mode=0o700)
    env = {key: environ[key] for key in ("PATH", "HOME", "LANG", "LC_ALL", "LC_CTYPE", "TZ", "GOCACHE", "GOMODCACHE") if key in environ}
    env.update(PYTHONDONTWRITEBYTECODE="1", GOMAXPROCS="1", GOFLAGS="-p=1 -mod=readonly", GOMEMLIMIT="1GiB",
               CHERRI_FULL_SUITE_SUBPROCESS_PARALLEL="2", GOTOOLCHAIN="local", GOENV="off", GOWORK="off", GOTELEMETRY="off",
               GIT_NO_REPLACE_OBJECTS="1", GIT_OPTIONAL_LOCKS="0", GIT_TERMINAL_PROMPT="0",
               TMPDIR=str(temporary), GOTMPDIR=str(temporary), CGO_ENABLED="1")
    processes = []
    minimum = {"disk": shutil.disk_usage(output).free, "memory": 1 << 62}
    result = {"schema":"cherri-current-source-root-internal-regression/1", "status":"FAILED",
              "source":SOURCE, "tree":TREE, "sourceParents":list(PARENTS), "controller":controller_sha,
              "runId":int(environ["GITHUB_RUN_ID"]), "attempt":1, "event":"create", "branch":BRANCH,
              "host":{"system":platform.system(), "machine":platform.machine(), "release":platform.release(),
                      "python":platform.python_version()}, "runner":"ubuntu-24.04-arm",
              "wholeOwnerSeconds":1200, "goTestInternalSeconds":600,
              "compilationInclusiveChildSeconds":900, "processes":processes, "goTests":[],
              "requiredParentCount":85, "requiredScenarioChildCount":88,
              "fullSuiteScheduler":{"CHERRI_FULL_SUITE_SUBPROCESS_PARALLEL":"2", "testParallel":2},
              "productsStatus":"NOT_PRODUCED", "products":{},
              "priorFull8333Outcome":"FAIL_ORIGINAL_PACKAGE_ALARM_AND_COMPARATOR_BASELINE; preserved e782d9f8",
              "priorFocused8333Outcome":"PASS_23_PREVIOUSLY_UNREACHED_PARENTS_618_EVENTS; separate d80ece16",
              "formalAcceptanceGranted":False, "rawNativeTypesObserved":False,
              "full179Executed":False, "nativeAppleExecuted":False, "providerInvoked":False,
              "privateCorpusPresent":False, "releasePromoted":False, "startedAt":owned.utc()}
    owned.original(output/"github-event.original.json", Path(environ["GITHUB_EVENT_PATH"]).read_bytes())
    with owned.WholeBudget(1200, cleanup_seconds=8, final_reserve=60) as budget:
        def monitor():
            mem = {k:int(v.strip().split()[0])*1024 for k,v in
                   (line.split(":",1) for line in Path("/proc/meminfo").read_text().splitlines())}
            observed = {"disk":shutil.disk_usage(output).free,"memory":mem["MemAvailable"]}
            for key in minimum:
                minimum[key] = min(minimum[key],observed[key])
            require(observed["disk"] >= 2*1024**3 and observed["memory"] >= 1024**3, "HOSTED_LIVE_CAPACITY")
        def run(name, argv, seconds=20, finalizing=False, override=None):
            actual_name = "%03d-%s"%(len(processes)+1,name)
            row = owned.run_owned(argv, cwd=source, env={**env, **(override or {})},
                                  output=commands, name=actual_name, budget=budget,
                                  seconds=seconds, monitor=monitor,
                                  post_reserve=2 if finalizing else None)
            processes.append(row)
            owned.require_success(row)
            require(row["cleanup"]["parentReaped"] and row["cleanup"]["after"]["state"] == "ABSENT",
                    "OWNED_GROUP_REAPED_AND_ABSENT")
            return (commands/(actual_name+".stdout.original")).read_bytes()
        def git(directory,name,*args,finalizing=False):
            return run(name,["git","--no-replace-objects","-C",str(directory),*args],20,finalizing)
        def stable(path):
            before=path.lstat()
            require(stat.S_ISREG(before.st_mode) and before.st_size <= 128*1024**2, "REGULAR_BOUNDED_INPUT")
            digest=hashlib.sha256()
            blob=hashlib.sha1(("blob %d\0"%before.st_size).encode())
            with path.open("rb") as stream:
                for chunk in iter(lambda:stream.read(1024**2),b""):
                    budget.check()
                    digest.update(chunk);blob.update(chunk)
            after=path.lstat()
            require(all(getattr(before,k)==getattr(after,k) for k in
                        ("st_dev","st_ino","st_mode","st_uid","st_gid","st_size","st_mtime_ns","st_ctime_ns")),
                    "FILE_CHANGED_DURING_READ")
            return {"bytes":before.st_size,"sha256":digest.hexdigest(),"blob":blob.hexdigest(),
                    "mode":stat.S_IMODE(before.st_mode)}
        def snapshot(directory,head,tree=None,selected=None,finalizing=False):
            require(not git(directory,"replace","replace","-l",finalizing=finalizing), "NO_REPLACE_REFS")
            actual=git(directory,"identity","rev-parse","HEAD","HEAD^{tree}",finalizing=finalizing).decode().splitlines()
            require(actual[0]==head and (tree is None or actual[1]==tree), "EXACT_SOURCE_IDENTITY")
            require(not git(directory,"status","status","--porcelain=v1","--untracked-files=all",finalizing=finalizing),
                    "CLEAN_SOURCE")
            entries=git(directory,"tree","ls-tree","-rz","--full-tree","HEAD",
                        *(["--",*selected] if selected else []),finalizing=finalizing)
            files={}
            for entry in entries.split(b"\0"):
                if not entry:continue
                metadata,path=entry.split(b"\t",1);mode,kind,blob=metadata.decode().split()
                path=path.decode()
                require(mode in ("100644","100755") and kind=="blob" and
                        not path.startswith("/") and all(part not in ("",".","..",".git") for part in path.split("/"))
                        and path not in files,"CLOSED_TRACKED_PATH")
                observed=stable(directory/path)
                require(observed["blob"]==blob and (observed["mode"]&0o111)==(int(mode,8)&0o111),
                        "PHYSICAL_SOURCE_MATCHES_GIT")
                files[path]=observed
            require(files and (selected is None or set(files)==set(selected)),"COMPLETE_INPUT_INVENTORY")
            return {"source":actual[0],"tree":actual[1],"files":files}
        before=controller_before=None
        try:
            monitor()
            before=snapshot(source,SOURCE,TREE)
            require(len(before["files"])==7959,"EXPECTED_SOURCE_FILE_COUNT")
            controller_before=snapshot(controller,controller_sha,selected=CONTROLLER_FILES)
            require(git(source,"source-parents","rev-list","--parents","-n","1","HEAD").decode().split()==
                    [SOURCE,*PARENTS],"EXACT_CURRENT_SOURCE_MERGE_PARENTS")
            owned.original(output/"source-before.original.json",before)
            owned.original(output/"controller-before.original.json",controller_before)
            for path in CONTROLLER_FILES:
                target=output/"controller-originals"/path
                target.parent.mkdir(parents=True,exist_ok=True)
                owned.original(target,(controller/path).read_bytes())
            version=run("go-version",["go","version"]).decode().strip()
            require(version=="go version go1.24.13 linux/arm64","PINNED_GO_TOOLCHAIN")
            result["goVersion"]=version
            result["goEnvironment"]=json.loads(run("go-environment",["go","env","-json","GOOS","GOARCH","CGO_ENABLED","GOVERSION","GOFLAGS","GOTOOLCHAIN","GOMOD","GOCACHE","GOMODCACHE"]))
            require(git(controller,"controller-parent","rev-parse","HEAD^").decode().strip()==SOURCE,
                    "CONTROLLER_IS_EXACT_SOURCE_CHILD")
            for path, digest in SOURCE_PINS.items():
                require(before["files"][path]["sha256"]==digest, "EXACT_REPAIR_AND_ORACLE_SOURCE:"+path)
            observed_parents=[]
            for path in SOURCE_PINS:
                if path.endswith("_test.go"):
                    directory=Path(path).parent.as_posix()
                    package=PACKAGE if directory=="." else PACKAGE+"/"+directory
                    for name in re.findall(r"(?m)^func (Test[A-Za-z0-9_]+)\(",(source/path).read_text()):
                        identity=(package,name)
                        if identity in REQUIRED_PARENTS:
                            observed_parents.append(identity)
            require(sorted(observed_parents)==sorted(REQUIRED_PARENTS) and len(observed_parents)==85,
                    "ALL_85_ORIGINAL_AND_NEW_AFFECTED_PARENTS")
            require(len(REQUIRED_SCENARIO_CHILDREN)==88 and len(set(REQUIRED_IDENTITIES))==173,
                    "ALL_EXACT_MEASURE_AND_COMPARATOR_SCENARIOS")
            internal_directories={str(Path(path).parent) for path in before["files"]
                                  if path.startswith("internal/") and path.endswith(".go") and
                                  "testdata" not in Path(path).parts}
            require(sorted([PACKAGE]+[PACKAGE+"/"+path for path in internal_directories])==
                    list(EXPECTED_PACKAGES), "ALL_ORIGINAL_INTERNAL_PACKAGES")
            no_test_directories={path for path in internal_directories
                                 if not any(str(Path(name).parent)==path and name.endswith("_test.go")
                                            for name in before["files"])}
            require({PACKAGE+"/"+path for path in no_test_directories}==set(NO_TEST_PACKAGES),
                    "EXACT_NO_TEST_FILE_PACKAGE_INVENTORY")
            # The original full-suite default is ten minutes per package.
            # The900-second child includes the actual compilation and test command.
            # The unchanged467 owner writes complete, uncapped streams directly
            # to exclusive files; the original live disk/memory guards remain.
            actual_name="%03d-full-root-and-internal-go"%(len(processes)+1)
            process=owned.run_owned(list(REGRESSION_COMMAND),cwd=source,env=env,output=commands,
                                    name=actual_name,budget=budget,seconds=900,monitor=monitor)
            processes.append(process)
            result["fullRootAndInternalSuiteExecuted"]=process["pid"] is not None
            result["originalGoCommand"]=list(REGRESSION_COMMAND)
            result["originalGoProcessName"]=actual_name
            result["originalGoStreams"]={key:process.get(key) for key in ("stdout","stderr")}
            require(process["cleanup"]["parentReaped"] and
                    process["cleanup"]["after"]["state"]=="ABSENT", "OWNED_GO_GROUP_CLOSED")
            require(process["stdout"]["complete"] and process["stderr"]["complete"],
                    "COMPLETE_UNCAPPED_ORIGINAL_GO_STREAMS")
            def intake_tick():
                budget.check()
            summary=inspect_regression_stream(commands/(actual_name+".stdout.original"),
                    list(EXPECTED_PACKAGES),NO_TEST_PACKAGES,
                    list(REQUIRED_IDENTITIES),tick=intake_tick)
            require(summary["originalStreamBytes"]==process["stdout"]["bytes"] and
                    summary["originalStreamSHA256"]==process["stdout"]["sha256"],
                    "ENTIRE_ORIGINAL_GO_STREAM_RECONCILED")
            result["goTests"].append(summary)
            result["goEventIndex"]=owned.original(output/"go-event-index.derived.json",summary)
            result["repairSourcePins"]=SOURCE_PINS
            owned.require_success(process)
            require(summary["passed"],"FULL_ROOT_INTERNAL_REGRESSION_FAILED")
            remaining=budget.deadline-time.monotonic()
            result["productBuildAdmission"]={
                "remainingWholeSeconds":remaining,"requiredWholeSeconds":PRODUCT_ADMISSION_SECONDS,
                "threeBuildCommandsSeconds":540,"threeMetadataCommandsSeconds":60,
                "sixCleanupReservesSeconds":48,"unchangedFinalReserveSeconds":60,
                "synchronousReadAndReceiptReserveSeconds":92,
                "qualifiedOriginalBuilderSHA256":"567cb33831c5a4be315205068432ecc4ddc03e1de5d9f82ed78b24f08c0e8ac4"}
            if remaining>=PRODUCT_ADMISSION_SECONDS:
                budget.check(reserve=PRODUCT_ADMISSION_SECONDS)
                products=output/"products"
                products.mkdir(mode=0o700)
                run("compile-original-test-package",["go","test","-c","-buildvcs=true","-o",str(products/"cherri.test"),"."],180)
                for name,package in (("cherri","."),("cherri-migrate","./cmd/cherri-migrate")):
                    run("build-"+name,["go","build","-trimpath","-buildvcs=true","-o",str(products/name),package],180,override={"CGO_ENABLED":"0"})
                result["products"]={}
                for name in ("cherri.test","cherri","cherri-migrate"):
                    info=run("build-info-"+name,["go","version","-m",str(products/name)])
                    require(b"vcs.revision="+SOURCE.encode() in info and b"vcs.modified=false" in info,
                            "CLEAN_ORIGINAL_BINARY_BUILD_INFO")
                    result["products"][name]=stable(products/name)
                result["productsStatus"]="PASS_ORIGINAL_THREE_PRODUCTS"
            else:
                result["productsNotProducedReason"]="INSUFFICIENT_REMAINING_ORIGINAL_WHOLE_BUDGET_FOR_ALL_THREE_BUILDS"
            result["status"]="PASS_FULL_ROOT_AND_INTERNAL_GO"

        except BaseException as error:
            result["error"]={"type":type(error).__name__,"message":str(error),"traceback":traceback.format_exc()}
        finally:
            try:
                with budget.finalizing():
                    after=snapshot(source,SOURCE,TREE,finalizing=True)
                    owned.original(output/"source-after.original.json",after)
                    controller_after=snapshot(controller,controller_sha,selected=CONTROLLER_FILES,finalizing=True)
                    owned.original(output/"controller-after.original.json",controller_after)
                    require(before==after and controller_before==controller_after,"SOURCE_OR_CONTROLLER_CHANGED")
                    require(sha(helper.read_bytes())==HELPER_SHA,"HELPER_CHANGED")
                    result["sourceUnchanged"]=True
            except BaseException as error:
                result["closingError"]={"type":type(error).__name__,"message":str(error)}
                result["status"]="FAILED"
            result["minimumCapacity"]=minimum
            result["finishedAt"]=owned.utc()
            receipt=owned.original(output/"result.original.json",result)
            print(json.dumps({"path":str(output/"result.original.json"),"sha256":receipt["sha256"],
                              "status":result["status"],"source":SOURCE,
                              "namedPasses":[x["namedPasses"] for x in result["goTests"]],
                              "streamBytes":result.get("originalGoStreams"),"error":result.get("error"),
                              "closingError":result.get("closingError")}))
    return 0 if result["status"]=="PASS_FULL_ROOT_AND_INTERNAL_GO" else 1


if __name__ == "__main__":
    raise SystemExit(main())
