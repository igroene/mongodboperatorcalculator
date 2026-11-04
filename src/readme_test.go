package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

var updateReadme = flag.Bool("update-readme", false, "rewrite README response examples from current output")

// README examples are marked with <!-- request:NAME --> and
// <!-- response:NAME --> comments, each directly followed by a fenced block.
var readmeBlock = regexp.MustCompile("(?s)<!-- (request|response):([a-z0-9-]+) -->\n```([a-z]*)\n(.*?)```")

func TestReadmeExamplesMatchOutput(t *testing.T) {
	path := "../README.md"
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	readme := string(raw)
	requests := map[string]string{}
	for _, m := range readmeBlock.FindAllStringSubmatch(readme, -1) {
		if m[1] == "request" {
			requests[m[2]] = m[4]
		}
	}
	if len(requests) == 0 {
		t.Fatal("no README examples found")
	}
	seen := map[string]bool{}
	updated := readmeBlock.ReplaceAllStringFunc(readme, func(block string) string {
		m := readmeBlock.FindStringSubmatch(block)
		if m[1] != "response" {
			return block
		}
		name, lang, documented := m[2], m[3], m[4]
		seen[name] = true
		body, ok := requests[name]
		if !ok {
			t.Errorf("README response %q has no matching request", name)
			return block
		}
		req := httptest.NewRequest(http.MethodPost, "/calculator", strings.NewReader(body))
		res := httptest.NewRecorder()
		routes(newLogger("ERROR")).ServeHTTP(res, req)
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
				t.Errorf("%s: README response is not valid JSON: %v", name, err)
			}
			_ = json.Unmarshal([]byte(actual), &got)
			if !reflect.DeepEqual(want, got) && !*updateReadme {
				t.Errorf("README example %q is out of date; run go test ./src -run TestReadmeExamples -update-readme\n%s", name, actual)
			}
		} else if strings.TrimSpace(documented) != strings.TrimSpace(actual) && !*updateReadme {
			t.Errorf("README example %q is out of date; run go test ./src -run TestReadmeExamples -update-readme\n%s", name, actual)
		}
		if !strings.HasSuffix(actual, "\n") {
			actual += "\n"
		}
		return "<!-- response:" + name + " -->\n```" + lang + "\n" + actual + "```"
	})
	for name := range requests {
		if !seen[name] {
			t.Errorf("README request %q has no matching response", name)
		}
	}
	if *updateReadme && updated != readme {
		if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
