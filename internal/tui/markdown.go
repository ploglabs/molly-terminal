package tui

import (
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"
)

var emojiMap = map[string]string{
	"rocket":             "🚀",
	"fire":               "🔥",
	"heart":              "❤️",
	"thumbsup":           "👍",
	"thumbsdown":         "👎",
	"check":              "✅",
	"x":                  "❌",
	"warning":            "⚠️",
	"star":               "⭐",
	"wave":               "👋",
	"joy":                "😂",
	"smile":              "😊",
	"eyes":               "👀",
	"100":                "💯",
	"tada":               "🎉",
	"bug":                "🐛",
	"zap":                "⚡",
	"sparkles":           "✨",
	"pray":               "🙏",
	"thinking":           "🤔",
	"clap":               "👏",
	"ok_hand":            "👌",
	"white_check_mark":   "✅",
	"information_source": "ℹ️",
	"pencil":             "✏️",
	"memo":               "📝",
	"computer":           "💻",
	"key":                "🔑",
	"lock":               "🔒",
	"unlock":             "🔓",
	"package":            "📦",
	"link":               "🔗",
	"hammer":             "🔨",
	"wrench":             "🔧",
	"gear":               "⚙️",
	"point_right":        "👉",
	"raised_hand":        "✋",
	"+1":                 "👍",
	"-1":                 "👎",
}

var (
	codeKeyword = lipgloss.Color("#4d96ff")
	codeString  = lipgloss.Color("#6bcb77")
	codeComment = lipgloss.Color("#555555")
	codeNumber  = lipgloss.Color("#ffd93d")
	codeType    = lipgloss.Color("#cc5de8")
	codePunct   = lipgloss.Color("#888888")
)

var goKeywords = map[string]bool{
	"break": true, "case": true, "chan": true, "const": true, "continue": true,
	"default": true, "defer": true, "else": true, "fallthrough": true, "for": true,
	"func": true, "go": true, "goto": true, "if": true, "import": true,
	"interface": true, "map": true, "package": true, "range": true, "return": true,
	"select": true, "struct": true, "switch": true, "type": true, "var": true,
	"nil": true, "true": true, "false": true, "make": true, "new": true,
	"len": true, "cap": true, "append": true, "copy": true, "delete": true,
	"close": true, "panic": true, "recover": true, "print": true, "println": true,
}

var goTypes = map[string]bool{
	"string": true, "int": true, "int8": true, "int16": true, "int32": true, "int64": true,
	"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
	"float32": true, "float64": true, "complex64": true, "complex128": true,
	"bool": true, "byte": true, "rune": true, "error": true, "uintptr": true,
}

var cKeywords = map[string]bool{
	"auto": true, "break": true, "case": true, "char": true, "const": true,
	"continue": true, "default": true, "do": true, "double": true, "else": true,
	"enum": true, "extern": true, "float": true, "for": true, "goto": true,
	"if": true, "int": true, "long": true, "register": true, "return": true,
	"short": true, "signed": true, "sizeof": true, "static": true, "struct": true,
	"switch": true, "typedef": true, "union": true, "unsigned": true, "void": true,
	"volatile": true, "while": true, "NULL": true, "true": true, "false": true,
	"bool": true, "inline": true, "restrict": true,
}

var rustKeywords = map[string]bool{
	"as": true, "async": true, "await": true, "break": true, "const": true,
	"continue": true, "crate": true, "dyn": true, "else": true, "enum": true,
	"extern": true, "false": true, "fn": true, "for": true, "if": true,
	"impl": true, "in": true, "let": true, "loop": true, "match": true,
	"mod": true, "move": true, "mut": true, "pub": true, "ref": true,
	"return": true, "self": true, "Self": true, "static": true, "struct": true,
	"super": true, "trait": true, "true": true, "type": true, "union": true,
	"unsafe": true, "use": true, "where": true, "while": true,
}

