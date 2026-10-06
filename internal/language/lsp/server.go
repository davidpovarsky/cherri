package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/service"
)

// Server is a standard JSON-RPC Language Server for Cherri v2.0.
type Server struct {
	svc    *service.Service
	reader *bufio.Reader
	writer io.Writer
	mu     sync.Mutex
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
	}
}

func (s *Server) readMessage() (*rpcMessage, error) {
	// Read headers
	contentLength := 0
	for {
		line, err := s.reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break // Blank line indicates end of headers
		}
		if strings.HasPrefix(strings.ToLower(line), "content-length:") {
			val := strings.TrimSpace(line[15:])
			contentLength, _ = strconv.Atoi(val)
		}
	}

	if contentLength == 0 {
		return nil, fmt.Errorf("missing or zero Content-Length header")
	}

	body := make([]byte, contentLength)
	if _, err := io.ReadFull(s.reader, body); err != nil {
		return nil, err
	}

	var msg rpcMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

func (s *Server) sendResponse(id interface{}, result interface{}) {
	resp := rpcMessage{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	s.sendMessage(resp)
}

func (s *Server) sendNotification(method string, params interface{}) {
	paramBytes, _ := json.Marshal(params)
	msg := rpcMessage{
		JSONRPC: "2.0",
		Method:  method,
		Params:  paramBytes,
	}
	s.sendMessage(msg)
}

func (s *Server) sendMessage(msg rpcMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()

	body, _ := json.Marshal(msg)
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))
	_, _ = s.writer.Write([]byte(header))
	_, _ = s.writer.Write(body)
}

func (s *Server) handleMessage(msg *rpcMessage) {
	switch msg.Method {
	case "initialize":
		s.sendResponse(msg.ID, map[string]interface{}{
			"capabilities": map[string]interface{}{
				"textDocumentSync": 1, // Full
				"completionProvider": map[string]interface{}{
					"triggerCharacters": []string{".", ":", "\""},
				},
				"hoverProvider":              true,
				"documentFormattingProvider": true,
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
		if err := json.Unmarshal(msg.Params, &p); err == nil {
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
		if err := json.Unmarshal(msg.Params, &p); err == nil && len(p.ContentChanges) > 0 {
			s.svc.ChangeDocument(p.TextDocument.URI, p.TextDocument.Version, p.ContentChanges[0].Text)
			s.publishDiagnostics(p.TextDocument.URI)
		}

	case "textDocument/didClose":
		var p struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
		}
		if err := json.Unmarshal(msg.Params, &p); err == nil {
			s.svc.CloseDocument(p.TextDocument.URI)
		}

	case "textDocument/formatting":
		var p struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
		}
		if err := json.Unmarshal(msg.Params, &p); err == nil {
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
		if err := json.Unmarshal(msg.Params, &p); err == nil {
			items := s.svc.Complete(p.TextDocument.URI, p.Position.Line+1, p.Position.Character+1)
			s.sendResponse(msg.ID, items)
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
		if err := json.Unmarshal(msg.Params, &p); err == nil {
			hover := s.svc.Hover(p.TextDocument.URI, p.Position.Line+1, p.Position.Character+1)
			if hover == nil {
				s.sendResponse(msg.ID, nil)
			} else {
				s.sendResponse(msg.ID, map[string]interface{}{
					"contents": map[string]string{
						"kind":  "markdown",
						"value": hover.Contents,
					},
				})
			}
		}

	default:
		if msg.ID != nil {
			s.sendResponse(msg.ID, nil)
		}
	}
}

func (s *Server) publishDiagnostics(uri string) {
	diags, _ := s.svc.Analyze(uri)
	var wireDiags []map[string]interface{}
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
					"character": d.Span.Start.Column - 1,
				},
				"end": map[string]int{
					"line":      d.Span.End.LSPLine(),
					"character": d.Span.End.Column - 1,
				},
			},
		})
	}

	s.sendNotification("textDocument/publishDiagnostics", map[string]interface{}{
		"uri":         uri,
		"diagnostics": wireDiags,
	})
}
