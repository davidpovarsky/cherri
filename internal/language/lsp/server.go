package lsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/service"
	"github.com/electrikmilk/cherri/internal/language/source"
)

// Server is a standard JSON-RPC Language Server for Cherri v2.0.
type Server struct {
	svc      *service.Service
	reader   *bufio.Reader
	writer   io.Writer
	mu       sync.Mutex
	writeErr error
	stopped  bool
	shutdown bool
}

// NewServer creates a new LSP server instance.
func NewServer(in io.Reader, out io.Writer, registry *schema.Registry) *Server {
	return &Server{
		svc:    service.NewService(registry),
		reader: bufio.NewReader(in),
		writer: out,
	}
}

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  interface{}     `json:"result,omitempty"`
	Error   interface{}     `json:"error,omitempty"`
}

// Run starts the JSON-RPC read/dispatch loop until EOF.
func (s *Server) Run() error {
	for {
		msg, err := s.readMessage()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		s.handleMessage(msg)
		if s.writeErr != nil {
			return s.writeErr
		}
		if s.stopped {
			return nil
		}
	}
}

func (s *Server) readMessage() (*rpcMessage, error) {
	// Bound framing before allocation. ReadSlice also caps each header line at
	// the reader's buffer size; a clean EOF is valid only between messages.
	const maxMessageBytes = 32 << 20
	contentLength, headerBytes := 0, 0
	seenLength := false
	for {
		lineBytes, err := s.reader.ReadSlice('\n')
		if err != nil {
			if err == io.EOF && headerBytes == 0 && len(lineBytes) == 0 {
				return nil, io.EOF
			}
			return nil, fmt.Errorf("incomplete LSP header: %w", err)
		}
		headerBytes += len(lineBytes)
		if headerBytes > 16<<10 {
			return nil, fmt.Errorf("LSP headers exceed limit")
		}
		line := strings.TrimSpace(string(lineBytes))
		if line == "" {
			break
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			return nil, fmt.Errorf("malformed LSP header")
		}
		if strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			if seenLength {
				return nil, fmt.Errorf("duplicate Content-Length header")
			}
			seenLength = true
			contentLength, err = strconv.Atoi(strings.TrimSpace(value))
			if err != nil || contentLength <= 0 || contentLength > maxMessageBytes {
				return nil, fmt.Errorf("invalid or oversized Content-Length")
			}
		}
	}
	if !seenLength {
		return nil, fmt.Errorf("missing Content-Length header")
	}
	body := make([]byte, contentLength)
	if _, err := io.ReadFull(s.reader, body); err != nil {
		return nil, fmt.Errorf("incomplete LSP body: %w", err)
	}
	var msg rpcMessage
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber() // Request IDs must survive without float64 rounding.
	if err := decoder.Decode(&msg); err != nil {
		return nil, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("multiple or malformed JSON values in LSP frame")
	}
	return &msg, nil
}

