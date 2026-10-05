import XCTest

class ImportHelperUITests: XCTestCase {
    func testInteractWithShortcuts() {
        let app = XCUIApplication(bundleIdentifier: "com.apple.shortcuts")
        let springboard = XCUIApplication(bundleIdentifier: "com.apple.springboard")
        app.activate()

        _ = app.wait(for: .runningForeground, timeout: 5)
        print("=== SHORTCUTS DEBUG HIERARCHY ===")
        print(app.debugDescription)

        // 1. Check for alerts in app or springboard
        for alertApp in [app, springboard] {
            for alert in alertApp.alerts.allElementsBoundByIndex {
                print("Found alert: \(alert.label)")
                for btnName in ["OK", "Allow", "Always Allow", "Run", "Dismiss", "Close"] {
                    let b = alert.buttons[btnName]
                    if b.exists {
                        print("Tapping alert button: \(btnName)")
                        b.tap()
                        break
                    }
                }
            }
        }

        // 2. Check for Onboarding "Continue" button
        let continueBtn = app.buttons["Continue"]
        if continueBtn.waitForExistence(timeout: 2) {
            print("Tapping 'Continue' onboarding button...")
            continueBtn.tap()
            sleep(1)
        }

        // 3. Search for Add Shortcut confirmation button across all buttons and sheets
        let candidates = [
            "Add Shortcut",
            "+ Add Shortcut",
            "Add",
            "Set Up Shortcut",
            "Replace",
            "Run Shortcut",
            "Run"
        ]

        var tapped = false
        for label in candidates {
            let btn = app.buttons[label]
            if btn.waitForExistence(timeout: 2) {
                print("Tapping button by label: '\(label)'...")
                btn.tap()
                tapped = true
                break
            }
        }

        if !tapped {
            // Predicate search across buttons
            let predicate = NSPredicate(format: "label CONTAINS[c] 'Add'")
            let matchingButtons = app.buttons.matching(predicate)
            if matchingButtons.count > 0 {
                let firstMatching = matchingButtons.element(boundBy: 0)
                print("Tapping matching button with label: '\(firstMatching.label)'")
                firstMatching.tap()
                tapped = true
            }
        }

        // 4. Also check sheets / dialogs if button was inside a modal sheet
        if !tapped {
            for sheet in app.sheets.allElementsBoundByIndex {
                for label in candidates {
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