var pythonKeywords = map[string]bool{
	"False": true, "None": true, "True": true, "and": true, "as": true,
	"assert": true, "async": true, "await": true, "break": true, "class": true,
	"continue": true, "def": true, "del": true, "elif": true, "else": true,
	"except": true, "finally": true, "for": true, "from": true, "global": true,
	"if": true, "import": true, "in": true, "is": true, "lambda": true,
	"nonlocal": true, "not": true, "or": true, "pass": true, "raise": true,
	"return": true, "try": true, "while": true, "with": true, "yield": true,
	"print": true, "len": true, "range": true, "type": true, "int": true,
	"str": true, "list": true, "dict": true, "set": true, "tuple": true,
	"bool": true, "float": true,
}

var jsKeywords = map[string]bool{
	"break": true, "case": true, "catch": true, "class": true, "const": true,
	"continue": true, "debugger": true, "default": true, "delete": true, "do": true,
	"else": true, "export": true, "extends": true, "false": true, "finally": true,
	"for": true, "function": true, "if": true, "import": true, "in": true,
	"instanceof": true, "let": true, "new": true, "null": true, "return": true,
	"static": true, "super": true, "switch": true, "this": true, "throw": true,
	"true": true, "try": true, "typeof": true, "undefined": true, "var": true,
	"void": true, "while": true, "with": true, "yield": true, "async": true,
	"await": true, "of": true, "from": true, "interface": true, "type": true,
	"enum": true, "as": true, "implements": true, "readonly": true,
}

func renderMarkdown(text string, myUsername string, width int) string {
	var result strings.Builder
	lines := strings.Split(text, "\n")
	inCodeBlock := false
	var codeLang string
	var codeLines []string

	for i, line := range lines {
		if i > 0 {
			result.WriteString("\n")
		}

		if strings.HasPrefix(line, "```") {
			if !inCodeBlock && strings.HasSuffix(line, "```") && len(line) > 6 {
				inner := strings.TrimSuffix(strings.TrimPrefix(line, "```"), "```")
				parts := strings.SplitN(strings.TrimSpace(inner), " ", 2)
				if len(parts) == 2 {
					result.WriteString(lipgloss.NewStyle().Foreground(codeComment).Render("```" + parts[0]))
					result.WriteString("\n")
					result.WriteString(renderCodeBlock(parts[0], []string{parts[1]}))
					result.WriteString("\n")
					result.WriteString(lipgloss.NewStyle().Foreground(codeComment).Render("```"))
					continue
				}
			}
			if inCodeBlock {
				inCodeBlock = false
				rendered := renderCodeBlock(codeLang, codeLines)
				result.WriteString(rendered)
				result.WriteString("\n")
				result.WriteString(lipgloss.NewStyle().Foreground(codeComment).Render("```"))
				codeLines = nil
				codeLang = ""
			} else {
				inCodeBlock = true
				codeLang = strings.TrimSpace(strings.TrimPrefix(line, "```"))
				result.WriteString(lipgloss.NewStyle().Foreground(codeComment).Render("```" + codeLang))
			}
			continue
		}

		if inCodeBlock {
			codeLines = append(codeLines, line)
			continue
		}

		result.WriteString(renderInline(line, myUsername))
	}

	// Unclosed code block
	if inCodeBlock && len(codeLines) > 0 {
		result.WriteString("\n")
		result.WriteString(renderCodeBlock(codeLang, codeLines))
	}

	return result.String()
}

func renderCodeBlock(lang string, lines []string) string {
	var result strings.Builder
	for i, line := range lines {
		if i > 0 {
			result.WriteString("\n")
		}
		result.WriteString(highlightLine(lang, line))
	}
	return result.String()
}

