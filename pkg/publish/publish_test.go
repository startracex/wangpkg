package publish

import (
	"encoding/json/v2"
	"net/url"
	"testing"
	"time"

	"github.com/startracex/ciinfo"
	"github.com/startracex/wangpkg/pkg/manifest"
	"google.golang.org/protobuf/encoding/protojson"
)

func testOptions(env map[string]string, vendors map[string]bool) Options {
	m := &manifest.Manifest{}
	m.Name = "@scope/demo"
	m.Version = "1.2.3"

	u, err := url.Parse("https://registry.npmjs.org")
	if err != nil {
		panic(err)
	}
	return Options{
		Manifest: m,
		CI:       ciinfo.Info{Env: env, Vendors: vendors},
		Registry: u,
		Tag:      "latest",
	}
}

func newGitHubEnv() map[string]string {
	return map[string]string{
		"GITHUB_ACTIONS":             "true",
		"GITHUB_SERVER_URL":          "https://github.com",
		"GITHUB_REPOSITORY":          "startracex/wangpkg",
		"GITHUB_WORKFLOW_REF":        "startracex/wangpkg/.github/workflows/release.yml@refs/heads/main",
		"GITHUB_REF":                 "refs/heads/main",
		"GITHUB_SHA":                 "0000000000000000000000000000000000000000",
		"GITHUB_RUN_ID":              "42",
		"GITHUB_RUN_ATTEMPT":         "1",
		"GITHUB_EVENT_NAME":          "push",
		"GITHUB_REPOSITORY_ID":       "111",
		"GITHUB_REPOSITORY_OWNER_ID": "222",
		"RUNNER_ENVIRONMENT":         "github-hosted",
	}
}

func newGitLabEnv() map[string]string {
	return map[string]string{
		"GITLAB_CI":                 "true",
		"CI_PROJECT_URL":            "https://gitlab.com/startracex/wangpkg",
		"CI_COMMIT_SHA":             "1111111111111111111111111111111111111111",
		"CI_JOB_NAME":               "publish",
		"CI_JOB_ID":                 "77",
		"CI_JOB_URL":                "https://gitlab.com/startracex/wangpkg/-/jobs/77",
		"CI_PIPELINE_ID":            "88",
		"CI_CONFIG_PATH":            ".gitlab-ci.yml",
		"CI_RUNNER_ID":              "9",
		"CI_RUNNER_DESCRIPTION":     "shared-runner",
		"CI_RUNNER_EXECUTABLE_ARCH": "linux/amd64",
		"CI_SERVER_URL":             "https://gitlab.com",
		"CI_PROJECT_PATH":           "startracex/wangpkg",
	}
}

