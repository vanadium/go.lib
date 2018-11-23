package cmdline_test

import (
	"flag"
	"fmt"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"v.io/x/lib/cmdline"
)

func ExampleRegisterFlagsInStruct() {
	eg := struct {
		A int    `cmdline:"int-flag::-1,intVar flag"`
		B string `cmdline:"string-flag::'some,value,with,a,comma',stringVar flag"`
		O int
	}{
		O: 23,
	}
	flagSet := &flag.FlagSet{}
	err := cmdline.RegisterFlagsInStruct(flagSet, "cmdline", &eg, nil, nil)
	if err != nil {
		panic(err)
	}
	fmt.Println(eg.A)
	fmt.Println(eg.B)
	flagSet.Parse([]string{"-int-flag=42"})
	fmt.Println(eg.A)
	fmt.Println(eg.B)
	// Output:
	// -1
	// some,value,with,a,comma
	// 42
	// some,value,with,a,comma
}

type myFlagVar int64

func (mf *myFlagVar) Set(v string) error {
	i, err := strconv.ParseInt(v, 10, 64)
	*mf = myFlagVar(i)
	return err
}

func (mf *myFlagVar) String() string {
	return fmt.Sprintf("%v", *mf)
}

func TestReflectTags(t *testing.T) {
	for _, tc := range []struct {
		tag              string
		name, val, usage string
		err              string
	}{
		{"", "", "", "", "empty or missing tag"},
		{"::", "", "", "", "empty or missing flag name"},
		{"nn::,", "nn", "", "", ""},
		{"n::xx", "n", "", "xx", ""},
		{"n::,yy", "n", "", "yy", ""},
		{"n::'xx,yy'", "n", "xx,yy", "", ""},
		{"n::'xx,yy',", "n", "xx,yy", "", ""},
		{"n::'xx,yy',zz", "n", "xx,yy", "zz", ""},
		{"n::'xx,yy'zz", "n", "xx,yy", "zz", `has spurious text starting at pos 6, "zz"`},
		{"n::'xx,yy'zz,aa", "n", "xx,yy", "aa", `has spurious text starting at pos 6, "zz,aa"`},
		{"n::'xx,yy',aa,bb", "n", "xx,yy", "aa,bb", ""},
		{"n::xx,yy,zz", "n", "xx", "yy,zz", ""},
	} {
		n, v, d, err := cmdline.ParseFlagTag(tc.tag)
		if err != nil {
			if got, want := err.Error(), tc.err; got != want {
				t.Errorf("tag %v: got %v, want %v", tc.tag, got, want)
			}
			continue
		}
		if got, want := n, tc.name; got != want {
			t.Errorf("tag %q: got %q, want %q", tc.tag, got, want)
		}
		if got, want := v, tc.val; got != want {
			t.Errorf("tag %q: got %q, want %q", tc.tag, got, want)
		}
		if got, want := d, tc.usage; got != want {
			t.Errorf("tag %q: got %q, want %q", tc.tag, got, want)
		}
	}
}

type dummy struct{}

