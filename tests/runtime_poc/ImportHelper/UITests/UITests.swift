import XCTest

class ImportHelperUITests: XCTestCase {
    func testInteractWithShortcuts() {
        let app = XCUIApplication(bundleIdentifier: "com.apple.shortcuts")
        let springboard = XCUIApplication(bundleIdentifier: "com.apple.springboard")
        app.activate()

        _ = app.wait(for: .runningForeground, timeout: 3)

        print("=== SHORTCUTS DEBUG HIERARCHY ===")
        print(app.debugDescription)
        print("=== SPRINGBOARD DEBUG HIERARCHY ===")
        print(springboard.debugDescription)

        var stage = ProcessInfo.processInfo.environment["CHERRI_STAGE"] ?? ""
        if stage.isEmpty, let content = try? String(contentsOfFile: "/tmp/cherri_stage.txt", encoding: .utf8) {
            stage = content.trimmingCharacters(in: .whitespacesAndNewlines)
        }
        var menuTarget = ProcessInfo.processInfo.environment["CHERRI_MENU_TARGET"] ?? ""
        if menuTarget.isEmpty, let content = try? String(contentsOfFile: "/tmp/cherri_menu_target.txt", encoding: .utf8) {
            menuTarget = content.trimmingCharacters(in: .whitespacesAndNewlines)
        }
        print("Active UI Helper stage: '\(stage)', menuTarget: '\(menuTarget)'")

        var tapped = false
        let targets = [("SpringBoard", springboard), ("Shortcuts", app)]

        // Strict Lumen import: generic onboarding clicks must never count as success.
        if stage == "lumen_import_strict" {
            let addButton = app.buttons["Add Shortcut"]
            guard addButton.waitForExistence(timeout: 20) else {
                XCTFail("LUMEN_IMPORT_FAILED: exact Add Shortcut button is missing")
                return
            }
            addButton.tap()
            print("LUMEN_IMPORT_EXACT_ADD_TAPPED")
            let gone = NSPredicate(format: "exists == false")
            let disappeared = XCTWaiter.wait(for: [XCTNSPredicateExpectation(predicate: gone, object: addButton)], timeout: 15)
            guard disappeared == .completed else {
                XCTFail("LUMEN_IMPORT_FAILED: Add Shortcut sheet remained after tap")
                return
            }
            print("LUMEN_IMPORT_ADD_SHEET_DISMISSED")
            return
        }
        // 0. STRICT MENU SELECTION STAGE
        // Must locate and tap the exact requested menu item (Alpha or Beta).
        // Never fall through to Allow, Run, Add, or generic buttons.
        if stage.hasPrefix("menu_select_") {
            let validTargets = ["Alpha", "Beta"]
            guard validTargets.contains(menuTarget) else {
                XCTFail("CHERRI_EXACT_MENU_TAP failed: invalid or disallowed menuTarget '\(menuTarget)' for strict stage '\(stage)'")
                return
            }
            if stage == "menu_select_alpha" && menuTarget != "Alpha" {
                XCTFail("CHERRI_EXACT_MENU_TAP failed: stage menu_select_alpha expected target 'Alpha', got '\(menuTarget)'")
                return
            }
            if stage == "menu_select_beta" && menuTarget != "Beta" {
                XCTFail("CHERRI_EXACT_MENU_TAP failed: stage menu_select_beta expected target 'Beta', got '\(menuTarget)'")
                return
            }

            print("Strict menu selection stage: '\(stage)' with target '\(menuTarget)'")

            var exactTapped = false
            for (name, targetApp) in targets {
                // Priority 1: staticText (as observed on iOS 27 for Choose from Menu)
                let text = targetApp.staticTexts[menuTarget]
                if text.waitForExistence(timeout: 4) {
                    let actualLabel = text.label.isEmpty ? menuTarget : text.label
                    print("Found exact staticText '\(menuTarget)' in \(name), tapping...")
                    text.tap()
                    print("CHERRI_EXACT_MENU_TAP target=\(menuTarget) actual_label=\(actualLabel) app=\(name) kind=staticText stage=\(stage) result=tapped")
                    exactTapped = true
                    break
                }

                // Priority 2: cell
                let cell = targetApp.cells[menuTarget]
                if cell.waitForExistence(timeout: 2) {
                    let actualLabel = cell.label.isEmpty ? menuTarget : cell.label
                    print("Found exact menu cell '\(menuTarget)' in \(name), tapping...")
                    cell.tap()
                    print("CHERRI_EXACT_MENU_TAP target=\(menuTarget) actual_label=\(actualLabel) app=\(name) kind=cell stage=\(stage) result=tapped")
                    exactTapped = true
                    break
                }

                // Priority 3: cell containing staticText with exact identifier
                let cellWithText = targetApp.tables.cells.containing(.staticText, identifier: menuTarget).element
                if cellWithText.waitForExistence(timeout: 2) {
                    print("Found exact cell containing staticText '\(menuTarget)' in \(name), tapping...")
                    cellWithText.tap()
                    print("CHERRI_EXACT_MENU_TAP target=\(menuTarget) actual_label=\(menuTarget) app=\(name) kind=cellContainingText stage=\(stage) result=tapped")
                    exactTapped = true
                    break
                }

                // Priority 4: button
                let btn = targetApp.buttons[menuTarget]
                if btn.waitForExistence(timeout: 2) {
                    let actualLabel = btn.label.isEmpty ? menuTarget : btn.label
                    print("Found exact menu button '\(menuTarget)' in \(name), tapping...")
                    btn.tap()
                    print("CHERRI_EXACT_MENU_TAP target=\(menuTarget) actual_label=\(actualLabel) app=\(name) kind=button stage=\(stage) result=tapped")
                    exactTapped = true
                    break
                }

                // Priority 5: any descendant with exact identifier
                let elem = targetApp.descendants(matching: .any)[menuTarget]
                if elem.waitForExistence(timeout: 2) {
                    let actualLabel = elem.label.isEmpty ? menuTarget : elem.label
                    print("Found exact descendant matching '\(menuTarget)' in \(name), tapping...")
                    elem.tap()
                    print("CHERRI_EXACT_MENU_TAP target=\(menuTarget) actual_label=\(actualLabel) app=\(name) kind=descendant stage=\(stage) result=tapped")
                    exactTapped = true
                    break
                }
            }

            if !exactTapped {
                XCTFail("CHERRI_EXACT_MENU_TAP failed: could not locate or tap exact menu target '\(menuTarget)' in stage '\(stage)'. Never falling through to permissions or generic buttons.")
                return
            }

            print("UI Test Helper finished strict menu tap. Target: \(menuTarget)")
            return
        }

        // 1. If in IMPORT mode, prioritize tapping Add Shortcut / Replace / Set Up
        if stage.contains("import") || app.buttons["Add Shortcut"].exists {
            let importCandidates = [
                "Add Shortcut",
                "+ Add Shortcut",
                "Add",
                "Set Up Shortcut",
                "Replace"
            ]
            for (name, targetApp) in targets {
                for label in importCandidates {
                    let btn = targetApp.buttons[label]
                    if btn.waitForExistence(timeout: 3) {
                        print("Import mode: found and tapping button '\(label)' in \(name)...")
                        btn.tap()
                        tapped = true
                        break
                    }
                }
                if tapped { break }
            }
        }

        // 3. Runtime permission "Allow" / "Always Allow"
        if !tapped {
            for (name, targetApp) in targets {
                for allowLabel in ["Always Allow", "Allow", "Allow Once"] {
                    let btn = targetApp.buttons[allowLabel]
                    if btn.waitForExistence(timeout: 1) {
                        print("Found and tapping '\(allowLabel)' in \(name)...")
                        btn.tap()
                        tapped = true
                        break
                    }
                }
                if tapped { break }
            }
        }

        // 4. Check for alerts in springboard or app
        if !tapped {
            for (name, alertApp) in targets {
                for alert in alertApp.alerts.allElementsBoundByIndex {
                    print("Found alert: \(alert.label) in \(name)")
                    for btnName in ["Allow", "Always Allow", "OK", "Run", "Dismiss", "Close"] {
                        let b = alert.buttons[btnName]
                        if b.exists {
                            print("Tapping alert button: \(btnName)")
                            b.tap()
                            tapped = true
                            break
                        }
                    }
                    if tapped { break }
                }
                if tapped { break }
            }
        }

        // 5. Check for Onboarding "Continue" button
        if !tapped {
            let continueBtn = app.buttons["Continue"]
            if continueBtn.waitForExistence(timeout: 1) {
                print("Tapping 'Continue' onboarding button...")
                continueBtn.tap()
                tapped = true
                sleep(1)
            }
        }

        // 6. Search for Add Shortcut confirmation button across all buttons and sheets (only when not in run_permission stage)
        if !tapped && !stage.contains("run_permission") {
            var candidates = [
                "Add Shortcut",
                "+ Add Shortcut",
                "Add",
                "Set Up Shortcut",
                "Replace",
                "Run Shortcut",
                "Run"
            ]
            if stage.contains("dict") {
                candidates.append("Play, CherriDictionaryRuntimePOC")
                candidates.append("CherriDictionaryRuntimePOC")
            } else if stage.contains("menu") {
                candidates.append("Play, CherriMenuRuntimePOC")
                candidates.append("CherriMenuRuntimePOC")
            } else if stage.contains("legacy_if") {
                candidates.append("Play, CherriLegacyIfPOC")
                candidates.append("CherriLegacyIfPOC")
            } else if stage.contains("poc") {
                candidates.append("Play, CherriRuntimePOC")
                candidates.append("CherriRuntimePOC")
            }

            for (name, targetApp) in targets {
                for label in candidates {
                    let btn = targetApp.buttons[label]
                    if btn.waitForExistence(timeout: 1) {
                        print("Tapping button by label: '\(label)' in \(name)...")
                        btn.tap()
                        tapped = true
                        break
                    }
                }
                if tapped { break }
            }
        }

        // 7. Predicate search across buttons for Allow or Add
        if !tapped {
            for (name, targetApp) in targets {
                let predicate: NSPredicate
                if stage.contains("run_permission") {
                    predicate = NSPredicate(format: "label ==[c] 'Allow' OR label CONTAINS[c] 'Allow' OR label CONTAINS[c] 'Always Allow'")
                } else {
                    predicate = NSPredicate(format: "label ==[c] 'Allow' OR label CONTAINS[c] 'Allow' OR label CONTAINS[c] 'Add'")
                }
                let matchingButtons = targetApp.buttons.matching(predicate)
                if matchingButtons.count > 0 {
                    let firstMatching = matchingButtons.element(boundBy: 0)
                    print("Tapping matching button with label: '\(firstMatching.label)' in \(name)...")
                    firstMatching.tap()
                    tapped = true
                    break
                }
            }
        }

        // 8. Also check sheets / dialogs if button was inside a modal sheet
        if !tapped {
            for sheet in app.sheets.allElementsBoundByIndex {
                for label in ["Add Shortcut", "+ Add Shortcut", "Add", "Allow"] {
                    let btn = sheet.buttons[label]
                    if btn.exists {
                        print("Tapping sheet button: '\(label)'...")
                        btn.tap()
                        tapped = true
                        break
                    }
                }
                if tapped { break }
            }
        }

        print("UI Test Helper finished. Tapped button: \(tapped)")
    }
    func testStorageProbeImportStrict() {
        let app = XCUIApplication(bundleIdentifier: "com.apple.shortcuts")
        app.activate()
        let add = app.buttons["Add Shortcut"]
        XCTAssertTrue(add.waitForExistence(timeout: 25), "STORAGE_IMPORT_FAIL: exact Add Shortcut not visible")
        add.tap()
        let gone = NSPredicate(format: "exists == false")
        let result = XCTWaiter.wait(for: [XCTNSPredicateExpectation(predicate: gone, object: add)], timeout: 20)
        XCTAssertEqual(result, .completed, "STORAGE_IMPORT_FAIL: import sheet did not close")
        print("STORAGE_PROBE_IMPORT_PASS")
    }
    func testStorageProbeNoRuntimeError() {
        let app = XCUIApplication(bundleIdentifier: "com.apple.shortcuts")
        app.activate()
        sleep(5)
        let dump = app.debugDescription
        print("STORAGE_RUNTIME_HIERARCHY_START")
        print(dump)
        print("STORAGE_RUNTIME_HIERARCHY_END")
        XCTAssertFalse(dump.localizedCaseInsensitiveContains("Location Doesn't Exist"), "STORAGE_RUNTIME_FAIL: missing location")
        XCTAssertFalse(dump.localizedCaseInsensitiveContains("Could not find the shortcut"), "STORAGE_RUNTIME_FAIL: shortcut missing")
        XCTAssertFalse(dump.localizedCaseInsensitiveContains("The action failed"), "STORAGE_RUNTIME_FAIL: action error")
        print("STORAGE_PROBE_NO_VISIBLE_ERROR")
    }}
