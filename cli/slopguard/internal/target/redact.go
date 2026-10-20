package target

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/uinaf/ffss/cli/slopguard/internal/protocol"
)

const redactedValue = "[REDACTED]"

var environmentAssignment = regexp.MustCompile(`^[ \t]*(?:export[ \t]+)?[A-Za-z_][A-Za-z0-9_.-]*[ \t]*=`)

// environmentFilePath reports a changed dotenv file that enters the bundle with
// its values redacted instead of failing the target.
func environmentFilePath(value string) bool {
	base := strings.ToLower(path.Base(value))
	return (base == ".env" || strings.HasPrefix(base, ".env.")) && !environmentTemplatePath(value)
}

func reviewablePath(value string) bool {
	return !sensitivePath(value) || environmentTemplatePath(value) || environmentFilePath(value)
}

func redactEnvironment(content []byte) []byte {
	return bytes.Join(redactEnvironmentLines(bytes.Split(content, []byte{'\n'})), []byte{'\n'})
}

// redactEnvironmentLines keeps key names only when no line leaves a quote
// open. Then no value can continue onto another line in any dotenv dialect, so
// every KEY= prefix is a real key. Otherwise every line is redacted whole.
func redactEnvironmentLines(lines [][]byte) [][]byte {
	keys := true
	for _, line := range lines {
		if !bytes.HasPrefix(bytes.TrimLeft(line, " \t"), []byte{'#'}) && leavesQuoteOpen(line) {
			keys = false
			break
		}
	}
	redacted := make([][]byte, len(lines))
	for index, line := range lines {
		text, carriage := bytes.CutSuffix(line, []byte{'\r'})
		switch end := environmentAssignment.FindIndex(text); {
		case len(bytes.TrimSpace(text)) == 0:
			redacted[index] = append([]byte(nil), text...)
		case keys && end != nil:
			redacted[index] = append(append([]byte(nil), text[:end[1]]...), redactedValue...)
		default:
			redacted[index] = []byte(redactedValue)
		}
		if carriage {
			redacted[index] = append(redacted[index], '\r')
		}
	}
	return redacted
}

// leavesQuoteOpen treats a quote preceded by a backslash as escaped, whatever
// the dialect, so a disagreement with the file's parser over-redacts.
func leavesQuoteOpen(line []byte) bool {
	var quote byte
	for index, character := range line {
		switch {
		case quote == 0 && (character == '"' || character == '\'' || character == '`'):
			quote = character
		case quote != 0 && character == quote && line[index-1] != '\\':
			quote = 0
		}
	}
	return quote != 0
}

// redactEnvironmentDiff rewrites the dotenv sections of a --unified=0 diff.
// Hunks start mid-file, so each changed line is looked up in a full-context
// diff of the same file and replaced by its redaction in whole-file order.
// Blob IDs and hunk function context are dropped because both derive from
// values.
func (collector *Collector) redactEnvironmentDiff(ctx context.Context, root string, plan *targetPlan, diff []byte, commandPaths [][]string, limit int64) ([]byte, error) {
	contextCommands := diffCommands(plan, "--no-color", "--unified="+strconv.Itoa(protocol.MaxLineNumber))
	var sectionCommands []int
	var sectionPaths []string
	for command, paths := range commandPaths {
		for _, changed := range paths {
			sectionCommands = append(sectionCommands, command)
			sectionPaths = append(sectionPaths, changed)
		}
	}
	output := make([]byte, 0, len(diff))
	section := -1
	var file *environmentDiffFile
	for start := 0; start < len(diff); {
		end := bytes.IndexByte(diff[start:], '\n')
		next := len(diff)
		if end < 0 {
			end = len(diff)
		} else {
			end += start
			next = end + 1
		}
		line := diff[start:end]
		if bytes.HasPrefix(line, []byte("diff --git ")) {
			section++
			if section >= len(sectionPaths) {
				return nil, fmt.Errorf("diff has more file sections than path inventory")
			}
			file = nil
			if changed := sectionPaths[section]; environmentFilePath(changed) {
				var err error
				file, err = collector.environmentDiffFile(ctx, root, plan, contextCommands[sectionCommands[section]], changed, limit)
				if err != nil {
					return nil, fmt.Errorf("redact environment file %q: %w", changed, err)
				}
			}
		} else if file != nil {
			redacted, err := file.redact(line)
			if err != nil {
				return nil, fmt.Errorf("redact environment file %q: %w", sectionPaths[section], err)
			}
			line = redacted
		}
		output = append(output, line...)
		output = append(output, diff[end:next]...)
		start = next
	}
	return output, nil
}

