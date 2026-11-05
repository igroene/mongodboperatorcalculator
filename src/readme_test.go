package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var updateReadme = flag.Bool("update-readme", false, "rewrite README and docs response examples from current output")

// Documentation examples are marked with <!-- request:NAME --> and
// <!-- response:NAME --> comments, each directly followed by a fenced block.
// A response marker may carry the expected HTTP status, for example
// <!-- response:overload status=422 -->; it defaults to 200.
var readmeBlock = regexp.MustCompile("(?s)<!-- (request|response):([a-z0-9-]+)(?: status=([0-9]+))? -->\n```([a-z]*)\n(.*?)```")

func TestReadmeExamplesMatchOutput(t *testing.T) {
	paths, err := filepath.Glob("../docs/*.md")
	if err != nil {
		t.Fatal(err)
	}
	paths = append([]string{"../README.md"}, paths...)
	examples := 0
	for _, path := range paths {
		t.Run(filepath.ToSlash(path), func(t *testing.T) {
			examples += checkDocumentationExamples(t, path)
		})
	}
	if examples == 0 {
		t.Fatal("no documentation examples found")
	}
}

func checkDocumentationExamples(t *testing.T, path string) int {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	readme := string(raw)
	requests := map[string]string{}
	for _, m := range readmeBlock.FindAllStringSubmatch(readme, -1) {
		if m[1] == "request" {
			if _, exists := requests[m[2]]; exists {
				t.Errorf("duplicate documentation request %q", m[2])
			}
			requests[m[2]] = m[5]
		}
	}
	seen := map[string]bool{}
	updated := readmeBlock.ReplaceAllStringFunc(readme, func(block string) string {
		m := readmeBlock.FindStringSubmatch(block)
		if m[1] != "response" {
			return block
		}
		name, status, lang, documented := m[2], m[3], m[4], m[5]
		if seen[name] {
			t.Errorf("duplicate documentation response %q", name)
		}
		seen[name] = true
		body, ok := requests[name]
		if !ok {
			t.Errorf("documentation response %q has no matching request", name)
			return block
		}
		wantStatus := http.StatusOK
		if status != "" {
			wantStatus, _ = strconv.Atoi(status)
		}
		req := httptest.NewRequest(http.MethodPost, "/calculator", strings.NewReader(body))
		res := httptest.NewRecorder()
		routes(newLogger("ERROR")).ServeHTTP(res, req)
		if res.Code != wantStatus {
			t.Errorf("documentation example %q documents HTTP %d, got %d", name, wantStatus, res.Code)
		}
		actual := res.Body.String()
		if lang == "json" {
			var pretty bytes.Buffer
			if err := json.Indent(&pretty, []byte(actual), "", "  "); err != nil {
				t.Errorf("%s: response is not JSON: %v", name, err)
				return block
			}
			actual = pretty.String() + "\n"
			var want, got any
			if err := json.Unmarshal([]byte(documented), &want); err != nil {
				t.Errorf("%s: documentation response is not valid JSON: %v", name, err)
			}
			_ = json.Unmarshal([]byte(actual), &got)
			if !reflect.DeepEqual(want, got) && !*updateReadme {
				t.Errorf("documentation example %q is out of date; run go test ./src -run TestReadmeExamples -update-readme\n%s", name, actual)
			}
		} else if strings.TrimSpace(documented) != strings.TrimSpace(actual) && !*updateReadme {
			t.Errorf("documentation example %q is out of date; run go test ./src -run TestReadmeExamples -update-readme\n%s", name, actual)
		}
		if !strings.HasSuffix(actual, "\n") {
			actual += "\n"
		}
		marker := "<!-- response:" + name
		if status != "" {
			marker += " status=" + status
		}
		return marker + " -->\n```" + lang + "\n" + actual + "```"
	})
	for name := range requests {
		if !seen[name] {
			t.Errorf("documentation request %q has no matching response", name)
		}
	}
	if *updateReadme && updated != readme {
		if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return len(requests)
}
