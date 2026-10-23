package api

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/asilamuxitdinova/online-pharmacy/backend/openapi"
)

type specDoc struct {
	OpenAPI string                    `yaml:"openapi"`
	Servers []struct{ URL string }    `yaml:"servers"`
	Paths   map[string]map[string]any `yaml:"paths"`
}

var snake = regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)*$`)

// The spec is the AI's tool list, so it must match the router exactly and
// follow the rules in the testing guide.
func TestSpecMatchesRoutes(t *testing.T) {
	var doc specDoc
	if err := yaml.Unmarshal([]byte(openapi.Spec), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Servers) != 1 || doc.Servers[0].URL != openapi.ServerPlaceholder {
		t.Fatalf("want exactly one server, the placeholder; got %v", doc.Servers)
	}
	if len(openapi.Spec) > 5<<20 {
		t.Fatal("the spec must stay under the 5 MB upload limit")
	}

	inSpec := map[string]bool{}
	ids := map[string]string{}
	authOps := map[string]bool{}
	for path, item := range doc.Paths {
		for method, raw := range item {
			op, ok := raw.(map[string]any)
			if method == "parameters" || !ok {
				continue
			}
			key := strings.ToUpper(method) + " " + path
			inSpec[key] = true
			id, _ := op["operationId"].(string)
			if !snake.MatchString(id) {
				t.Errorf("%s: operationId %q is not snake_case", key, id)
			}
			if other, dup := ids[id]; dup {
				t.Errorf("operationId %q used by %s and %s", id, key, other)
			}
			ids[id] = key
			if s, _ := op["summary"].(string); s == "" {
				t.Errorf("%s has no summary", key)
			}
			if op["x-frege-auth-op"] == true {
				authOps[id] = true
			}
			if rb, ok := op["requestBody"].(map[string]any); ok && id != "oauth_token" {
				content, _ := rb["content"].(map[string]any)
				if _, ok := content["application/json"]; !ok || len(content) != 1 {
					t.Errorf("%s: request bodies must be JSON only", key)
				}
			}
		}
	}

	e := newEnv(t)
	routed := map[string]bool{}
	for _, r := range e.srv.Routes() {
		routed[r.Pattern] = true
		if r.InSpec && !inSpec[r.Pattern] {
			t.Errorf("route %s is missing from the spec", r.Pattern)
		}
		if !r.InSpec && inSpec[r.Pattern] {
			t.Errorf("route %s is hidden but appears in the spec", r.Pattern)
		}
	}
	for key := range inSpec {
		if !routed[key] {
			t.Errorf("spec operation %s has no route", key)
		}
	}

	wantAuth := []string{"register_account", "sign_in_with_password", "request_sign_in_code", "verify_sign_in_code",
		"refresh_access_token", "start_cookie_session", "oauth_authorize", "oauth_token"}
	for _, id := range wantAuth {
		if !authOps[id] {
			t.Errorf("%s must be marked x-frege-auth-op", id)
		}
	}
	for id := range authOps {
		if !slices.Contains(wantAuth, id) {
			t.Errorf("%s is marked x-frege-auth-op but is not sign-in machinery", id)
		}
	}
}