func highlightLine(lang, line string) string {
	var keywords map[string]bool
	commentPrefix := "//"
	altCommentPrefix := ""

	switch lang {
	case "go":
		keywords = goKeywords
	case "c", "cpp", "c++", "h":
		keywords = cKeywords
	case "rust", "rs":
		keywords = rustKeywords
	case "python", "py":
		keywords = pythonKeywords
		commentPrefix = "#"
		altCommentPrefix = ""
	case "javascript", "js", "typescript", "ts", "jsx", "tsx":
		keywords = jsKeywords
	case "bash", "sh", "zsh", "fish", "shell":
		keywords = map[string]bool{
			"if": true, "then": true, "else": true, "elif": true, "fi": true,
			"for": true, "do": true, "done": true, "while": true, "until": true,
			"case": true, "esac": true, "in": true, "function": true,
			"return": true, "exit": true, "echo": true, "export": true,
			"local": true, "readonly": true, "declare": true, "source": true,
		}
		commentPrefix = "#"
	case "java", "kotlin", "scala":
		keywords = map[string]bool{
			"abstract": true, "assert": true, "boolean": true, "break": true,
			"byte": true, "case": true, "catch": true, "char": true, "class": true,
			"const": true, "continue": true, "default": true, "do": true,
			"double": true, "else": true, "enum": true, "extends": true,
			"final": true, "finally": true, "float": true, "for": true,
			"goto": true, "if": true, "implements": true, "import": true,
			"instanceof": true, "int": true, "interface": true, "long": true,
			"native": true, "new": true, "null": true, "package": true,
			"private": true, "protected": true, "public": true, "return": true,
			"short": true, "static": true, "strictfp": true, "super": true,
			"switch": true, "synchronized": true, "this": true, "throw": true,
			"throws": true, "transient": true, "try": true, "void": true,
			"volatile": true, "while": true, "true": true, "false": true,
		}
	default:
		return lipgloss.NewStyle().Foreground(themeCyan).Render(line)
	}

	trimmed := strings.TrimLeft(line, " \t")
	indent := line[:len(line)-len(trimmed)]

	// Full-line comment check
	if strings.HasPrefix(trimmed, commentPrefix) || (altCommentPrefix != "" && strings.HasPrefix(trimmed, altCommentPrefix)) {
		return indent + lipgloss.NewStyle().Foreground(codeComment).Italic(true).Render(trimmed)
	}

	return indent + tokenizeAndColor(trimmed, keywords, commentPrefix)
}

func tokenizeAndColor(line string, keywords map[string]bool, commentPrefix string) string {
	runes := []rune(line)
	n := len(runes)
	var out strings.Builder
	i := 0

	for i < n {
		// Inline comment (// or #)
		if strings.HasPrefix(string(runes[i:]), commentPrefix) {
			out.WriteString(lipgloss.NewStyle().Foreground(codeComment).Italic(true).Render(string(runes[i:])))
			break
		}

		// String literal (double or single quote)
		if runes[i] == '"' || runes[i] == '\'' || runes[i] == '`' {
			quote := runes[i]
			j := i + 1
			for j < n {
				if runes[j] == '\\' {
					j += 2
					continue
				}
				if runes[j] == quote {
					j++
					break
				}
				j++
			}
			out.WriteString(lipgloss.NewStyle().Foreground(codeString).Render(string(runes[i:j])))
			i = j
			continue
		}

		// Number literal
		if unicode.IsDigit(runes[i]) || (runes[i] == '-' && i+1 < n && unicode.IsDigit(runes[i+1])) {
			j := i
			if runes[j] == '-' {
				j++
			}
			for j < n && (unicode.IsDigit(runes[j]) || runes[j] == '.' || runes[j] == 'x' || runes[j] == 'X' ||
				(runes[j] >= 'a' && runes[j] <= 'f') || (runes[j] >= 'A' && runes[j] <= 'F')) {
				j++
			}
			out.WriteString(lipgloss.NewStyle().Foreground(codeNumber).Render(string(runes[i:j])))
			i = j
			continue
		}

		// Identifier or keyword
		if unicode.IsLetter(runes[i]) || runes[i] == '_' {
			j := i
			for j < n && (unicode.IsLetter(runes[j]) || unicode.IsDigit(runes[j]) || runes[j] == '_') {
				j++
			}
			word := string(runes[i:j])
			if keywords[word] {
				out.WriteString(lipgloss.NewStyle().Foreground(codeKeyword).Bold(true).Render(word))
			} else if len(word) > 0 && unicode.IsUpper([]rune(word)[0]) {
				out.WriteString(lipgloss.NewStyle().Foreground(codeType).Render(word))
			} else {
				out.WriteString(lipgloss.NewStyle().Foreground(themeCyan).Render(word))
			}
			i = j
			continue
		}

		// Punctuation / operators
		if strings.ContainsRune("{}()[];,.<>=!&|^~%+-*/", runes[i]) {
			out.WriteString(lipgloss.NewStyle().Foreground(codePunct).Render(string(runes[i])))
			i++
			continue
		}

		// Everything else
		out.WriteRune(runes[i])
		i++
	}

	return out.String()
}

