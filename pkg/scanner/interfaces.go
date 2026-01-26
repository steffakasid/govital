package scanner

import (
	"io/fs"
	"time"
)

// CommandExecutor executes shell commands
type CommandExecutor interface {
	Execute(name string, args ...string) ([]byte, error)
	ExecuteInDir(dir, name string, args ...string) ([]byte, error)
}

// FileReader reads files from the filesystem
type FileReader interface {
	ReadFile(path string) ([]byte, error)
	Stat(path string) (fs.FileInfo, error)
	MkdirTemp(dir, pattern string) (string, error)
	RemoveAll(path string) error
}

// GitClient interacts with git repositories
type GitClient interface {
	GetCommitTime(repoURL, commitHash string) (time.Time, error)
}

// DefaultCommandExecutor is the default implementation using exec.Command
type DefaultCommandExecutor struct{}

// DefaultFileReader is the default implementation using os functions
type DefaultFileReader struct{}
