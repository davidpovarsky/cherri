# Cherri P10R15 — דוח מסירה טכני מאומת לשיחה הבאה

**נבדק:** יום חמישי, 19 בנובמבר 2026, 21:12–21:20 שעון ישראל. **סטטוס:** OPEN / RELEASE NOT VERIFIED / CI BLOCKED.  
**הערה:** מסמך זה נכתב בענף תיעוד `handoff/cherri-p10r15-20261119` המבוסס על קומיט המקור, ואינו משנה את ענף הקוד או מעניק לו הצלחת בדיקות. יש לבצע בדיקה חיה חדשה לפני פעולה נוספת.

## נקודת פתיחה וענפים

| רכיב | ערך ומגבלות |
|---|---|
| קוד מקור Windows | `C:\Users\DAVID\Code\cherri-p10r15-project-closure-20261118` |
| ענף העבודה המקומי | `agent/cherri-p10r15-project-closure-20261118` |
| קומיט משולב שנבדק | `d7052f4568db26dba169bdefc72623e48c04968a` |
| ענף אינטגרציה GitHub | `staging/cherri-p10r15-integration-20261119`, אותו SHA `d7052f45` |
| מצב working tree | נקי בבדיקה 2026-11-19T19:12:14Z |
| Consumer K4 | `7546d0203405ba326e4e71ff9f74115dd0d34fa8` |
| Compiler S4 | `b97ae752205f1e747b6c98ff7511f3484dd53bc2` |
| Docs D4 | `205217a256b5f220724363b44b5ae516ffe6a539`, אומת כ-main של `davidpovarsky/cherrilang.org` |
| מסמכים קיימים | `docs/ai-team/handoffs/cherri-p10r15-execution-ledger.md`; `docs/apple-runtime/p10r15/`; `docs/language-v2/evidence/` |
| משימת־על | [Asana 1220635996500993](https://app.asana.com/1/1219597879016953/project/1219603572114432/task/1220635996500993) |
| דוח מצב הפרויקט | [Asana status 1220719800380571](https://app.asana.com/0/1219603572114432/1220719800380571) |
| משימת חסימה חדשה | [P0 Legacy Parity 1220719969438779](https://app.asana.com/1/1219597879016953/project/1219603572114432/task/1220719969438779) |

**חשוב:** יש שני צירים: `d7052f45` הוא מקור האינטגרציה האחרון, ו-`K4 7546d020` הוא המקור האחרון עם ריצות Build & Test/iOS מוכרות. אין להעתיק תוצאות בדיקות של K4 אל d705. ענף מסמך המסירה אינו מועמד שחרור.

## תקציר קבלה רשמית

קובץ `docs/language-v2/acceptance-results.json` נבדק על הדיסק. Timestamp: `2026-11-19T04:15:19Z`, phase `local`, `closure_complete=false`.

| סטטוס | מספר |
|---|---:|
| PASSED | 73 |
| FAILED | 0 |
| NOT_RUN | 100 |
| PENDING_EXTERNAL | 5 |
| AI_EVAL_NOT_RUN | 1 |
| סך הכול | 179 |
| לא הושלמו | **106** |

חמש דרישות `PENDING_EXTERNAL`: `S01` — Unicode/variable tokens; `L02` — native indexing boundaries; `L04` — native dynamic bound safety; `F01` — observable native short-circuit effects; `E07` — exact editor source edit behavior. `AI01` טרם בוצע. הדרישות 100 `NOT_RUN` מופיעות בשמות, רמות ודרישות המקור בקובץ הקבלה. אסור להתייחס ל־0 FAILED כ־179 PASS, ואין לשנות status רק על סמך smoke tests.

## ריצות CI רשמיות

| מקור | ריצה | תוצאה מדויקת |
|---|---|---|
| K4 `7546d020` | [Build & Test 44466481583](https://github.com/davidpovarsky/cherri/actions/runs/44466481583) | **FAIL**: Linux/macOS Build + Full Go + Matrix + Discovery + Decompilation + Package Manager PASS, אך Differential Legacy Parity Gate FAIL בשני ה־OS. Nix package build PASS. חלק מהשערים אחריו דולגו. |
| K4 `7546d020` | [iOS Build 44466483177](https://github.com/davidpovarsky/cherri/actions/runs/44466483177) | **CANCELLED BEFORE EXECUTION**, לא כשל app/UI ולא PASS. |
| S4 `b97ae752` | [Build & Test 44462962145](https://github.com/davidpovarsky/cherri/actions/runs/44462962145) | FAILED על menus asset בשלושה test events, תוקן בתיקון ממוקד אך ריצת CI הישנה לא השתנתה. |
| d7052f45 | — | **אין ריצת CI מקיפה מוכחת** על המקור המשולב הזה. |

### כשל ה־Legacy Parity המדויק (K4)

לוג job `135365378437` בלינוקס ו־`135365378866` ב־macOS מאשר:

- **47** קובצי fixtures מקוריים; **9** התאמות Plist bit-for-bit; **35** שקילויות סמנטיות; **1** כשל קומפילציית V2; **2** הבדלים מבניים; **0** כשלי Legacy מקוריים ו־**0** כשלי מיגרציה.
- `globals.cherri`: **7 Structural Differences**, עדיין לא הוכח שהבדלי המבנה מותרים.
- `standard.cherri`: **9 Structural Differences**, עדיין לא הוכח שהבדלי המבנה מותרים.
- `menus.cherri`: `E_WORKSPACE_ACCESS: embedded local asset: file does not exist` עבור `assets/cherri_icon.png` בתוך סביבת מיגרציה מבודדת.
- **17/17** פעולות Apple נוספות נבדקו והצליחו בשני מסלולי הקומפילציה; אין בכך הוכחת runtime מלא של 505 פעולות.
- תיקון מבודד `fd790278f40c2b1352eba2821863b33ddea29c06`, משולב כ־`e6c1a43c46c6f73bd87f0bf622899def56a419b6`, שומר קובץ PNG מקורי בתוך סביבת test owned והצליח ב־11 בדיקות root וב־1 producer. הוא **כבר אב קדמון של K4**, אך ה־Differential Runner הרשמי עדיין נכשל; יש לתקן גם אותו. **לא לבטל** את מדיניות `E_WORKSPACE_ACCESS`, לא לשנות fixture מקור בלא ראיה, ולא לבצע "accept new expected" רק להסרת ה־FAIL.

## מה הושלם עם ראיות — לפי גבול ההוכחה

1. **Compiler/Language:** תיקוני source משמעותיים ב־V2/Legacy parser, typed defaults, enums, output UUID and OutputName preservation, module/import paths, source package promotion, numeric handling, asset integrity, fail-closed validation, scoped macros ופריטי language acceptance. קיימים commits, בדיקות ממוקדות, דוחות אימות ו־Go package tests. עם זאת אין קבלה רחבה על d705.
2. **38 real Shortcuts corpus:** על S4 בוצעו **5/5** בדיקות private מקוריות; **38** קיצורים, **76/76** עצי plist תואמים בשני dialects; **27,652** פעולות, **23,353** NATURAL_VERIFIED (**84.45%**), **4,298** EXACT_NATIVE_FALLBACK, **1** SOURCE_INVALID מקורי, **0** demotions. ה־fallback המדויק אינו קוד Cherri טבעי קריא ואינו מצדיק סימון הייבוא הרחב כשלם. חומרי גלם נשארו פרטיים.
3. **Docs/Skill:** אתר Docs ב־D4 פורסם; בדיקות Go תיעוד ממוקדות **74/74**, SwiftUI component isolated typecheck, חלק מבדיקות Skill negative/packaging/install מאומתות על גרסאות נבדלות. אין הוכחה להתקנת כל הצרכנים על אותו SHA.
4. **Apple signed Simulator:** חתימה וייבוא אמיתיים של fixtures סינתטיים ב־iPadOS Simulator, callbacks ו־trace/RTL מוגבלים עברו בחלק מהריצות הישנות. קיימות גם ריצות failed/cancelled ותרחישי native Unicode, Clipboard, nested List, AX/Plist שאינם מוכחים. אין להעביר את הצלחת ה־Simulator למכשיר iPad פיזי.
5. **Security/provenance:** בקרות מקור קבצים, אימות ראיות, החלפת names/UUID, צילום מסך ו־AX privacy quarantine, negative signing tests ו־rollback staging. אין לעקוף הגנת workspace כדי לעבור CI, ואין להעלות corpus פרטי או raster פרטי לארטיפקט ציבורי.
6. **Macros/Runtime:** הוכחות חלקיות ל־require/measure/collect, Debug/Release, trace, Stored Content Text, HTTP response continuation ו־iOS callbacks. **עדיין לא** הוכחו במלואם user-facing `debugExport()`, `cache()` TTL typed, `retry()` backoff/transport errors או `paginate()`.

## מה נשאר לביצוע — סדר עדיפויות מחייב

### P0 — חסימות לפני מועמד שחרור

1. לאמת עדכנית branch/HEAD/status, לקרוא את `AGENTS.md` ו־`CLAUDE.md` כמדיניות עבודה מבלי לעקוף מנגנוני אבטחה.
2. לפתור את שלוש חריגות ה־Parity (globals, standard, menus) ב־Differential Runner עצמו; לשמר invariants ו־public fixtures. להוציא receipts לפני/אחרי עם SHA, לקבל 47/47 מצבים תקינים ו־17/17 פעולות.
3. להריץ **full Build & Test Linux/macOS/Nix על candidate frozen SHA אחד** ולוודא כל השערים, לא רק Go. לשמור כישלון אמיתי.
4. להריץ **iOS Build + XCUI UI suite + signed simulator QA** על אותו קומיט: docs D03 app-local search, foreground import/library, one-to-one editor AX vs native plist, typed defaults, Unicode/Clipboard/Debug export, nested empty List cardinality, signed Legacy/V2, runtime cancel/error. K4 iOS לא הורץ.
5. לבצע `formal-179` end-to-end fresh רמות Unit/Integration/Simulator/External/AI עם 179 contracts, לקרוא כל `NOT_RUN` ו־`PENDING_EXTERNAL` ולסגור רק עם הוכחה. לא לחצות SHA בלי הצהרה.
6. לחזור על **private P10/P11/P13 corpus 38×2** על SHA האחרון תוך שמירה על 100% native exact tree וקו 23,353 natural threshold המקורי, ללא דליפת מקור.

### P1 — פיצ'רים וצרכנים שעדיין לא נסגרו

7. להשלים את משימות המאקרו הקיימות: `debugLog(label:)`, `debugExport()`, typed block AST/IR, `require()` native/QA, `measure()`, `collect()`, `cache()` TTL+typed cross-run, `retry()` ו־`paginate()`, compile-time DEBUG/RELEASE; לכל אחד Legacy/V2/negative/native/Docs/Skill.
8. לוודא קטלוג **505 definitions** מול פרמטרים/Runtime נפרדים; פערי **118 built-ins** ו־**173 discoverable App Intents** הם בסיסי ספירה היסטוריים שדורשים סיווג מחדש; Actions App **99 intents/301 params**, Pure+Pack **15 Go-only** נפרדים. לא להציג 505 כהוכחת הרצה טבעית.
9. לקבע coherent installed tuple Compiler / Docs / Skill / iOS Bridge/UI/Preview / Remote Builder, לבדוק SHA של artifacts חיים, signing fault negatives, signed healthy smoke, התקנה, rollback, no private leak. **Remote Builder live טרם קודם**.
10. להשלים בדיקות iPadOS פיזי כשהתקן זמין; זהו BLOCKED_DEVICE חיצוני, לא תירוץ לביטול שאר המשימות.
11. רק אחרי כל שערי השחרור: תהליך שילוב מבוקר מול `main` ו־release, ללא force-reset; Pure Original נשאר מבודד וה־Pack אופציונלי.

## ביצוע ושמירת מצב בין שיחות

- עדכון Asana כולל [סטטוס רשמי](https://app.asana.com/0/1219603572114432/1220719800380571), [משימת־על](https://app.asana.com/1/1219597879016953/project/1219603572114432/task/1220635996500993) ו־[משימת P0 Parity](https://app.asana.com/1/1219597879016953/project/1219603572114432/task/1220719969438779). מתחתיהם משימות קיימות ל־Docs/Skill, QA/Simulator, Remote Builder, Macros, Native Catalog, Storage ו־Private Import. אין לסגור משימות בכוח.
- מצב המשימות בעת יצירת הדוח: **95 משימות, 42 מסומנות הושלמו, 53 פתוחות**. ״הושלמו״ הן סטטוס היסטורי של משימה בהיקף שלה — לא בהכרח שחרור המוצר.
- לוגים/דוחות קיימים ב־`docs/apple-runtime/p10r15/`, `docs/language-v2/evidence/`, `docs/ai-team/handoffs/cherri-p10r15-execution-ledger.md`.
- כל ביצוע עתידי: (א) verify sources, (ב) שינוי ממוקד, (ג) run actual tests, (ד) commit/push isolated branch, (ה) attach receipts & Asana update, (ו) continue until validated or explicitly report external blocker. המשתמש נתן הרשאת ביצוע רחבה, אך אין לעקוף privacy/security/provenance.
- הרצות CI ישנות הן facts historical, לא תחליף ל־CI חדש. אין עבודה רציפה של עוזר ChatGPT בין שיחות אם לא הופעל job ממשי.

## פקודות בדיקה ראשוניות לשיחה החדשה

```powershell
cd 'C:\Users\DAVID\Code\cherri-p10r15-project-closure-20261118'
git branch --show-current
git status --short
git rev-parse HEAD
git ls-remote origin refs/heads/staging/cherri-p10r15-integration-20261119
gh run view 44466481583 -R davidpovarsky/cherri
gh run view 44466483177 -R davidpovarsky/cherri
```

**Definition of Done:** parity 47/47 without unreviewed differences; final CI Linux/macOS/Nix+Apple iOS on exact final SHA; 179/179 requirements either actually accepted or explicitly agreed external scope; private corpus intact; native Apple runtime and UI with honest oracle; working required macros and action surface; coherent installed and rollback-tested consumers; physical acceptance where mandated. עד אז — **OPEN**.
