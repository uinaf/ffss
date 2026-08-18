package serve

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/uinaf/ffss/cli/slopmachine/internal/machine"
)

// The pin and the digests of the vendored pair. scripts/sync-design-css.sh is
// the only writer of these three values, so a moved version cannot come apart
// from the files it fetched. Hand-patching the stylesheet is what turns a shared
// design system back into one repo's private CSS, so a local edit has to fail
// here and be re-synced from the registry instead.
const designCSSVersion = "1.13.5"

var designCSSDigests = map[string]string{
	"static/tokens.css":     "3abce8d3b44c3317e11b6817ae6bab7bcec57e0d9d9f198714068c4243c9c7a4",
	"static/components.css": "fd2f8854cc05d6516b367d758c5a6fba69858d10fe49938d01529cd7c4d36df5",
}

func TestVendoredDesignCSSIsUnmodified(t *testing.T) {
	for name, want := range designCSSDigests {
		body, err := assets.ReadFile(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		sum := sha256.Sum256(body)
		if got := hex.EncodeToString(sum[:]); got != want {
			t.Errorf("%s: digest %s, want %s (@uinaf/design@%s)\n"+
				"re-sync instead of editing: ./scripts/sync-design-css.sh",
				name, got, want, designCSSVersion)
		}
	}
}

var (
	classAttr = regexp.MustCompile(`class="([^"]*)"`)
	// A class attribute may be part literal and part action, as in
	// class="{{statusDot .State .Open}}". Drop the actions and check the rest.
	templateAction = regexp.MustCompile(`{{[^{}]*}}`)
)

// Every class this package emits has to be defined by the vendored pair or by
// app.css. A utility renamed upstream otherwise renders unstyled and nothing
// fails: the projector is Go templates and Go strings, neither of which the
// design system's own markup linter can see. Bare names count too — the
// crumb separator ships as `.sep`, not `.u-sep`.
func TestEmittedClassesExistInVendoredCSS(t *testing.T) {
	sheet := servedCSS(t)

	used := map[string][]string{}
	entries, err := assets.ReadDir("templates")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		body, err := assets.ReadFile("templates/" + entry.Name())
		if err != nil {
			t.Fatalf("%s: %v", entry.Name(), err)
		}
		for _, match := range classAttr.FindAllStringSubmatch(string(body), -1) {
			for _, class := range strings.Fields(templateAction.ReplaceAllString(match[1], " ")) {
				used[class] = append(used[class], entry.Name())
			}
		}
	}

	// statusDotClass builds its classes in Go, so no template carries them.
	states := []string{
		string(machine.StateBlocked),
		string(machine.StateNeedsDecision),
		string(machine.StateRework),
		string(machine.StateRunDone),
		string(machine.StateIntake),
	}
	for _, state := range states {
		for _, open := range []bool{true, false} {
			for _, class := range strings.Fields(statusDotClass(state, open)) {
				used[class] = append(used[class], "statusDotClass")
			}
		}
	}

	var missing []string
	for class := range used {
		if !definedIn(sheet, class) {
			missing = append(missing, class)
		}
	}
	sort.Strings(missing)
	for _, class := range missing {
		t.Errorf("%s: not in @uinaf/design@%s or app.css, used by %s",
			class, designCSSVersion, strings.Join(used[class], ", "))
	}
}

// The vendored pair plus the local chrome: every stylesheet the browser gets.
func servedCSS(t *testing.T) string {
	t.Helper()
	var sheet strings.Builder
	for _, name := range []string{"static/tokens.css", "static/components.css", "static/app.css"} {
		body, err := assets.ReadFile(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		sheet.Write(body)
	}
	return sheet.String()
}

// A whole-selector match: `.u-dot` must not be satisfied by `.u-dot--ok`.
func definedIn(sheet, class string) bool {
	pattern := regexp.MustCompile(`\.` + regexp.QuoteMeta(class) + `([^\w-]|$)`)
	return pattern.MatchString(sheet)
}
