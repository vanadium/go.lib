package pflagvar_test

import (
	"fmt"

	"github.com/spf13/pflag"
	"v.io/x/lib/cmd/pflagvar"
)

func ExampleRegisterFlagsInStruct() {
	eg := struct {
		A int    `flag:"int-flag,-1,intVar flag"`
		B string `flag:"string-flag,'some,value,with,a,comma',stringVar flag"`
		O int
	}{
		O: 23,
	}
	flagSet := &pflag.FlagSet{}
	err := pflagvar.RegisterFlagsInStruct(flagSet, "flag", &eg, nil, nil)
	if err != nil {
		panic(err)
	}
	fmt.Println(eg.A)
	fmt.Println(eg.B)
	flagSet.Parse([]string{"--int-flag=42"})
	fmt.Println(eg.A)
	fmt.Println(eg.B)
	// Output:
	// -1
	// some,value,with,a,comma
	// 42
	// some,value,with,a,comma
}
