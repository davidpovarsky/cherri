# P10R10 Semantic Readability & Fallback Breakdown (`001`–`038`)

- **Natural Readable Actions:** `10871/27652` (`39.31%`, **+9408** over P10R9 `1463`)

## Reason Code Distribution

| Reason Code | Count |
|---|---:|
| `AMBIGUOUS_VARIANT` | 46 |
| `CONTROL_REGION` | 10160 |
| `KNOWN_DISAMBIGUATED` | 10871 |
| `SOURCE_INVALID` | 1 |
| `THIRD_PARTY_NO_CANONICAL_DEF` | 137 |
| `UNKNOWN_NATIVE_ID` | 13 |
| `UNREPRESENTABLE_PARAM` | 6424 |

## Top Natural Callables Emitted

| Callable | Count |
|---|---:|
| `setVariable` | 2487 |
| `text` | 2176 |
| `comment` | 650 |
| `stop` | 444 |
| `chooseFromList` | 378 |
| `setName` | 374 |
| `replaceText` | 324 |
| `repeat` | 214 |
| `count` | 212 |
| `repeat.each` | 196 |
| `alert` | 184 |
| `setValue` | 170 |
| `saveFile` | 160 |
| `prompt` | 149 |
| `base64Decode` | 144 |
| `nothing` | 135 |
| `output` | 130 |
| `show` | 115 |
| `getVariable` | 88 |
| `getText` | 82 |
| `dictionary` | 80 |
| `showWebpage` | 80 |
| `mustOutput` | 78 |
| `speak` | 73 |
| `getImages` | 72 |
| `getLastItem` | 70 |
| `appendVariable` | 64 |
| `filterFiles` | 64 |
| `customImageOverlay` | 62 |
| `getDictionary` | 60 |
| `getDeviceDetail` | 59 |
| `showNotification` | 55 |
| `resizeImage` | 51 |
| `getFile` | 47 |
| `setClipboard` | 42 |
| `quicklook` | 40 |
| `getURLs` | 36 |
| `deleteFiles` | 32 |
| `createFolder` | 31 |
| `makePDF` | 30 |

## Top Remaining Native Fallbacks

| Apple Identifier | Count |
|---|---:|
| `is.workflow.actions.conditional` | 8880 |
| `is.workflow.actions.choosefrommenu` | 976 |
| `is.workflow.actions.getvalueforkey` | 704 |
| `is.workflow.actions.dictionary` | 418 |
| `is.workflow.actions.text.replace` | 404 |
| `is.workflow.actions.gettext` | 364 |
| `is.workflow.actions.runworkflow` | 316 |
| `is.workflow.actions.setvariable` | 274 |
| `is.workflow.actions.text.split` | 271 |
| `is.workflow.actions.repeat.each` | 264 |
| `is.workflow.actions.number` | 224 |
| `is.workflow.actions.getitemfromlist` | 221 |
| `is.workflow.actions.downloadurl` | 202 |
| `is.workflow.actions.text.match` | 201 |
| `is.workflow.actions.appendvariable` | 190 |
| `is.workflow.actions.url` | 183 |
| `is.workflow.actions.ask` | 167 |
| `is.workflow.actions.calculateexpression` | 159 |
| `is.workflow.actions.base64encode` | 156 |
| `is.workflow.actions.image.resize` | 144 |
| `is.workflow.actions.text.combine` | 135 |
| `is.workflow.actions.openurl` | 131 |
| `is.workflow.actions.list` | 127 |
| `is.workflow.actions.documentpicker.open` | 100 |
| `is.workflow.actions.documentpicker.save` | 91 |
| `com.openai.chat.AskIntent` | 72 |
| `is.workflow.actions.math` | 71 |
| `is.workflow.actions.filter.files` | 68 |
| `is.workflow.actions.alert` | 67 |
| `is.workflow.actions.image.crop` | 47 |