func TestReflect(t *testing.T) {
	assert := func(got, want interface{}) {
		_, file, line, _ := runtime.Caller(1)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%v:%v:got %v, want %v", filepath.Base(file), line, got, want)
		}
	}

	allFlags := func(fs *flag.FlagSet) string {
		out := []string{}
		fs.VisitAll(func(f *flag.Flag) {
			rest := ""
			if len(f.DefValue) == 0 {
				rest = "," + f.Usage
			} else {
				rest = f.DefValue + "," + f.Usage
				if strings.Contains(f.DefValue, ",") {
					rest = "'" + f.DefValue + "'," + f.Usage
				}
			}
			out = append(out, fmt.Sprintf(`cmdline:"%v::%v"`, f.Name, rest))
		})
		sort.Strings(out)
		return strings.Join(out, "\n")
	}

	// Test all 'empty' defaults.
	s0 := struct {
		A   int           `cmdline:"iv::,intVar flag"`
		AND int           `cmdline:"iv-nd::,intVar no default flag"`
		B   int64         `cmdline:"iv64::,int64var flag"`
		C   uint          `cmdline:"u::,uintVar flag"`
		D   uint64        `cmdline:"u64::,uint64Var flag"`
		E   float64       `cmdline:"f64::,float64Var flag"`
		F   bool          `cmdline:"doit::,boolVar flag"`
		G   time.Duration `cmdline:"wait::,durationVar flag"`
		HQ  string        `cmdline:"str::,stringVar flag"`
		HNQ string        `cmdline:"str-nq::,stringVar no default flag"`
		V   myFlagVar     `cmdline:"some-var::,user defined var flag"`
	}{}

	expectedUsage := []string{`cmdline:"iv::0,intVar flag"`,
		`cmdline:"iv-nd::0,intVar no default flag"`,
		`cmdline:"iv64::0,int64var flag"`,
		`cmdline:"u::0,uintVar flag"`,
		`cmdline:"u64::0,uint64Var flag"`,
		`cmdline:"f64::0,float64Var flag"`,
		`cmdline:"doit::false,boolVar flag"`,
		`cmdline:"wait::0,durationVar flag"`,
		`cmdline:"str::,stringVar flag"`,
		`cmdline:"str-nq::,stringVar no default flag"`,
		`cmdline:"some-var::,user defined var flag"`,
	}
	sort.Strings(expectedUsage)

	fs := &flag.FlagSet{}
	err := cmdline.RegisterFlagsInStruct(fs, "cmdline", &s0, nil, nil)
	if err != nil {
		t.Errorf("%v", err)
	}
	if got, want := allFlags(fs), strings.Join(expectedUsage, "\n"); got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	assert(s0.A, 0)
	assert(s0.AND, 0)
	assert(s0.B, int64(0))
	assert(s0.C, uint(0))
	assert(s0.D, uint64(0))
	assert(s0.E, float64(0))
	assert(s0.F, false)
	assert(s0.G, time.Duration(0))
	assert(s0.HQ, "")
	assert(s0.HNQ, "")
	assert(s0.V, myFlagVar(0))

	// Test with some explicit literal defaults, some value and usage
	// defaults also.
	s1 := struct {
		A   int           `cmdline:"iv::-1,intVar flag"`
		AND int           `cmdline:"iv-nd::,intVar no default flag"`
		B   int64         `cmdline:"iv64::-2,int64var flag"`
		C   uint          `cmdline:"u::3,uintVar flag"`
		D   uint64        `cmdline:"u64::3,uint64Var flag"`
		E   float64       `cmdline:"f64::2.03,float64Var flag"`
		F   bool          `cmdline:"doit::true,boolVar flag"`
		G   time.Duration `cmdline:"wait::2s,durationVar flag"`
		HQ  string        `cmdline:"str::'xx,yy',stringVar flag"`
		HNQ string        `cmdline:"str-nq::xxyy,stringVar no default flag"`
		V   myFlagVar     `cmdline:"some-var::22,user defined var flag"`
		X   myFlagVar     `cmdline:"env-var::33,user defined var flag"`
		ZZ  string        // ignored
		zz  string        // ignored
	}{}

	values := map[string]interface{}{
		"iv": 33,
		"u":  runtime.NumCPU(),
	}

	usageDefaults := map[string]string{
		"u":       "<num-cores>",
		"env-var": "$ENVVAR",
	}

	expectedUsage = []string{`cmdline:"iv::-1,intVar flag"`,
		`cmdline:"iv-nd::0,intVar no default flag"`,
		`cmdline:"iv64::-2,int64var flag"`,
		`cmdline:"u::<num-cores>,uintVar flag"`,
		`cmdline:"u64::3,uint64Var flag"`,
		`cmdline:"f64::2.03,float64Var flag"`,
		`cmdline:"doit::true,boolVar flag"`,
		`cmdline:"wait::2s,durationVar flag"`,
		`cmdline:"str::'xx,yy',stringVar flag"`,
		`cmdline:"str-nq::xxyy,stringVar no default flag"`,
		`cmdline:"some-var::22,user defined var flag"`,
		`cmdline:"env-var::$ENVVAR,user defined var flag"`,
	}
	sort.Strings(expectedUsage)

	fs = &flag.FlagSet{}
	err = cmdline.RegisterFlagsInStruct(fs, "cmdline", &s1, nil, usageDefaults)
	if err != nil {
		t.Errorf("%v", err)
	}
	if got, want := allFlags(fs), strings.Join(expectedUsage, "\n"); got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	assert(s1.A, -1)
	assert(s1.AND, 0)
	assert(s1.B, int64(-2))
	assert(s1.C, uint(3))
	assert(s1.D, uint64(3))
	assert(s1.E, 2.03)
	assert(s1.F, true)
	assert(s1.G, 2*time.Second)
	assert(s1.HQ, "xx,yy")
	assert(s1.HNQ, "xxyy")
	assert(s1.V, myFlagVar(22))
	assert(s1.X, myFlagVar(33))

	fs = &flag.FlagSet{}
	err = cmdline.RegisterFlagsInStruct(fs, "cmdline", &s1, values, usageDefaults)
	if err != nil {
		t.Errorf("%v", err)
	}

	assert(s1.A, 33)
	assert(s1.AND, 0)
	assert(s1.B, int64(-2))
	assert(s1.C, uint(runtime.NumCPU()))
	assert(s1.D, uint64(3))
	assert(s1.E, 2.03)
	assert(s1.F, true)
	assert(s1.G, 2*time.Second)
	assert(s1.HQ, "xx,yy")
	assert(s1.HNQ, "xxyy")
	assert(s1.V, myFlagVar(22))
	assert(s1.X, myFlagVar(33))

	if err := fs.Parse([]string{
		"-iv=42",
		"-iv-nd=42",
		"-iv64=42",
		"-u=42",
		"-u64=42",
		"-f64=42.42",
		"-doit=false",
		"--wait=42h",
		"--str=42",
		"--str-nq=42",
		"--some-var=42",
		"--env-var=12",
	}); err != nil {
		t.Errorf("%v", err)
	}
	assert(s1.A, 42)
	assert(s1.AND, 42)
	assert(s1.B, int64(42))
	assert(s1.C, uint(42))
	assert(s1.D, uint64(42))
	assert(s1.E, 42.42)
	assert(s1.F, false)
	assert(s1.G, 42*time.Hour)
	assert(s1.HQ, "42")
	assert(s1.HNQ, "42")
	assert(s1.V, myFlagVar(42))
	assert(s1.X, myFlagVar(12))

}

