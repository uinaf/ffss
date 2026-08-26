package target

import (
	"io"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

type byteRange struct {
	start int
	end   int
}

type objectIDRedactingReader struct {
	value    string
	ranges   []byteRange
	position int
	rangeAt  int
}

func (reader *objectIDRedactingReader) Read(buffer []byte) (int, error) {
	if reader.position >= len(reader.value) {
		return 0, io.EOF
	}
	count := copy(buffer, reader.value[reader.position:])
	chunkStart := reader.position
	chunkEnd := chunkStart + count
	for reader.rangeAt < len(reader.ranges) && reader.ranges[reader.rangeAt].end <= chunkStart {
		reader.rangeAt++
	}
	for index := reader.rangeAt; index < len(reader.ranges); index++ {
		current := reader.ranges[index]
		if current.start >= chunkEnd {
			break
		}
		start := max(current.start, chunkStart) - chunkStart
		end := min(current.end, chunkEnd) - chunkStart
		for index := start; index < end; index++ {
			buffer[index] = 'x'
		}
	}
	reader.position = chunkEnd
	return count, nil
}

func gitIndexObjectIDRanges(diff string, offset int) []byteRange {
	var ranges []byteRange
	position := 0
	for len(diff) > 0 {
		line, remaining, found := strings.Cut(diff, "\n")
		ids, ok := gitIndexLineObjectIDs(line)
		if ok {
			oldStart := offset + position + len("index ")
			oldEnd := oldStart + len(ids[0])
			newStart := oldEnd + len("..")
			ranges = append(ranges,
				byteRange{start: oldStart, end: oldEnd},
				byteRange{start: newStart, end: newStart + len(ids[1])},
			)
		}
		if !found {
			break
		}
		position += len(line) + 1
		diff = remaining
	}
	return ranges
}

type objectIDCandidateMatcher struct {
	candidates map[string]struct{}
	run        []byte
}

func (matcher *objectIDCandidateMatcher) boundary() {
	matcher.run = matcher.run[:0]
}

func (matcher *objectIDCandidateMatcher) write(value string) bool {
	for index := 0; index < len(value); index++ {
		character := value[index]
		if !isLowerHex(character) {
			matcher.boundary()
			continue
		}
		if len(matcher.run) == 64 {
			copy(matcher.run, matcher.run[1:])
			matcher.run[63] = character
		} else {
			matcher.run = append(matcher.run, character)
		}
		if len(matcher.run) >= 40 {
			candidate := string(matcher.run[len(matcher.run)-40:])
			if _, ok := matcher.candidates[candidate]; ok {
				return true
			}
		}
		if len(matcher.run) == 64 {
			if _, ok := matcher.candidates[string(matcher.run)]; ok {
				return true
			}
		}
	}
	return false
}

var truffleHogHTMLHighSignalAttributes = map[string]bool{
	"href":    true,
	"src":     true,
	"action":  true,
	"value":   true,
	"content": true,
	"alt":     true,
	"title":   true,
}

var truffleHogHTMLBlockElements = map[string]bool{
	"p": true, "div": true, "br": true, "hr": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"li": true, "ol": true, "ul": true,
	"tr": true, "td": true, "th": true, "table": true, "thead": true, "tbody": true, "tfoot": true,
	"blockquote": true, "section": true, "article": true, "header": true, "footer": true,
	"pre": true, "address": true, "figcaption": true, "figure": true,
	"details": true, "summary": true, "main": true, "nav": true, "aside": true,
	"form": true, "fieldset": true, "legend": true,
	"dd": true, "dt": true, "dl": true,
	"script": true, "style": true,
}

var truffleHogHTMLInvisibleReplacer = strings.NewReplacer(
	"\u200B", "",
	"\u200C", "",
	"\u200D", "",
	"\uFEFF", "",
	"\u00AD", "",
	"\u2060", "",
	"\u200E", "",
	"\u200F", "",
)

// htmlDecodedPayloadContainsObjectID mirrors the candidate-bearing parts of
// TruffleHog's HTML decoder after structural Git object IDs are redacted.
func htmlDecodedPayloadContainsObjectID(payload string, redactions []byteRange, candidates map[string]struct{}) bool {
	reader := &objectIDRedactingReader{value: payload, ranges: redactions}
	tokenizer := html.NewTokenizer(reader)
	matcher := objectIDCandidateMatcher{candidates: candidates, run: make([]byte, 0, 64)}
	for {
		tokenType := tokenizer.Next()
		if tokenType == html.ErrorToken {
			return tokenizer.Err() != io.EOF
		}
		token := tokenizer.Token()
		switch tokenType {
		case html.TextToken:
			if matcher.write(truffleHogHTMLInvisibleReplacer.Replace(token.Data)) {
				return true
			}
		case html.CommentToken:
			matcher.boundary()
			if matcher.write(truffleHogHTMLInvisibleReplacer.Replace(strings.TrimSpace(token.Data))) {
				return true
			}
			matcher.boundary()
		case html.StartTagToken, html.SelfClosingTagToken:
			if truffleHogHTMLBlockElements[token.Data] || hasSyntaxHighlightAttribute(token.Attr) {
				matcher.boundary()
			}
			if htmlAttributesContainObjectID(&matcher, token) {
				return true
			}
			if tokenType == html.SelfClosingTagToken && truffleHogHTMLBlockElements[token.Data] {
				matcher.boundary()
			}
		case html.EndTagToken:
			if truffleHogHTMLBlockElements[token.Data] {
				matcher.boundary()
			}
		}
	}
}

func hasSyntaxHighlightAttribute(attributes []html.Attribute) bool {
	for _, attribute := range attributes {
		if attribute.Key != "class" {
			continue
		}
		for _, class := range strings.Fields(attribute.Val) {
			if strings.HasPrefix(class, "hljs-") {
				return true
			}
		}
	}
	return false
}

func htmlAttributesContainObjectID(matcher *objectIDCandidateMatcher, token html.Token) bool {
	namespaced := strings.Contains(token.Data, ":")
	for _, attribute := range token.Attr {
		if !namespaced && !truffleHogHTMLHighSignalAttributes[attribute.Key] && !strings.HasPrefix(attribute.Key, "data-") {
			continue
		}
		value := strings.TrimSpace(attribute.Val)
		if value == "" || value == "#" {
			continue
		}
		if decoded, err := url.PathUnescape(value); err == nil {
			value = decoded
		}
		matcher.boundary()
		if matcher.write(truffleHogHTMLInvisibleReplacer.Replace(value)) {
			return true
		}
		matcher.boundary()
	}
	return false
}
