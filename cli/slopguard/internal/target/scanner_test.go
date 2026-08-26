package target

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/uinaf/ffss/cli/slopguard/internal/protocol"
)

const (
	testOldObjectID    = "e7cb395913bbb23a138f5897" + "4ecd36dd155a6ace"
	testNewObjectID    = "48e3179e5fd9a1023e9df68a" + "74883a603f1d92da"
	testCloudflarePath = "src/example/cloudflare." + "py"
)

func TestTruffleHogFindingsIgnoreCloudflareMatchesOnGitObjectIDs(t *testing.T) {
	t.Parallel()
	indexLine := "index " + testOldObjectID + ".." + testNewObjectID + " 100644"
	payload := scannerTestPayload(t, "diff --git a/"+testCloudflarePath+" b/"+testCloudflarePath+"\n"+indexLine+"\n--- a/"+testCloudflarePath+"\n+++ b/"+testCloudflarePath+"\n@@ -1 +1 @@\n-old\n+new\n", nil)
	output := scannerTestFinding(t, "CloudflareApiToken", testOldObjectID, scannerTestLine(t, payload, indexLine))

	found, err := truffleHogFindingsContainSecret(output, payload)
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatal("Git-generated object ID was treated as a secret")
	}
}

func TestTruffleHogFindingsIgnoreHTMLDecoderDuplicateOfGitObjectID(t *testing.T) {
	t.Parallel()
	firstIndexLine := "index " + strings.Repeat("1", 40) + ".." + strings.Repeat("2", 40) + " 100644"
	targetIndexLine := "index " + testOldObjectID + ".." + testNewObjectID + " 100644"
	payload := scannerTestPayload(t, "diff --git a/page.html b/page.html\n"+firstIndexLine+"\n--- a/page.html\n+++ b/page.html\n@@ -1 +1 @@\n-<p>old</p>\n+<p>new</p>\ndiff --git a/"+testCloudflarePath+" b/"+testCloudflarePath+"\n"+targetIndexLine+"\n--- a/"+testCloudflarePath+"\n+++ b/"+testCloudflarePath+"\n@@ -1 +1 @@\n-old\n+new\n", nil)
	output := scannerTestFindingWithDecoder(t, "CloudflareApiToken", "HTML", testOldObjectID, scannerTestLine(t, payload, firstIndexLine))
	output = append(output, scannerTestFindingWithDecoder(t, "CloudflareApiToken", "PLAIN", testOldObjectID, scannerTestLine(t, payload, targetIndexLine))...)

	found, err := truffleHogFindingsContainSecret(output, payload)
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatal("HTML-decoded duplicate of a Git-generated object ID was treated as a secret")
	}
}

func TestTruffleHogFindingsKeepUserControlledMatches(t *testing.T) {
	t.Parallel()
	indexLine := "index " + testOldObjectID + ".." + testNewObjectID + " 100644"
	tests := []struct {
		name     string
		diff     string
		contexts map[string][]byte
		needle   string
		detector string
		raw      string
	}{
		{
			name:     "added line",
			diff:     "diff --git a/" + testCloudflarePath + " b/" + testCloudflarePath + "\n" + indexLine + "\n--- a/" + testCloudflarePath + "\n+++ b/" + testCloudflarePath + "\n@@ -0,0 +1 @@\n+" + testOldObjectID + "\n",
			needle:   "+" + testOldObjectID,
			detector: "CloudflareApiToken",
			raw:      testOldObjectID,
		},
		{
			name:     "context file",
			diff:     "diff --git a/file.txt b/file.txt\n" + indexLine + "\n--- a/file.txt\n+++ b/file.txt\n@@ -1 +1 @@\n-old\n+new\n",
			contexts: map[string][]byte{"cloudflare.txt": []byte(indexLine + "\n")},
			needle:   "BEGIN UNTRUSTED-CONTEXT-FILE PATH-BYTES 14 cloudflare.txt",
			detector: "CloudflareApiToken",
			raw:      testOldObjectID,
		},
		{
			name:     "different detector",
			diff:     "diff --git a/" + testCloudflarePath + " b/" + testCloudflarePath + "\n" + indexLine + "\n--- a/" + testCloudflarePath + "\n+++ b/" + testCloudflarePath + "\n@@ -1 +1 @@\n-old\n+new\n",
			needle:   indexLine,
			detector: "GenericCredential",
			raw:      testOldObjectID,
		},
		{
			name:     "different raw value",
			diff:     "diff --git a/" + testCloudflarePath + " b/" + testCloudflarePath + "\n" + indexLine + "\n--- a/" + testCloudflarePath + "\n+++ b/" + testCloudflarePath + "\n@@ -1 +1 @@\n-old\n+new\n",
			needle:   indexLine,
			detector: "CloudflareApiToken",
			raw:      strings.Repeat("c", 40),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			payload := scannerTestPayload(t, test.diff, test.contexts)
			line := scannerTestLine(t, payload, test.needle)
			if test.name == "context file" {
				line++
			}
			output := scannerTestFinding(t, test.detector, test.raw, line)
			found, err := truffleHogFindingsContainSecret(output, payload)
			if err != nil {
				t.Fatal(err)
			}
			if !found {
				t.Fatal("user-controlled or non-metadata finding was ignored")
			}
		})
	}
}

