package modules

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/workspace"
)

type Issue struct {
	Code    string
	Message string
	Span    source.Span
}

func (e *Issue) Error() string { return "[" + e.Code + "] " + e.Message }

type Module struct {
	Key        string
	Path       string
	Program    *syntax.Program
	Builtin    string
	SourceHash string
	Imports    map[*syntax.ImportDecl]*Module
	info       os.FileInfo
}

type Graph struct {
	Root    *Module
	Modules []*Module // dependencies before their importers
}

func HasImports(program *syntax.Program) bool {
	for _, declaration := range program.Declarations {
		if _, ok := declaration.(*syntax.ImportDecl); ok {
			return true
		}
	}
	return false
}

type resolver struct {
	registry      *schema.Registry
	rootPath      string
	root          *workspace.Root
	migrateSource func(string) string
	virtual       map[string]string
	cache         map[string]*Module
	active        map[*Module]bool
	stack         []*Module
	graph         *Graph
}

func Resolve(program *syntax.Program, entry string, virtual map[string]string, registry *schema.Registry, migrateSource func(string) string) (*Graph, *Issue) {
	if !HasImports(program) {
		root := &Module{Key: "entry", Path: "main.cherri", Program: program, Imports: map[*syntax.ImportDecl]*Module{}}
		return &Graph{Root: root, Modules: []*Module{root}}, nil
	}
	if registry == nil {
		registry = schema.DefaultRegistry()
	}
	path, local := workspace.FilePath(entry)
	rootPath := ""
	if local {
		rootPath = filepath.Dir(path)
	}
	r := &resolver{
		registry: registry, rootPath: rootPath, virtual: map[string]string{}, migrateSource: migrateSource,
		cache: map[string]*Module{}, active: map[*Module]bool{}, graph: &Graph{},
	}
	defer func() {
		if r.root != nil {
			r.root.Close()
		}
	}()
	for key, text := range virtual {
		clean := filepath.Clean(filepath.FromSlash(key))
		if absolute, ok := workspace.FilePath(key); ok && (filepath.IsAbs(key) || strings.HasPrefix(strings.ToLower(key), "file:")) {
			if rootPath == "" {
				continue
			}
			relative, err := filepath.Rel(rootPath, absolute)
			if err != nil {
				continue
			}
			clean = relative
		}
		if filepath.IsLocal(clean) {
			r.virtual[filepath.ToSlash(clean)] = text
		}
	}
	name := "main.cherri"
	if local {
		name = filepath.Base(path)
	}
	root := &Module{Key: filepath.ToSlash(name), Path: filepath.ToSlash(name), Program: program, Imports: map[*syntax.ImportDecl]*Module{}}
	r.graph.Root = root
	r.cache[root.Key] = root
	// File identity is used only after an authorized rooted open. It catches
	// cycles/duplicates through in-workspace symlinks without exposing their targets.
	if local {
		if opened, err := workspace.Open(entry); err == nil {
			r.root = opened
			if resource, err := opened.Open(name, name); err == nil {
				root.info = resource.Info
				resource.Close()
			}
		}
	}
	if issue := r.visit(root); issue != nil {
		return nil, issue
	}
	return r.graph, nil
}

func (r *resolver) visit(module *Module) *Issue {
	r.active[module] = true
	r.stack = append(r.stack, module)
	defer func() {
		delete(r.active, module)
		r.stack = r.stack[:len(r.stack)-1]
	}()
	if module != r.graph.Root && len(module.Program.Statements) != 0 {
		return &Issue{"E_MODULE_INITIALIZATION", fmt.Sprintf("module top-level runtime initialization rejected: imported module %q contains top-level executable statements", module.Path), module.Program.Span}
	}
	aliases := map[string]bool{}
	for _, declaration := range module.Program.Declarations {
		imported, ok := declaration.(*syntax.ImportDecl)
		if !ok {
			if module != r.graph.Root {
				switch declaration.(type) {
				case *syntax.FunctionDecl, *syntax.EnumDecl, *syntax.TypeDecl, *syntax.ActionDecl, *syntax.CopyBlockStmt:
				default:
					return &Issue{"E_MODULE_INITIALIZATION", fmt.Sprintf("module top-level runtime initialization rejected: imported module %q contains an unsupported workflow declaration", module.Path), declaration.NodeSpan()}
				}
			}
			continue
		}
		if aliases[imported.Alias] {
			return &Issue{"E_DUPLICATE_NAME", fmt.Sprintf("duplicate import alias %q", imported.Alias), imported.Span}
		}
		aliases[imported.Alias] = true
		dependency, issue := r.resolve(module, imported)
		if issue != nil {
			return issue
		}
		module.Imports[imported] = dependency
		if r.active[dependency] {
			var chain []string
			for _, item := range r.stack {
				chain = append(chain, item.Path)
			}
			chain = append(chain, dependency.Path)
			return &Issue{"E_IMPORT_CYCLE", "import cycle detected: " + strings.Join(chain, " -> "), imported.Span}
		}
		alreadyVisited := false
		for _, known := range r.graph.Modules {
			alreadyVisited = alreadyVisited || known == dependency
		}
		if !alreadyVisited && dependency.Builtin == "" {
			if issue := r.visit(dependency); issue != nil {
				return issue
			}
		}
	}
	r.graph.Modules = append(r.graph.Modules, module)
	return nil
}

