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

        var tapped = false
        let targets = [("SpringBoard", springboard), ("Shortcuts", app)]

        // 1. Check for runtime permission "Allow" / "Always Allow" in SpringBoard and Shortcuts
        for (name, targetApp) in targets {
            for allowLabel in ["Allow", "Always Allow"] {
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

        // 2. Check for alerts in springboard or app
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

        // 3. Check for Onboarding "Continue" button
        if !tapped {
            let continueBtn = app.buttons["Continue"]
            if continueBtn.waitForExistence(timeout: 1) {
                print("Tapping 'Continue' onboarding button...")
                continueBtn.tap()
                tapped = true
                sleep(1)
            }
        }

        // 4. Search for Add Shortcut confirmation button across all buttons and sheets
        if !tapped {
            let candidates = [
                "Add Shortcut",
                "+ Add Shortcut",
                "Add",
                "Set Up Shortcut",
                "Replace",
                "Run Shortcut",
                "Run",
                "Play, CherriRuntimePOC",
                "CherriRuntimePOC"
            ]

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

        // 5. Predicate search across buttons for Allow or Add
        if !tapped {
            for (name, targetApp) in targets {
                let predicate = NSPredicate(format: "label ==[c] 'Allow' OR label CONTAINS[c] 'Allow' OR label CONTAINS[c] 'Add'")
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

        // 6. Also check sheets / dialogs if button was inside a modal sheet
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