func statementMap(t *testing.T, opts Options, sr sumResult) map[string]any {
	t.Helper()
	stmt, err := opts.buildProvenanceStatement(sr, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	if err != nil {
		t.Fatalf("buildProvenanceStatement: %v", err)
	}
	raw, err := protojson.Marshal(stmt)
	if err != nil {
		t.Fatalf("marshal statement: %v", err)
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode statement: %v", err)
	}
	return out
}

func TestBuildProvenanceStatementGitHub(t *testing.T) {
	env := newGitHubEnv()
	opts := testOptions(env, map[string]bool{"GITHUB_ACTIONS": true})
	got := statementMap(t, opts, sumResult{SHA512: "abc512"})

	if got["_type"] != inTotoTypeV1 {
		t.Errorf("_type = %v, want %q", got["_type"], inTotoTypeV1)
	}
	if got["predicateType"] != slsaV1 {
		t.Errorf("predicateType = %v, want %q", got["predicateType"], slsaV1)
	}

	subject, ok := got["subject"].([]any)
	if !ok || len(subject) != 1 {
		t.Fatalf("subject = %v", got["subject"])
	}
	first := subject[0].(map[string]any)
	if first["name"] != "pkg:npm/%40scope/demo@1.2.3" {
		t.Errorf("subject name = %v", first["name"])
	}
	if digest, _ := first["digest"].(map[string]any); digest["sha512"] != "abc512" {
		t.Errorf("subject digest = %v", first["digest"])
	}

	predicate := got["predicate"].(map[string]any)
	buildDef := predicate["buildDefinition"].(map[string]any)
	if buildDef["buildType"] != githubBuildType {
		t.Errorf("buildType = %v", buildDef["buildType"])
	}

	external := buildDef["externalParameters"].(map[string]any)
	workflow := external["workflow"].(map[string]any)
	if workflow["path"] != ".github/workflows/release.yml" {
		t.Errorf("workflow.path = %v", workflow["path"])
	}
	if workflow["ref"] != "refs/heads/main" {
		t.Errorf("workflow.ref = %v", workflow["ref"])
	}
	if workflow["repository"] != "https://github.com/startracex/wangpkg" {
		t.Errorf("workflow.repository = %v", workflow["repository"])
	}

	internal := buildDef["internalParameters"].(map[string]any)
	gh := internal["github"].(map[string]any)
	if gh["event_name"] != "push" || gh["repository_id"] != "111" || gh["repository_owner_id"] != "222" {
		t.Errorf("internalParameters.github = %v", gh)
	}

	deps := buildDef["resolvedDependencies"].([]any)
	if len(deps) != 1 {
		t.Fatalf("resolvedDependencies = %v", deps)
	}
	dep := deps[0].(map[string]any)
	if dep["uri"] != "git+https://github.com/startracex/wangpkg@refs/heads/main" {
		t.Errorf("resolvedDependencies uri = %v", dep["uri"])
	}
	if digest, _ := dep["digest"].(map[string]any); digest["gitCommit"] != env["GITHUB_SHA"] {
		t.Errorf("resolvedDependencies digest = %v", dep["digest"])
	}

	runDetails := predicate["runDetails"].(map[string]any)
	builder := runDetails["builder"].(map[string]any)
	if builder["id"] != "https://github.com/actions/runner/github-hosted" {
		t.Errorf("builder.id = %v", builder["id"])
	}
	metadata := runDetails["metadata"].(map[string]any)
	wantInvocation := "https://github.com/startracex/wangpkg/actions/runs/42/attempts/1"
	if metadata["invocationId"] != wantInvocation {
		t.Errorf("metadata.invocationId = %v, want %q", metadata["invocationId"], wantInvocation)
	}
	if metadata["startedOn"] == nil || metadata["finishedOn"] == nil {
		t.Errorf("metadata timestamps are missing: %v", metadata)
	}
}

func TestBuildProvenanceStatementGitLab(t *testing.T) {
	env := newGitLabEnv()
	opts := testOptions(env, map[string]bool{"GITLAB": true})
	got := statementMap(t, opts, sumResult{SHA512: "abc512"})

	if got["_type"] != inTotoTypeV01 {
		t.Errorf("_type = %v, want %q", got["_type"], inTotoTypeV01)
	}
	if got["predicateType"] != slsaV02 {
		t.Errorf("predicateType = %v, want %q", got["predicateType"], slsaV02)
	}

	predicate := got["predicate"].(map[string]any)
	if predicate["buildType"] != gitlabBuildType {
		t.Errorf("buildType = %v", predicate["buildType"])
	}
	if builder := predicate["builder"].(map[string]any); builder["id"] != "https://gitlab.com/startracex/wangpkg/-/runners/9" {
		t.Errorf("builder.id = %v", builder["id"])
	}

	invocation := predicate["invocation"].(map[string]any)
	configSource := invocation["configSource"].(map[string]any)
	if configSource["uri"] != "git+https://gitlab.com/startracex/wangpkg" {
		t.Errorf("configSource.uri = %v", configSource["uri"])
	}
	if configSource["entryPoint"] != "publish" {
		t.Errorf("configSource.entryPoint = %v", configSource["entryPoint"])
	}
	if digest, _ := configSource["digest"].(map[string]any); digest["sha1"] != env["CI_COMMIT_SHA"] {
		t.Errorf("configSource.digest = %v", configSource["digest"])
	}
	if invocation["parameters"] == nil {
		t.Error("invocation.parameters is missing")
	}
	environment := invocation["environment"].(map[string]any)
	if environment["project"] != "startracex/wangpkg" {
		t.Errorf("environment.project = %v", environment["project"])
	}

	metadata := predicate["metadata"].(map[string]any)
	if metadata["buildInvocationId"] != env["CI_JOB_URL"] {
		t.Errorf("metadata.buildInvocationId = %v", metadata["buildInvocationId"])
	}
	completeness := metadata["completeness"].(map[string]any)
	if completeness["parameters"] != true || completeness["environment"] != true || completeness["materials"] != false {
		t.Errorf("completeness = %v", completeness)
	}

	materials := predicate["materials"].([]any)
	if len(materials) != 1 {
		t.Fatalf("materials = %v", materials)
	}
	if uri := materials[0].(map[string]any)["uri"]; uri != "git+https://gitlab.com/startracex/wangpkg" {
		t.Errorf("materials[0].uri = %v", uri)
	}
}

func TestBuildProvenanceStatementUnsupportedCI(t *testing.T) {
	opts := testOptions(nil, nil)
	if _, err := opts.buildProvenanceStatement(sumResult{}, time.Now()); err == nil {
		t.Error("expected an error for an unsupported CI provider")
	}
}

func TestNewStruct(t *testing.T) {
	got, err := newStruct(map[string]any{
		"nested": map[string]string{"a": "1"},
		"list":   []string{"x", "y"},
		"flag":   true,
	})
	if err != nil {
		t.Fatalf("newStruct: %v", err)
	}
	nested, ok := got.AsMap()["nested"].(map[string]any)
	if !ok {
		t.Fatalf("nested = %#v", got.AsMap()["nested"])
	}
	if nested["a"] != "1" {
		t.Errorf("nested.a = %v", nested["a"])
	}
	if got.AsMap()["flag"] != true {
		t.Errorf("flag = %v", got.AsMap()["flag"])
	}
}

func TestSignDSSERequiresIdentityToken(t *testing.T) {
	if _, err := SignDSSE([]byte("{}"), SignOptions{}); err == nil {
		t.Error("expected an error when no identity token is provided")
	}
}

func TestPkgURL(t *testing.T) {
	tests := []struct {
		name, version, want string
	}{
		{name: "@scope/demo", version: "1.2.3", want: "pkg:npm/%40scope/demo@1.2.3"},
		{name: "demo", version: "0.0.1", want: "pkg:npm/demo@0.0.1"},
	}
	for _, tt := range tests {
		if got := pkgurl(tt.name, tt.version); got != tt.want {
			t.Errorf("pkgurl(%q,%q) = %q, want %q", tt.name, tt.version, got, tt.want)
		}
	}
}