func (r *resolver) resolve(parent *Module, declaration *syntax.ImportDecl) (*Module, *Issue) {
	requested := declaration.Path
	fail := func(code, message string) (*Module, *Issue) {
		return nil, &Issue{code, message, declaration.Span}
	}
	if strings.Contains(requested, "://") {
		return fail("E_WORKSPACE_ACCESS", fmt.Sprintf("remote package imports not permitted: %q", requested))
	}
	clean := filepath.Clean(filepath.FromSlash(requested))
	if filepath.IsAbs(clean) || filepath.VolumeName(clean) != "" {
		return fail("E_WORKSPACE_ACCESS", fmt.Sprintf("path traversal outside workspace not permitted: %q", requested))
	}
	for _, action := range r.registry.AllActions() {
		if action.Module != "" && action.Module == filepath.ToSlash(clean) {
			key := "builtin-action:" + action.Module
			if module := r.cache[key]; module != nil {
				return module, nil
			}
			module := &Module{Key: key, Path: action.Module, Builtin: action.Module, Imports: map[*syntax.ImportDecl]*Module{}}
			r.cache[key] = module
			return module, nil
		}
	}
	if clean == "stdfunc" || clean == "stdlib" {
		const key = "builtin:stdfunc"
		if module := r.cache[key]; module != nil {
			return module, nil
		}
		text := StandardLibrarySource()
		if text == "" {
			return fail("E_MODULE_DEPENDENCY", "cannot resolve module \"stdfunc\": preserved standard library source is unavailable")
		}
		return r.parse(key, "stdfunc.cherri", text, nil, declaration.Span)
	}
	relative, relativeErr := workspace.Relative(parent.Path, requested)
	if relativeErr != nil {
		return fail("E_WORKSPACE_ACCESS", fmt.Sprintf("path traversal outside workspace not permitted: %q", requested))
	}
	key := relative
	if module := r.cache[key]; module != nil {
		return module, nil
	}
	if text, exists := r.virtual[key]; exists {
		return r.parse(key, key, text, nil, declaration.Span)
	}
	if text, exists := r.virtual[key+".cherri"]; exists {
		if module := r.cache[key+".cherri"]; module != nil {
			return module, nil
		}
		return r.parse(key+".cherri", key+".cherri", text, nil, declaration.Span)
	}
	if r.root == nil {
		return fail("E_MODULE_DEPENDENCY", fmt.Sprintf("cannot resolve module %q: local workspace is unavailable", requested))
	}
	resource, err := r.root.Open(parent.Path, requested)
	if errors.Is(err, fs.ErrNotExist) {
		resource, err = r.root.Open(parent.Path, requested+".cherri")
	}
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fail("E_MODULE_DEPENDENCY", fmt.Sprintf("cannot resolve module %q: file not found", requested))
		}
		if errors.Is(err, workspace.ErrNotRegular) {
			return fail("E_WORKSPACE_ACCESS", fmt.Sprintf("module %q must be a regular file within workspace", requested))
		}
		return fail("E_WORKSPACE_ACCESS", fmt.Sprintf("cannot access module %q within workspace", requested))
	}
	defer resource.Close()
	info := resource.Info
	for _, module := range r.cache {
		if module.info != nil && os.SameFile(module.info, info) {
			return module, nil
		}
	}
	text, err := io.ReadAll(resource.File)
	if err != nil {
		return fail("E_WORKSPACE_ACCESS", fmt.Sprintf("cannot read module %q within workspace", requested))
	}
	key = resource.RelativePath
	return r.parse(key, key, string(text), info, declaration.Span)
}

func (r *resolver) parse(key, path, text string, info os.FileInfo, span source.Span) (*Module, *Issue) {
	file := source.NewFile(source.DocumentID(path), path, 1, text)
	parser := syntax.NewParser(file)
	program := parser.ParseProgram()
	if len(parser.Errors()) != 0 {
		if r.migrateSource == nil {
			return nil, &Issue{"E_SYNTAX", fmt.Sprintf("syntax error in imported module %q: %v", path, parser.Errors()[0]), span}
		}
		file = source.NewFile(source.DocumentID(path), path, 1, r.migrateSource(text))
		parser = syntax.NewParser(file)
		program = parser.ParseProgram()
		if len(parser.Errors()) != 0 {
			return nil, &Issue{"E_SYNTAX", fmt.Sprintf("syntax error in imported module %q: %v", path, parser.Errors()[0]), span}
		}
	}
	module := &Module{
		Key: key, Path: path, Program: program, SourceHash: fmt.Sprintf("%x", sha256.Sum256([]byte(text))),
		Imports: map[*syntax.ImportDecl]*Module{}, info: info,
	}
	r.cache[key] = module
	return module, nil
}
