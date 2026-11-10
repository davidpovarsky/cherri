# CHERRI P10R6 — Baseline State & Triage Report (10 בנובמבר 2026)

**מחברים לוגיים:** `[איתן | ARCH] [MAIN_SESSION]`, `[נועם | DEV] [MAIN_SESSION]`, `[רוני | QA] [MAIN_SESSION]`  
**Base Commit SHA:** `3fdf58c53f9a5acbe09cfbc58ad5770701d5bb3b` (verified HEAD on `agent/cherri-p10-apple-actions-coverage`)  
**Worktree:** `C:\Users\DAVID\Code\cherri-p10r6-real-shortcuts-20261110`  
**Branch:** `agent/cherri-p10r6-real-shortcuts-20261110`  
**Private Audit Dir:** `C:\Users\DAVID\Code\cherri-shortcut-audit-20261110`  
**Privacy Status:** Sanitize all outputs; no raw Shortcut text, URLs, credentials, or personal content.

---

## 1. תקציר מנהלים של ה-Baseline

בקורפוס הפרטי נבדקו **38 קיצורים אמיתיים**, הכוללים **27,652 מופעי פעולות** ו-**252 מזהי פעולה ייחודיים**.
התוצאות ההיסטוריות והרצות ה-Baseline המאומתות מ-`rerun_p10r5_results.json` מציגות תמונת מצב חמורה:

| מדד שער | תוצאת Baseline | אחוז מעבר | משמעות הנדסית |
|---|---|---|---|
| **Direct JSON Import** (`--import=<file.json>`) | 2 / 38 | 5.3% | 33 נפלו על `plist: type mismatch`, 2 על `interface conversion: float64, not uint64`, 1 על timeout |
| **Independent Binary Plist Import** | 26 / 38 | 68.4% | 4 נפלו על `PANIC_INTERFACE`, 4 על `PANIC_OTHER`, 3 על `ERROR`, 1 על `TIMEOUT` |
| **Recompile of Decompiled Source** | 1 / 26 | 3.8% | 25 קבצים הניבו קוד Cherri שאינו מתקמפל (Syntax errors / missing args / types) |
| **Action Sequence Preservation** | 0 / 38 | 0.0% | אף קיצור לא שמר על רצף הפעולות המדויק; בקיצור היחיד שהתקמפל (`032`), נשמרו 5 פעולות מתוך 7 במקור |
| **Full Workflow Semantic Parity** | 0 / 38 | 0.0% | אפס קיצורים נבנו מחדש באופן זהה סמנטית לקובץ המקורי |
| **AppIntentDescriptor Preservation** | 0 / 2 | 0.0% | מילוני native App Intent נהרסו והומרו ל-`WFDictionaryFieldValue` עטוף |
| **Apple Hardware Runtime Verified** | 0 / 38 | 0.0% | BLOCKED_DEVICE (חומרה חיצונית לא זמינה) |

---

## 2. פילוח מדויק של קבוצות הכשל ב-Baseline

### 2.1. Direct JSON Import (36 כשלים)
- **33 מקרים (`PANIC_PLIST_MISMATCH`):**
  - קבצים: `001, 002, 003, 004, 005, 006, 008, 009, 010, 011, 012, 013, 014, 015, 016, 018, 019, 020, 021, 022, 023, 024, 025, 026, 027, 028, 029, 030, 031, 033, 034, 035, 036`.
  - שורש הבאג: `json.Unmarshal` מפענח מספרים כ-`float64`. כאשר `mapToStruct` קורא ל-`plist.Marshal(..., plist.XMLFormat)`, המספרים מקודדים כ-`<real>`. כאשר `plist.Unmarshal` מנסה לפרוס את ה-XML לתוך struct עם שדות מסוג `int` או `uint64`, הוא קורס עם `plist: type mismatch: tried to decode plist type 'real' into value of type 'int'`. בנוסף, `fmt.Println(..., data)` מדליף נתונים פרטיים ל-stdout בעת שגיאה.
- **2 מקרים (`PANIC_INTERFACE`):**
  - קבצים: `037, 038`.
  - שורש הבאג: `mapControlFlowOutputs` מבצע `params["WFControlFlowMode"].(uint64)`. ב-JSON הערך הוא `float64`, מה שגורם ל-`panic: interface conversion: interface {} is float64, not uint64`.
- **1 מקרה (`TIMEOUT`):**
  - קובץ: `017`.
  - שורש הבאג: קיצור ענק או מבנה מורכב שנכנס ללולאה/צוואר בקבוק של ניתוח מחרוזות ומשאבים.
- **2 מקרים שעברו ייבוא בלבד (`PASS`):**
  - קבצים: `007, 032`.

### 2.2. Plist Import (12 כשלים מתוך 38)
- **4 מקרים (`PANIC_INTERFACE`):**
  - קבצים: `018, 021, 025, 035`.
  - שורש הבאג: הטלות טיפוסים בלתי בטוחות ב-`decompile.go` כגון `value.(map[string]interface{})`, `item.WFValue.(map[string]interface{})`, `conditionValue.(uint64)`.
- **4 מקרים (`PANIC_OTHER`):**
  - קבצים: `027, 028, 033, 034`.
  - שורש הבאג: קריסות פנימיות ב-`mapIdentifiers`, `checkParamValueAttachments`, או `decompAttachmentString`.
- **3 מקרים (`ERROR`):**
  - קבצים: `016, 024, 030`.
  - שורש הבאג: שגיאות עיבוד מובנות בקלט.
- **1 מקרה (`TIMEOUT`):**
  - קובץ: `017`.

### 2.3. קומפילציה חוזרת של 26 קובצי Cherri שנוצרו
- **25 מקרים (`ERROR`):**
  - שורש הבאג: ה-Decompiler מייצר קוד Cherri בלתי חוקי תחבירית, כגון קריאות לפעולות ללא ארגומנטי חובה, בעיות סוגריים, שמות משתנים משובשים, חוסר ב-`#include`, או שגיאות ב-`rawAction`.
- **1 מקרה בלבד שהתקמפל (`032`):**
  - קובץ: `LocalNeuro` (`032.plist`).
  - תוצאה: המקור הכיל 7 פעולות, אך הקיצור שנבנה מחדש הכיל 5 פעולות בלבד. 2 פעולות `is.workflow.actions.comment` הושמטו כיוון שדגל `--comments` אינו מופעל כברירת מחדל, והן נכתבו כהערות טקסט `//` בקוד Cherri ולא נבנו מחדש כפעולות Shortcuts.

---

## 3. יעדי P10R6 לאור ה-Baseline
1. ביטול מוחלט של כל משפחות ה-Panics בייבוא JSON ובייבוא Plist.
2. איחוד מלא של נתיבי הייבוא (`--import` ו-`cherri import`) עם טיפול מובנה בשגיאות וללא `os.Exit(0)` מטעה.
3. הבטחת קוד Cherri חוקי ועצמאי ב-Legacy וב-V2, ללא fallback שקט של migration.
4. שימור רצף פעולות מדויק (Sequence Parity) — כולל פעולות Comment ו-`nothing`.
5. שימור מבנה Native Dict עבור `AppIntentDescriptor` ו-`rawAction`.
6. בדיקה מקיפה על כל 38 הקיצורים ומדידה מדויקת של השיפור.
