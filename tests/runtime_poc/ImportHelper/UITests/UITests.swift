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

        // 1. If in IMPORT mode, prioritize tapping Add Shortcut / Replace / Set Up
        if stage.contains("import") {
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

        // 2. If menu target is requested, search for and tap the menu item
        if !tapped && !menuTarget.isEmpty {
            print("Looking for menu item: '\(menuTarget)'")
            for (name, targetApp) in targets {
                let text = targetApp.staticTexts[menuTarget]
                if text.waitForExistence(timeout: 2) {
                    print("Found and tapping staticText '\(menuTarget)' in \(name)...")
                    text.tap()
                    tapped = true
                    break
                }
                let cell = targetApp.cells[menuTarget]
                if cell.waitForExistence(timeout: 1) {
                    print("Found and tapping menu cell '\(menuTarget)' in \(name)...")
                    cell.tap()
                    tapped = true
                    break
                }
                let cellWithText = targetApp.tables.cells.containing(.staticText, identifier: menuTarget).element
                if cellWithText.waitForExistence(timeout: 1) {
                    print("Found and tapping cell containing '\(menuTarget)' in \(name)...")
                    cellWithText.tap()
                    tapped = true
                    break
                }
                let btn = targetApp.buttons[menuTarget]
                if btn.waitForExistence(timeout: 1) {
                    print("Found and tapping menu button '\(menuTarget)' in \(name)...")
                    btn.tap()
                    tapped = true
                    break
                }
                let elem = targetApp.descendants(matching: .any)[menuTarget]
                if elem.waitForExistence(timeout: 1) {
                    print("Found descendant matching '\(menuTarget)' in \(name), tapping...")
                    elem.tap()
                    tapped = true
                    break
                }
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
}
