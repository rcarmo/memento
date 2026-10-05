package tools

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectTemporaryPolicy(t *testing.T) {
	base := t.TempDir()
	run := func(t *testing.T, script string, extra ...string) (string, error) {
		t.Helper()
		cmd := exec.Command("bash", "-c", "source ./project-tmp.sh; "+script)
		for _, e := range os.Environ() {
			k := strings.SplitN(e, "=", 2)[0]
			switch k {
			case "PROJECT_TMP_ROOT", "PROJECT_TMP_BASE", "PROJECT_ORIGINAL_TMPDIR", "CI", "GITHUB_ACTIONS", "GITLAB_CI", "TF_BUILD", "CIRCLECI", "RUNNER_TEMP", "TMPDIR":
				continue
			}
			cmd.Env = append(cmd.Env, e)
		}
		cmd.Env = append(cmd.Env, extra...)
		out, err := cmd.CombinedOutput()
		return strings.TrimSpace(string(out)), err
	}
	cases := []struct {
		name, script, want string
		env                []string
		fail               bool
	}{
		{"base", "project_tmp_resolve memento-go", base + "/memento-go", []string{"PROJECT_TMP_BASE=" + base}, false},
		{"root", "project_tmp_resolve memento-go", base + "/memento-go", []string{"PROJECT_TMP_ROOT=" + base + "/memento-go"}, false},
		{"agree", "project_tmp_resolve memento-go", base + "/memento-go", []string{"PROJECT_TMP_BASE=" + base, "PROJECT_TMP_ROOT=" + base + "/memento-go"}, false},
		{"conflict", "project_tmp_resolve memento-go", "", []string{"PROJECT_TMP_BASE=" + base, "PROJECT_TMP_ROOT=" + base + "/other/memento-go"}, true},
		{"relative", "project_tmp_resolve memento-go", "", []string{"PROJECT_TMP_BASE=relative"}, true},
		{"empty", "project_tmp_resolve memento-go", "", []string{"PROJECT_TMP_ROOT="}, true},
		{"wrong-name", "project_tmp_resolve memento-go", "", []string{"PROJECT_TMP_ROOT=" + base}, true},
		{"ci-runner-first", "project_tmp_resolve memento-go", base + "/runner/memento-go", []string{"CI=true", "RUNNER_TEMP=" + base + "/runner", "TMPDIR=" + base + "/inherited"}, false},
		{"ci-inherited", "project_tmp_resolve memento-go", base + "/inherited/memento-go", []string{"CI=true", "TMPDIR=" + base + "/inherited"}, false},
		{"snapshot", "TMPDIR=/ignored/child; project_tmp_resolve memento-go", base + "/inherited/memento-go", []string{"CI=true", "TMPDIR=" + base + "/inherited"}, false},
		{"local-fallback", "project_tmp_resolve memento-go /nonexistent-ancestor/absent", "/tmp/memento-go", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := run(t, tc.script, tc.env...)
			if tc.fail {
				if err == nil {
					t.Fatalf("accepted invalid configuration: %s", out)
				}
				return
			}
			if err != nil || out != tc.want {
				t.Fatalf("got %q, %v; want %q", out, err, tc.want)
			}
		})
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(base, link); err != nil {
		t.Fatal(err)
	}
	if out, err := run(t, "project_tmp_resolve memento-go", "PROJECT_TMP_BASE="+link); err == nil {
		t.Fatalf("accepted symlink: %s", out)
	}
}
