package lower

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// readWorkspaceModule uses the Go 1.24 rooted filesystem API to keep every
// component lookup under the importing document's directory, including symlinks.
// A separate EvalSymlinks check followed by ReadFile would reintroduce a race.
// https://go.dev/blog/osroot
func (l *Lowerer) readWorkspaceModule(requested string) (string, string, error) {
	base := "."
	if l.baseFilePath != "" {
		base = filepath.Dir(l.baseFilePath)
	}
	root, err := os.OpenRoot(base)
	if err != nil {
		return "", "", fmt.Errorf("[E_WORKSPACE_ACCESS] cannot access module %q within workspace", requested)
	}
	defer root.Close()
	relative := filepath.Clean(requested)
	file, err := root.Open(relative)
	if os.IsNotExist(err) {
		relative += ".cherri"
		file, err = root.Open(relative)
	}
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", fmt.Errorf("cannot resolve module %q: file not found", requested)
		}
		// Do not expose a resolved symlink target or an OS error's host path.
		return "", "", fmt.Errorf("[E_WORKSPACE_ACCESS] cannot access module %q within workspace", requested)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return "", "", fmt.Errorf("[E_WORKSPACE_ACCESS] module %q must be a regular file within workspace", requested)
	}
	content, err := io.ReadAll(file)
	if err != nil {
		return "", "", fmt.Errorf("cannot read module %q within workspace", requested)
	}
	return string(content), filepath.Join(base, relative), nil
}