func renderInline(text, myUsername string) string {
	runes := []rune(text)
	n := len(runes)
	var out strings.Builder
	i := 0

	for i < n {
		switch {
		case runes[i] == '@':
			// @mention
			j := i + 1
			for j < n && !unicode.IsSpace(runes[j]) && runes[j] != ',' && runes[j] != '.' && runes[j] != '!' {
				j++
			}
			mention := string(runes[i+1 : j])
			if myUsername != "" && strings.EqualFold(mention, myUsername) {
				out.WriteString(lipgloss.NewStyle().Foreground(themeWarn).Bold(true).Render("@" + mention))
			} else {
				out.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#91a7ff")).Render("@" + mention))
			}
			i = j
			continue

		case runes[i] == ':':
			j := i + 1
			for j < n && runes[j] != ':' && runes[j] != ' ' && runes[j] != '\n' {
				j++
			}
			if j < n && runes[j] == ':' && j > i+1 {
				name := string(runes[i+1 : j])
				if emoji, ok := emojiMap[name]; ok {
					out.WriteString(emoji)
					i = j + 1
					continue
				}
			}
			out.WriteRune(runes[i])
			i++

		case i+1 < n && runes[i] == '*' && runes[i+1] == '*':
			j := i + 2
			for j+1 < n && !(runes[j] == '*' && runes[j+1] == '*') {
				j++
			}
			if j+1 < n {
				inner := string(runes[i+2 : j])
				out.WriteString(lipgloss.NewStyle().Bold(true).Foreground(themeAccent).Render(inner))
				i = j + 2
			} else {
				out.WriteRune('*')
				out.WriteRune('*')
				i += 2
			}

		case runes[i] == '*' || runes[i] == '_':
			delim := runes[i]
			j := i + 1
			for j < n && runes[j] != delim && runes[j] != '\n' {
				j++
			}
			if j < n && runes[j] == delim && j > i+1 {
				inner := string(runes[i+1 : j])
				out.WriteString(lipgloss.NewStyle().Italic(true).Foreground(themeFg).Render(inner))
				i = j + 1
			} else {
				out.WriteRune(runes[i])
				i++
			}

		case runes[i] == '`':
			j := i + 1
			for j < n && runes[j] != '`' && runes[j] != '\n' {
				j++
			}
			if j < n && runes[j] == '`' {
				inner := string(runes[i+1 : j])
				out.WriteString(lipgloss.NewStyle().Foreground(themeCyan).Background(lipgloss.Color("#1a1a1a")).Render(inner))
				i = j + 1
			} else {
				out.WriteRune(runes[i])
				i++
			}

		case i+7 <= n && string(runes[i:i+7]) == "http://":
			j := i
			for j < n && runes[j] != ' ' && runes[j] != '\n' {
				j++
			}
			url := string(runes[i:j])
			out.WriteString(lipgloss.NewStyle().Foreground(themeCyan).Underline(true).Render(url))
			i = j

		case i+8 <= n && string(runes[i:i+8]) == "https://":
			j := i
			for j < n && runes[j] != ' ' && runes[j] != '\n' {
				j++
			}
			url := string(runes[i:j])
			out.WriteString(lipgloss.NewStyle().Foreground(themeCyan).Underline(true).Render(url))
			i = j

		default:
			out.WriteRune(runes[i])
			i++
		}
	}

	return out.String()
}
