package pflagvar

import (
	"flag"

	"github.com/spf13/pflag"
	"v.io/x/lib/cmd/flagvar"
)

// RegisterFlagsInStruct is the same flagvar.RegisterFlagsInStruct except
// that it operates on pflag.FlagSet.
func RegisterFlagsInStruct(pfs *pflag.FlagSet, tag string, structWithFlags interface{}, valueDefaults map[string]interface{}, usageDefaults map[string]string) error {
	ps := flag.NewFlagSet("", flag.ExitOnError)
	if err := flagvar.RegisterFlagsInStruct(ps, tag, structWithFlags, valueDefaults, usageDefaults); err != nil {
		return err
	}
	pfs.AddGoFlagSet(ps)
	return nil
}
