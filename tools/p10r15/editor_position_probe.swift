import Foundation

@main struct EditorPositionProbe {
    struct Case {
        let name: String
        let source: String
        let selection: NSRange?
        let line: Int?
        let column: Int?
    }
    static func main() {
        let text = "שלום🙂\r\n👩‍💻e\u{0301}"
        precondition(Array(text.utf16).count == 15)
        let cases: [Case] = [
            Case(name:"start",source:text,selection:NSRange(location:0,length:0),line:1,column:1),
            Case(name:"hebrew_end",source:text,selection:NSRange(location:4,length:0),line:1,column:5),
            Case(name:"astral_end",source:text,selection:NSRange(location:6,length:0),line:1,column:7),
            Case(name:"crlf_next_line",source:text,selection:NSRange(location:8,length:0),line:2,column:1),
            Case(name:"woman_end",source:text,selection:NSRange(location:10,length:0),line:2,column:3),
            Case(name:"zwj_end",source:text,selection:NSRange(location:11,length:0),line:2,column:4),
            Case(name:"laptop_end",source:text,selection:NSRange(location:13,length:0),line:2,column:6),
            Case(name:"combining_base",source:text,selection:NSRange(location:14,length:0),line:2,column:7),
            Case(name:"combining_end",source:text,selection:NSRange(location:15,length:0),line:2,column:8),
            Case(name:"no_selection",source:text,selection:nil,line:nil,column:nil),
            Case(name:"negative_offset",source:text,selection:NSRange(location:-1,length:0),line:nil,column:nil),
            Case(name:"past_end",source:text,selection:NSRange(location:16,length:0),line:nil,column:nil),
            Case(name:"selection_not_caret",source:text,selection:NSRange(location:4,length:2),line:nil,column:nil),
            Case(name:"empty_start",source:"",selection:NSRange(location:0,length:0),line:1,column:1),
            Case(name:"empty_past_end",source:"",selection:NSRange(location:1,length:0),line:nil,column:nil),
            Case(name:"lf_next_line",source:"א\n🙂",selection:NSRange(location:2,length:0),line:2,column:1),
            Case(name:"lf_astral_end",source:"א\n🙂",selection:NSRange(location:4,length:0),line:2,column:3),
        ]
        var failed = 0
        for item in cases {
            let actual = CherriCompletionRequest(source:item.source,selection:item.selection).position
            let passed = actual?.line == item.line && actual?.column == item.column
            if !passed { failed += 1 }
            let observation: [String:Any] = ["line":actual.map{ $0.line as Any } ?? NSNull(),"column":actual.map{ $0.column as Any } ?? NSNull()]
            let row:[String:Any] = ["event":"control","name":item.name,"result":passed ? "PASS":"FAIL","observation":observation,"scope":"actual Foundation UTF16 helper; no UI"]
            let data=try! JSONSerialization.data(withJSONObject:row,options:[.sortedKeys])
            print(String(data:data,encoding:.utf8)!)
        }
        print("{\"event\":\"summary\",\"controls\":17,\"failed\":\(failed),\"uiExecuted\":false}")
        if failed != 0 {exit(1)}
    }
}
