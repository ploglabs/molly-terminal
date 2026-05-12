package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ploglabs/molly-terminal/internal/model"
)

var extLang = map[string]string{
	".go":       "go",
	".py":       "python",
	".js":       "javascript",
	".ts":       "typescript",
	".tsx":      "tsx",
	".jsx":      "jsx",
	".rs":       "rust",
	".c":        "c",
	".cpp":      "cpp",
	".h":        "c",
	".java":     "java",
	".sh":       "bash",
	".bash":     "bash",
	".zsh":      "bash",
	".fish":     "bash",
	".md":       "markdown",
	".json":     "json",
	".yaml":     "yaml",
	".yml":      "yaml",
	".toml":     "toml",
	".html":     "html",
	".css":      "css",
	".sql":      "sql",
	".rb":       "ruby",
	".php":      "php",
	".cs":       "csharp",
	".kt":       "kotlin",
	".swift":    "swift",
	".lua":      "lua",
	".r":        "r",
	".hs":       "haskell",
	".ex":       "elixir",
	".exs":      "elixir",
	".erl":      "erlang",
	".vim":      "vim",
	".tf":       "hcl",
	".proto":    "protobuf",
	".xml":      "xml",
	".Makefile": "makefile",
}

type FileCmd struct{}

func NewFileCmd() *FileCmd {
	return &FileCmd{}
}

func (c *FileCmd) Name() string { return "file" }

func (c *FileCmd) Description() string {
	return "Send a file: /file <path> or /file to open picker"
}

func (c *FileCmd) Execute(args []string) (tea.Cmd, error) {
	return func() tea.Msg {
		var path string
		if len(args) == 0 {
			// Open native file picker
			picked, err := pickFile()
			if err != nil {
				return CommandOutputMsg{Messages: []model.Message{SystemMsg("file picker: " + err.Error())}}
			}
			path = picked
		} else {
			path = args[0]
		}

		path = strings.TrimSpace(path)
		if path == "" {
			return CommandOutputMsg{Messages: []model.Message{SystemMsg("no file selected")}}
		}

		info, err := os.Stat(path)
		if err != nil {
			return CommandOutputMsg{Messages: []model.Message{SystemMsg("file error: " + err.Error())}}
		}
		if info.IsDir() {
			return CommandOutputMsg{Messages: []model.Message{SystemMsg("file error: selected path is a directory")}}
		}

		return SendFileMsg{
			Path:    path,
			Content: fmt.Sprintf("attached `%s`", filepath.Base(path)),
		}
	}, nil
}

func pickFile() (string, error) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("osascript", "-e", `POSIX path of (choose file)`)
	case "linux":
		if _, err := exec.LookPath("zenity"); err == nil {
			cmd = exec.Command("zenity", "--file-selection", "--title=Select File")
		} else if _, err := exec.LookPath("kdialog"); err == nil {
			cmd = exec.Command("kdialog", "--getopenfilename", ".", "*")
		} else {
			return "", fmt.Errorf("no file picker found — install zenity: sudo apt install zenity")
		}
	case "windows":
		cmd = exec.Command("powershell", "-Command",
			`Add-Type -AssemblyName System.Windows.Forms; $d = New-Object System.Windows.Forms.OpenFileDialog; if($d.ShowDialog() -eq 'OK'){$d.FileName}`)
	default:
		return "", fmt.Errorf("file picker not supported on %s — use /file <path>", runtime.GOOS)
	}

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("file picker cancelled or failed")
	}
	return strings.TrimSpace(string(out)), nil
}

func detectLang(path string) string {
	base := strings.ToLower(filepath.Base(path))
	if base == "dockerfile" || base == "makefile" || base == "gemfile" {
		return base
	}
	ext := strings.ToLower(filepath.Ext(path))
	if l, ok := extLang[ext]; ok {
		return l
	}
	return ""
}