func (s *Server) sendResponse(id interface{}, result interface{}) {
	if id == nil {
		return
	}
	// Success with no value still requires the result field to be present.
	s.sendMessage(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
}

func (s *Server) sendError(id any, code int, message string) {
	if id == nil {
		return
	}
	s.sendMessage(map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": code, "message": message}})
}

func (s *Server) decodeParams(msg *rpcMessage, target any) bool {
	trimmed := bytes.TrimSpace(msg.Params)
	if len(trimmed) == 0 || trimmed[0] != '{' || json.Unmarshal(trimmed, target) != nil {
		s.sendError(msg.ID, -32602, "Invalid params")
		return false
	}
	return true
}

func (s *Server) position(msg *rpcMessage, uri string, line, character int) (source.Position, bool) {
	position, ok := s.svc.PositionFromLSP(uri, line, character)
	if !ok {
		s.sendError(msg.ID, -32602, "Invalid document or position")
	}
	return position, ok
}

func (s *Server) sendNotification(method string, params interface{}) {
	s.sendMessage(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
}

func (s *Server) sendMessage(msg any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.writeErr != nil {
		return
	}
	body, err := json.Marshal(msg)
	if err != nil {
		s.writeErr = err
		return
	}
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))
	for _, part := range [][]byte{[]byte(header), body} {
		n, err := s.writer.Write(part)
		if err != nil {
			s.writeErr = err
			return
		}
		if n != len(part) {
			s.writeErr = io.ErrShortWrite
			return
		}
	}
}

func (s *Server) handleMessage(msg *rpcMessage) {
	if msg.JSONRPC != "2.0" || msg.Method == "" {
		s.sendError(msg.ID, -32600, "Invalid Request")
		return
	}
	if s.shutdown && msg.Method != "exit" {
		s.sendError(msg.ID, -32600, "Server has shut down")
		return
	}
	switch msg.Method {
	case "initialized":
		return
	case "shutdown":
		s.shutdown = true
		s.sendResponse(msg.ID, nil)
	case "exit":
		s.stopped = true
	case "initialize":
		s.sendResponse(msg.ID, map[string]interface{}{
			"capabilities": map[string]interface{}{
				"positionEncoding": "utf-16",
				"textDocumentSync": 1, // Full
				"completionProvider": map[string]interface{}{
					"triggerCharacters": []string{".", ":", "\""},
				},
				"hoverProvider":              true,
				"definitionProvider":         true,
				"renameProvider":             true,
				"documentFormattingProvider": true,
				"signatureHelpProvider": map[string]interface{}{
					"triggerCharacters": []string{"(", ","},
				},
			},
			"serverInfo": map[string]interface{}{
				"name":    "cherri-lsp",
				"version": schema.LanguageVersion,
			},
		})

	case "textDocument/didOpen":
		var p struct {
			TextDocument struct {
				URI     string `json:"uri"`
				Version int    `json:"version"`
				Text    string `json:"text"`
			} `json:"textDocument"`
		}
		if s.decodeParams(msg, &p) {
			s.svc.OpenDocument(p.TextDocument.URI, p.TextDocument.Version, p.TextDocument.Text)
			s.publishDiagnostics(p.TextDocument.URI)
		}

	case "textDocument/didChange":
		var p struct {
			TextDocument struct {
				URI     string `json:"uri"`
				Version int    `json:"version"`
			} `json:"textDocument"`
			ContentChanges []struct {
				Text string `json:"text"`
			} `json:"contentChanges"`
		}
		if s.decodeParams(msg, &p) && len(p.ContentChanges) > 0 {
			s.svc.ChangeDocument(p.TextDocument.URI, p.TextDocument.Version, p.ContentChanges[0].Text)
			s.publishDiagnostics(p.TextDocument.URI)
		}

	case "workspace/didChangeWatchedFiles":
		var changes struct {
			Changes []json.RawMessage `json:"changes"`
		}
		if err := json.Unmarshal(msg.Params, &changes); err == nil && len(changes.Changes) > 0 {
			for _, uri := range s.svc.InvalidateAssets() {
				s.publishDiagnostics(uri)
			}
		}

	case "textDocument/didClose":
		var p struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
		}
		if s.decodeParams(msg, &p) {
			s.svc.CloseDocument(p.TextDocument.URI)
		}

	case "textDocument/formatting":
		var p struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
		}
		if s.decodeParams(msg, &p) {
			formatted, err := s.svc.Format(p.TextDocument.URI)
			if err != nil {
				s.sendResponse(msg.ID, []interface{}{})
			} else {
				// Return full document text edit
				s.sendResponse(msg.ID, []map[string]interface{}{
					{
						"range": map[string]interface{}{
							"start": map[string]int{"line": 0, "character": 0},
							"end":   map[string]int{"line": 999999, "character": 0},
						},
						"newText": formatted,
					},
				})
			}
		}

	case "textDocument/completion":
		var p struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
			Position struct {
				Line      int `json:"line"`
				Character int `json:"character"`
			} `json:"position"`
		}
		if s.decodeParams(msg, &p) {
			position, ok := s.position(msg, p.TextDocument.URI, p.Position.Line, p.Position.Character)
			if !ok {
				return
			}
			items := s.svc.Complete(p.TextDocument.URI, position.Line, position.Column)
			s.sendResponse(msg.ID, completionWireItems(items))
		}

	case "textDocument/hover":
		var p struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
			Position struct {
				Line      int `json:"line"`
				Character int `json:"character"`
			} `json:"position"`
		}
		if s.decodeParams(msg, &p) {
			position, ok := s.position(msg, p.TextDocument.URI, p.Position.Line, p.Position.Character)
			if !ok {
				return
			}
			hover := s.svc.Hover(p.TextDocument.URI, position.Line, position.Column)
			if hover == nil {
				s.sendResponse(msg.ID, nil)
			} else {
				s.sendResponse(msg.ID, map[string]interface{}{
					"range": wireRange(hover.Span),
					"contents": map[string]string{
						"kind":  "markdown",
						"value": hover.Contents,
					},
				})
			}
		}

	case "textDocument/definition":
		var p struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
			Position struct {
				Line      int `json:"line"`
				Character int `json:"character"`
			} `json:"position"`
		}
		if s.decodeParams(msg, &p) {
			position, ok := s.position(msg, p.TextDocument.URI, p.Position.Line, p.Position.Character)
			if !ok {
				return
			}
			span := s.svc.Definition(p.TextDocument.URI, position.Line, position.Column)
			if span == nil {
				s.sendResponse(msg.ID, nil)
			} else {
				s.sendResponse(msg.ID, map[string]interface{}{
					"uri": p.TextDocument.URI,
					"range": map[string]interface{}{
						"start": map[string]int{
							"line":      span.Start.LSPLine(),
							"character": span.Start.LSPCharacterColumn(),
						},
						"end": map[string]int{
							"line":      span.End.LSPLine(),
							"character": span.End.LSPCharacterColumn(),
						},
					},
				})
			}
		}

	case "textDocument/signatureHelp":
		var p struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
			Position struct {
				Line      int `json:"line"`
				Character int `json:"character"`
			} `json:"position"`
		}
		if s.decodeParams(msg, &p) {
			position, ok := s.position(msg, p.TextDocument.URI, p.Position.Line, p.Position.Character)
			if !ok {
				return
			}
			sig := s.svc.SignatureHelp(p.TextDocument.URI, position.Line, position.Column)
			if sig == nil {
				s.sendResponse(msg.ID, nil)
			} else {
				params := make([]map[string]interface{}, len(sig.Parameters))
				for i, pl := range sig.Parameters {
					params[i] = map[string]interface{}{"label": pl}
				}
				s.sendResponse(msg.ID, map[string]interface{}{
					"signatures": []map[string]interface{}{
						{
							"label":         sig.Label,
							"documentation": sig.Documentation,
							"parameters":    params,
						},
					},
					"activeSignature": 0,
					"activeParameter": sig.ActiveParameter,
				})
			}
		}

	case "textDocument/rename":
		var p struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
			Position struct {
				Line      int `json:"line"`
				Character int `json:"character"`
			} `json:"position"`
			NewName string `json:"newName"`
		}
		if s.decodeParams(msg, &p) {
			position, ok := s.position(msg, p.TextDocument.URI, p.Position.Line, p.Position.Character)
			if !ok {
				return
			}
			_, edits, err := s.svc.Rename(p.TextDocument.URI, position.Line, position.Column, p.NewName)
			if err != nil {
				s.sendResponse(msg.ID, nil)
			} else {
				wireEdits := make([]map[string]interface{}, len(edits))
				for i, ed := range edits {
					wireEdits[i] = map[string]interface{}{
						"range": map[string]interface{}{
							"start": map[string]int{
								"line":      ed.Span.Start.LSPLine(),
								"character": ed.Span.Start.LSPCharacterColumn(),
							},
							"end": map[string]int{
								"line":      ed.Span.End.LSPLine(),
								"character": ed.Span.End.LSPCharacterColumn(),
							},
						},
						"newText": ed.NewText,
					}
				}
				s.sendResponse(msg.ID, map[string]interface{}{
					"changes": map[string]interface{}{
						p.TextDocument.URI: wireEdits,
					},
				})
			}
		}

	default:
		if msg.ID != nil {
			s.sendError(msg.ID, -32601, "Method not found")
		}
	}
}

func (s *Server) publishDiagnostics(uri string) {
	diags, _ := s.svc.Analyze(uri)
	wireDiags := make([]map[string]interface{}, 0, len(diags))
	for _, d := range diags {
		sev := 1 // Error
		if d.Severity == analysis.SeverityWarning {
			sev = 2
		} else if d.Severity == analysis.SeverityInformation {
			sev = 3
		} else if d.Severity == analysis.SeverityHint {
			sev = 4
		}
		wireDiags = append(wireDiags, map[string]interface{}{
			"code":     d.Code,
			"severity": sev,
			"message":  d.Message,
			"range": map[string]interface{}{
				"start": map[string]int{
					"line":      d.Span.Start.LSPLine(),
					"character": d.Span.Start.LSPCharacterColumn(),
				},
				"end": map[string]int{
					"line":      d.Span.End.LSPLine(),
					"character": d.Span.End.LSPCharacterColumn(),
				},
			},
		})
	}

	s.sendNotification("textDocument/publishDiagnostics", map[string]interface{}{
		"uri":         uri,
		"diagnostics": wireDiags,
	})
}
