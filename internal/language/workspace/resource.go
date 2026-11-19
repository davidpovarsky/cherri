// Package workspace confines local source and asset reads to one entry workspace.
package workspace

import (
	"errors"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var ErrAccess = errors.New("cannot access local resource within workspace")
var ErrNotRegular = errors.New("local resource must be a regular file")
var ErrRemote = errors.New("remote resources are not permitted")

type Root struct{ root *os.Root }

type Resource struct {
	File         *os.File
	Info         os.FileInfo
	RelativePath string
}

func (resource *Resource) Close() error { return resource.File.Close() }

func FilePath(document string) (string, bool) {
	if strings.HasPrefix(strings.ToLower(document), "file:") {
		u, err := url.Parse(document)
		if err != nil || (u.Host != "" && u.Host != "localhost") {
			return "", false
		}
		document = u.Path
		if runtime.GOOS == "windows" && len(document) > 2 && document[0] == '/' && document[2] == ':' {
			document = document[1:]
		}
	} else if strings.Contains(document, "://") {
		return "", false
	}
	if document == "" {
		document = "main.cherri"
	}
	path, err := filepath.Abs(filepath.FromSlash(document))
	return path, err == nil
}

// Open returns a root owned by the caller. Each opened resource must also be
// closed; its Info and readable File refer to the same authorized handle.
func Open(entryDocument string) (*Root, error) {
	path, local := FilePath(entryDocument)
	if !local {
		return nil, ErrAccess
	}
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, ErrAccess
	}
	return &Root{root: root}, nil
}

func (root *Root) Close() error { return root.root.Close() }

// Relative resolves against the importing document's workspace-relative path.
// Parent segments are valid only when their result remains inside the entry root.
func Relative(fromDocument, requested string) (string, error) {
	if strings.Contains(requested, "://") {
		return "", ErrRemote
	}
	from := filepath.Clean(filepath.FromSlash(fromDocument))
	path := filepath.Clean(filepath.FromSlash(requested))
	if !filepath.IsLocal(from) || filepath.IsAbs(path) || filepath.VolumeName(path) != "" {
		return "", ErrAccess
	}
	relative := filepath.Clean(filepath.Join(filepath.Dir(from), path))
	if !filepath.IsLocal(relative) {
		return "", ErrAccess
	}
	return filepath.ToSlash(relative), nil
}

// Open reads an exact requested path: it performs no extension inference,
// installation or network access. Rooted Open protects symlink traversal at use.
func (root *Root) Open(fromDocument, requested string) (*Resource, error) {
	relative, err := Relative(fromDocument, requested)
	if err != nil {
		return nil, err
	}
	file, err := root.root.Open(filepath.FromSlash(relative))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fs.ErrNotExist
		}
		return nil, ErrAccess
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, ErrAccess
	}
	if !info.Mode().IsRegular() {
		file.Close()
		return nil, ErrNotRegular
	}
	return &Resource{File: file, Info: info, RelativePath: relative}, nil
}
