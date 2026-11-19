# Actual e61 full iOS result: D03 UI failure

Run: https://github.com/davidpovarsky/cherri/actions/runs/44622064212

Source: `e61e0d306bb363c39ade612ef35ccfc8051ddf3c`. The previously queued job acquired a macOS 15 ARM runner. Compiler checks, the CherriCore XCFramework, Swift packages and Simulator app build passed. The real XCTest process exited 65. Of 53 observed tests, 52 passed and the documentation search UI test failed. The existing collector correctly emitted zero formal acceptance records.

The exact failed assertion is `CherriEditorUITests.testD03CurrentDocumentationSearchUI`, source line 271: no migration result was observed after searching for `@count` and tapping the explicit migration switch. The source asserted switch-on only after that failed result wait; the old diagnostic receipt therefore does not establish its post-tap state. The bundled public docs do contain the relevant `@count` example at `fork-language/migration.md:46`, and the native bundled-index tests passed. These observations do not distinguish a missed UI transition from a result-query problem and do not prove a production search-model defect.

The exact original six-file artifact `14584562226` is retained with its original ZIP: 66,591 bytes, SHA256 `6dc9974bc6cd5c52ba21d578097f0d0f3e8bad541b2fcba859c13ac2dec02ed9`. All file hashes and the original job-log digest are in the adjacent JSON receipt. No original bytes were rewritten. The entire `ios/` source diff from e61 to frozen R2 `4e56dbcfe895baa8f944b83715ecff0d07bfd24e` is empty, so the queued R2 run does not already contain a fix for this assertion.

Native UI diagnosis/replay remains required. Individual passing test observations are preserved as diagnostics; they have not been promoted through a failed whole-run gate. Full iOS, D03 app integration and final179 remain incomplete.
