package cmdline_test

import (
	"testing"

	"v.io/x/lib/cmdline"
)

type runner struct{}

func (r *runner) Run(env *cmdline.Env, args []string) error {
	return nil
}
func TestFlagVarIntegration(t *testing.T) {
	s1 := struct {
		A int `cmdline:"int-var::32,some-arg"`
	}{}
	cmd := &cmdline.Command{
		Name:     "test",
		FlagDefs: cmdline.FlagDefinitions{StructWithFlags: &s1},
		Runner:   &runner{},
	}
	_, _, err := cmdline.Parse(cmd, cmdline.EnvFromOS(), []string{"--int-var=33"})
	if err != nil {
		t.Errorf("%v", err)
	}
	if got, want := s1.A, 33; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	cmd = &cmdline.Command{
		Name:     "test",
		FlagDefs: cmdline.FlagDefinitions{StructWithFlags: &s1},
	}
	cmd.Children = append(cmd.Children, &cmdline.Command{
		Name:     "child1",
		FlagDefs: cmdline.FlagDefinitions{StructWithFlags: &s1},
		Runner:   &runner{},
	})

	_, _, err = cmdline.Parse(cmd, cmdline.EnvFromOS(), []string{
		"child1",
		"--int-var=44",
	})
	if err != nil {
		t.Errorf("%v", err)
	}
	if got, want := s1.A, 44; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

}