func TestReflectErrors(t *testing.T) {

	expected := func(err error, msg string) {
		_, file, line, _ := runtime.Caller(1)
		if err == nil {
			t.Errorf("%v:%v: expected an error", filepath.Base(file), line)
			return
		}
		if got, want := err.Error(), msg; got != want {
			t.Errorf("%v:%v:got %v, want %v", filepath.Base(file), line, got, want)
		}
	}

	fs := &flag.FlagSet{}
	err := cmdline.RegisterFlagsInStruct(fs, "cmdline", 23, nil, nil)
	expected(err, "int is not addressable")
	dummy := 0
	err = cmdline.RegisterFlagsInStruct(fs, "cmdline", &dummy, nil, nil)
	expected(err, "*int is not a pointer to a struct")
	t1 := struct {
		A int `cmdline:"xxx"`
	}{}
	err = cmdline.RegisterFlagsInStruct(fs, "cmdline", &t1, nil, nil)
	expected(err, "field A: failed to parse tag: xxx")

	t2 := struct {
		A interface{} `cmdline:"xx::,usage"`
	}{}
	err = cmdline.RegisterFlagsInStruct(fs, "cmdline", &t2, nil, nil)
	expected(err, "field: A of type interface {} for flag xx: does not implement flag.Value")

	t3 := struct {
		A myFlagVar `cmdline:"zzz::bad-number,usage"`
	}{}
	err = cmdline.RegisterFlagsInStruct(fs, "cmdline", &t3, nil, nil)
	expected(err, `field: A of type cmdline_test.myFlagVar for flag zzz: failed to set initial default value for flag.Value: strconv.ParseInt: parsing "bad-number": invalid syntax`)

	t4 := struct {
		A int `cmdline:"zzz::bad-number,usage"`
	}{}
	err = cmdline.RegisterFlagsInStruct(fs, "cmdline", &t4, nil, nil)
	expected(err, `field: A of type int for flag zzz: failed to set initial default value: strconv.ParseInt: parsing "bad-number": invalid syntax`)

	t5 := struct {
		A int `cmdline:"xxx::,zz"`
	}{}
	err = cmdline.RegisterFlagsInStruct(fs, "cmdline", &t5, nil, map[string]string{"xx": "yy"})
	fs = &flag.FlagSet{}
	expected(err, "flag xx does not exist but specified as a usage default")
	err = cmdline.RegisterFlagsInStruct(fs, "cmdline", &t5, map[string]interface{}{"xx": "yy"}, nil)
	expected(err, "flag xx does not exist but specified as a value default")
}

type runner struct{}

func (r *runner) Run(env *cmdline.Env, args []string) error {
	return nil
}
func TestReflectIntegration(t *testing.T) {
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