func TestTruffleHogFindingsFailClosed(t *testing.T) {
	t.Parallel()
	payload := scannerTestPayload(t, "diff --git a/file.txt b/file.txt\nindex "+testOldObjectID+".."+testNewObjectID+" 100644\n", nil)
	if _, err := truffleHogFindingsContainSecret([]byte("not json"), payload); err == nil {
		t.Fatal("malformed TruffleHog output was accepted")
	}
}

func TestTruffleHogFindingsKeepOtherFindingAfterGitObjectID(t *testing.T) {
	t.Parallel()
	indexLine := "index " + testOldObjectID + ".." + testNewObjectID + " 100644"
	addedLine := "+" + strings.Repeat("c", 40)
	payload := scannerTestPayload(t, "diff --git a/"+testCloudflarePath+" b/"+testCloudflarePath+"\n"+indexLine+"\n--- a/"+testCloudflarePath+"\n+++ b/"+testCloudflarePath+"\n@@ -0,0 +1 @@\n"+addedLine+"\n", nil)
	output := scannerTestFinding(t, "CloudflareApiToken", testOldObjectID, scannerTestLine(t, payload, indexLine))
	output = append(output, scannerTestFinding(t, "CloudflareApiToken", strings.TrimPrefix(addedLine, "+"), scannerTestLine(t, payload, addedLine))...)

	found, err := truffleHogFindingsContainSecret(output, payload)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("Git object ID finding hid a later source finding")
	}
}

func TestGitIndexLineContainsObjectID(t *testing.T) {
	t.Parallel()
	sha256Old := strings.Repeat("a", 64)
	sha256New := strings.Repeat("b", 64)
	tests := []struct {
		name string
		line string
		raw  string
		want bool
	}{
		{name: "sha1 with mode", line: "index " + testOldObjectID + ".." + testNewObjectID + " 100644", raw: testOldObjectID, want: true},
		{name: "sha256 without mode", line: "index " + sha256Old + ".." + sha256New, raw: sha256New, want: true},
		{name: "added source line", line: "+index " + testOldObjectID + ".." + testNewObjectID + " 100644", raw: testOldObjectID},
		{name: "context source line", line: " index " + testOldObjectID + ".." + testNewObjectID + " 100644", raw: testOldObjectID},
		{name: "invalid mode", line: "index " + testOldObjectID + ".." + testNewObjectID + " secret", raw: testOldObjectID},
		{name: "different object", line: "index " + testOldObjectID + ".." + testNewObjectID + " 100644", raw: strings.Repeat("c", 40)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := gitIndexLineContainsObjectID(test.line, test.raw); got != test.want {
				t.Fatalf("gitIndexLineContainsObjectID() = %v, want %v", got, test.want)
			}
		})
	}
}

func scannerTestPayload(t *testing.T, diff string, contexts map[string][]byte) string {
	t.Helper()
	target := protocol.Target{Mode: protocol.TargetLocal, HeadRevision: strings.Repeat("a", 40)}
	payload, _, _, err := composeBundle(target, "sha256:"+strings.Repeat("b", 64), "Review the change.", []byte(diff), []Contributor{{Name: "diff", Bytes: int64(len(diff))}}, nil, nil, contexts, DefaultMaxBytes)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func scannerTestFinding(t *testing.T, detector, raw string, line int) []byte {
	return scannerTestFindingWithDecoder(t, detector, "PLAIN", raw, line)
}

func scannerTestFindingWithDecoder(t *testing.T, detector, decoder, raw string, line int) []byte {
	t.Helper()
	finding := truffleHogFinding{DetectorName: detector, Raw: raw}
	finding.SourceMetadata.Data.Filesystem.Line = line
	encoded, err := json.Marshal(struct {
		truffleHogFinding
		DecoderName string
	}{truffleHogFinding: finding, DecoderName: decoder})
	if err != nil {
		t.Fatal(err)
	}
	return append(encoded, '\n')
}

func scannerTestLine(t *testing.T, payload, needle string) int {
	t.Helper()
	index := strings.Index(payload, needle)
	if index < 0 {
		t.Fatalf("payload does not contain %q", needle)
	}
	return strings.Count(payload[:index], "\n") + 1
}
