import XCTest

class ImportHelperUITests: XCTestCase {
    func testConfirmSheets() {
        let app = XCUIApplication(bundleIdentifier: "com.apple.shortcuts")
        app.activate()

        _ = app.wait(for: .runningForeground, timeout: 5)
        print("=== SHORTCUTS DEBUG HIERARCHY ===")
        print(app.debugDescription)

        // 1. Check for Onboarding "Continue" button
        let continueBtn = app.buttons["Continue"]
        if continueBtn.waitForExistence(timeout: 2) {
            print("Tapping 'Continue' onboarding button...")
            continueBtn.tap()
        }

        // 2. Check for "Add Shortcut" confirmation button
        for label in ["Add Shortcut", "Add", "Set Up Shortcut", "Replace"] {
            let btn = app.buttons[label]
            if btn.waitForExistence(timeout: 2) {
                print("Tapping '\(label)' button...")
                btn.tap()
                break
            }
        }

        // 3. Check for any OK dismiss button on alerts if needed
        let okBtn = app.alerts.buttons["OK"]
        if okBtn.waitForExistence(timeout: 1) {
            print("Dismissing alert with OK...")
            okBtn.tap()
        }
    }
}