type environmentDiffFile struct {
	oldRaw, newRaw           [][]byte
	oldRedacted, newRedacted [][]byte
	inHunk                   bool
	oldLine, newLine         int
}

func (collector *Collector) environmentDiffFile(ctx context.Context, root string, plan *targetPlan, command []string, changed string, limit int64) (*environmentDiffFile, error) {
	arguments := append(append([]string(nil), command...), ":(literal)"+changed)
	output := newLimitBuffer(limit)
	if err := collector.git.runSandboxWithAttributesTo(ctx, root, plan.sandbox, nil, output, arguments...); err != nil {
		return nil, fmt.Errorf("collect full-context diff: %w", err)
	}
	if output.exceeded {
		return nil, fmt.Errorf("full-context diff exceeds %d bytes", limit)
	}
	file := &environmentDiffFile{}
	inHunk := false
	var oldLine, newLine int
	for _, line := range bytes.Split(output.Bytes(), []byte{'\n'}) {
		if bytes.HasPrefix(line, []byte("@@ ")) {
			oldStart, newStart, err := parseHunkHeader(line)
			if err != nil {
				return nil, err
			}
			inHunk, oldLine, newLine = true, oldStart, newStart
			continue
		}
		if !inHunk || len(line) == 0 || line[0] == '\\' {
			continue
		}
		if line[0] == ' ' || line[0] == '-' {
			if oldLine != len(file.oldRaw)+1 {
				return nil, fmt.Errorf("full-context diff skips old lines")
			}
			file.oldRaw = append(file.oldRaw, line[1:])
			oldLine++
		}
		if line[0] == ' ' || line[0] == '+' {
			if newLine != len(file.newRaw)+1 {
				return nil, fmt.Errorf("full-context diff skips new lines")
			}
			file.newRaw = append(file.newRaw, line[1:])
			newLine++
		}
		if line[0] != ' ' && line[0] != '-' && line[0] != '+' {
			return nil, fmt.Errorf("full-context diff has an unexpected line")
		}
	}
	file.oldRedacted = redactEnvironmentLines(file.oldRaw)
	file.newRedacted = redactEnvironmentLines(file.newRaw)
	return file, nil
}

func (file *environmentDiffFile) redact(line []byte) ([]byte, error) {
	if bytes.HasPrefix(line, []byte("@@ ")) {
		oldStart, newStart, err := parseHunkHeader(line)
		if err != nil {
			return nil, err
		}
		file.inHunk, file.oldLine, file.newLine = true, oldStart, newStart
		return hunkHeader.Find(line), nil
	}
	if !file.inHunk {
		if bytes.HasPrefix(line, []byte("index ")) {
			return []byte("index " + redactedValue), nil
		}
		return line, nil
	}
	switch {
	case len(line) > 0 && line[0] == '-':
		redacted, err := lookupRedacted(file.oldRaw, file.oldRedacted, file.oldLine, line[1:])
		file.oldLine++
		return append([]byte{'-'}, redacted...), err
	case len(line) > 0 && line[0] == '+':
		redacted, err := lookupRedacted(file.newRaw, file.newRedacted, file.newLine, line[1:])
		file.newLine++
		return append([]byte{'+'}, redacted...), err
	case len(line) > 0 && line[0] == '\\':
		return line, nil
	default:
		return nil, fmt.Errorf("diff has an unexpected hunk line")
	}
}

func lookupRedacted(raw, redacted [][]byte, line int, content []byte) ([]byte, error) {
	if line < 1 || line > len(raw) || !bytes.Equal(raw[line-1], content) {
		return nil, fmt.Errorf("diff line %d does not match the full-context diff", line)
	}
	return redacted[line-1], nil
}

func parseHunkHeader(line []byte) (int, int, error) {
	matches := hunkHeader.FindSubmatch(line)
	if matches == nil {
		return 0, 0, fmt.Errorf("malformed diff hunk header")
	}
	oldStart, err := lineField(string(matches[1]), false)
	if err != nil {
		return 0, 0, fmt.Errorf("diff hunk old start: %w", err)
	}
	newStart, err := lineField(string(matches[3]), false)
	if err != nil {
		return 0, 0, fmt.Errorf("diff hunk new start: %w", err)
	}
	return oldStart, newStart, nil
}
