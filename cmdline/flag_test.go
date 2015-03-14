package cmdline

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestEnvFlagParser(t *testing.T) {
	type testCase struct {
		input, want string
	}
	testCases := []testCase{
		testCase{"${TEST}", "X"},
		testCase{"${TEST}${TEST}", "XX"},
		testCase{"${TEST}/${TEST}", "X/X"},
		testCase{"${TEST}/Y", "X/Y"},
		testCase{"Y/${TEST}", "Y/X"},
	}
	os.Setenv("TEST", "X")
	for _, test := range testCases {
		flag := EnvFlag(test.input)
		if got, want := flag.Get().(string), test.want; got != want {
			t.Errorf("unexpected output: got %v, want %v", got, want)
		}
	}
}

func TestRuntimeFlagParser(t *testing.T) {
	type testCase struct {
		input, want string
	}
	testCases := []testCase{
		testCase{"${GOOS}", runtime.GOOS},
		testCase{"${GOARCH}${GOOS}", runtime.GOARCH + runtime.GOOS},
		testCase{"${GOOS}/${GOARCH}", runtime.GOOS + "/" + runtime.GOARCH},
		testCase{"${GOARCH}/Y", runtime.GOARCH + "/Y"},
		testCase{"Y/${GOOS}", "Y/" + runtime.GOOS},
	}
	for _, test := range testCases {
		flag := RuntimeFlag(test.input)
		if got, want := flag.Get().(string), test.want; got != want {
			t.Errorf("unexpected output: got %v, want %v", got, want)
		}
	}
}

func TestFlagSubstitution(t *testing.T) {
	// Run a test program that checks that an environment variable in
	// the default value of a command-line flag is substituted.
	{
		cmd := exec.Command("go", "run", filepath.Join("testdata", "flag.go"))
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v failed:\n%v", strings.Join(cmd.Args, " "), string(output))
		}
	}
	// Check that the substitution occurs in the default documentation.
	{
		cmd := exec.Command("go", "run", filepath.Join("testdata", "flag.go"), "-help")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v failed:\n%v", strings.Join(cmd.Args, " "), string(output))
		}
		if got, want := string(output), "-test=HELLO"; !strings.Contains(got, want) {
			t.Fatalf("%q not found in:\n%v", want, got)
		}
	}
	// Check that the substitution does not occur in the GoDoc
	// documentation.
	{
		cmd := exec.Command("go", "run", filepath.Join("testdata", "flag.go"), "-help")
		cmd.Env = append(os.Environ(), "CMDLINE_STYLE=godoc")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v failed:\n%v", strings.Join(cmd.Args, " "), string(output))
		}
		if got, want := string(output), "-test=${TEST}"; !strings.Contains(got, want) {
			t.Fatalf("%q not found in:\n%v", want, got)
		}
	}
}
