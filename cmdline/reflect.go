package cmdline

import (
	"flag"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unsafe"
)

var (
	flagValueType = reflect.TypeOf((*flag.Value)(nil)).Elem()
)

// ParseFlagTag parses the supplied string into a flag name, default literal
// value and description components. It is used by
// CreatenAndRegisterFlagsInStruct to parse the field tags.
//
// The tag format is:
//
// <name>::<literal-default-value>,<usage>
//
// where <name> is the name of the flag, <default-value> is an optional
// literal default value for the flag and <usage> the detailed
// description for the flag. <name> must be supplied.
// <default-value> may be placed in single quotes (') if the default
// value needs to contain a comma. E.g 'some default, with comma',description.
// The <usage> field may contain commas since the parsing stops after
// the first comma is encountered.
func ParseFlagTag(t string) (name, value, usage string, err error) {
	if len(t) == 0 {
		err = fmt.Errorf("empty or missing tag")
		return
	}
	// Parse the flag name component.
	idx := strings.Index(t, "::")
	if idx <= 0 {
		err = fmt.Errorf("empty or missing flag name")
		return
	}
	name = t[:idx]
	t = t[idx+2:]
	// Parse the optionally quoted literal default value.
	if t[0] == '\'' {
		for i, r := range t[1:] {
			if r == '\'' {
				value = t[1 : i+1]
				t = t[i+2:]
				if len(t) > 0 && t[0] != ',' {
					err = fmt.Errorf("has spurious text starting at pos %v, %q", i+1, t)
					return
				}
				break
			}
		}
	}
	// Parse the usage.
	for i, r := range t {
		if r == ',' {
			if len(value) == 0 {
				if i > 0 {
					value = t[:i]
				}
			}
			usage = t[i+1:]
			return
		}
	}
	usage = t
	return
}

func literalDefault(typeName, literal string, initialValue interface{}) (value interface{}, err error) {
	if initialValue != nil {
		switch v := initialValue.(type) {
		case int, int64, uint, uint64, bool, float64, time.Duration:
			value = v
			return
		}
	}
	if len(literal) == 0 {
		switch typeName {
		case "int":
			value = int(0)
		case "int64", "time.Duration":
			value = int64(0)
		case "uint":
			value = uint(0)
		case "uint64":
			value = uint64(0)
		case "bool":
			value = bool(false)
		case "float64":
			value = float64(0)
		case "string":
			value = ""
		}
		return
	}
	var tmp int64
	var utmp uint64
	switch typeName {
	case "int":
		tmp, err = strconv.ParseInt(literal, 10, 64)
		value = int(tmp)
	case "int64":
		tmp, err = strconv.ParseInt(literal, 10, 64)
		value = tmp
	case "uint":
		utmp, err = strconv.ParseUint(literal, 10, 64)
		value = uint(utmp)
	case "uint64":
		utmp, err = strconv.ParseUint(literal, 10, 64)
		value = utmp
	case "bool":
		value, err = strconv.ParseBool(literal)
	case "float64":
		value, err = strconv.ParseFloat(literal, 64)
	case "time.Duration":
		value, err = time.ParseDuration(literal)
	case "string":
		value = literal
	}
	return
}

// RegisterFlagsInStruct will selectively register fields in the supplied struct
// as flags of the appropriate type with the supplied flag.FlagSet. Fields
// are selected if they have tag of the form `cmdline:"name::<literal>,<usage>"`
// associated with them, as defined by ParseFlagTag above.
// In addition to literal default values specified in the tag it is possible
// to provide computed default values via the valuesDefaults, and also
// defaults that will appear in the usage string for help messages that
// override the actual default value. The latter is useful for flags that
// have a default that is system dependent that is not informative in the usage
// statement. For example --home-dir which should default to /home/user but the
// usage message would more usefully say --home-dir=$HOME.
// Both maps are keyed by the name of the flag, not the field.
func RegisterFlagsInStruct(fs *flag.FlagSet, tag string, structWithFlags interface{}, valueDefaults map[string]interface{}, usageDefaults map[string]string) error {
	typ := reflect.TypeOf(structWithFlags)
	val := reflect.ValueOf(structWithFlags)
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
		val = reflect.Indirect(val)
	}
	if !val.CanAddr() {
		return fmt.Errorf("%T is not addressable", structWithFlags)
	}

	if typ.Kind() != reflect.Struct {
		return fmt.Errorf("%T is not a pointer to a struct", structWithFlags)
	}

	for i := 0; i < typ.NumField(); i++ {
		fieldType := typ.Field(i)
		tags, ok := fieldType.Tag.Lookup(tag)
		if !ok {
			continue
		}
		name, value, description, err := ParseFlagTag(tags)
		if err != nil {
			return fmt.Errorf("field %v: failed to parse tag: %v", fieldType.Name, tags)
		}
		fieldValue := val.Field(i)
		fieldName := fieldType.Name
		fieldTypeName := fieldType.Type.String()

		errPrefix := func() string {
			return fmt.Sprintf("field: %v of type %v for flag %v", fieldName, fieldTypeName, name)
		}

		initialValue, err := literalDefault(fieldTypeName, value, valueDefaults[name])
		if err != nil {
			return fmt.Errorf("%v: failed to set initial default value: %v", errPrefix(), err)
		}

		if initialValue == nil {
			addr := fieldValue.Addr()
			if !addr.Type().Implements(flagValueType) {
				return fmt.Errorf("%v: does not implement flag.Value", errPrefix())
			}
			dv := addr.Interface().(flag.Value)
			fs.Var(dv, name, description)
			if len(value) > 0 {
				if err := dv.Set(value); err != nil {
					return fmt.Errorf("%v: failed to set initial default value for flag.Value: %v", errPrefix(), err)
				}
			}
			if ud, ok := usageDefaults[name]; ok {
				fs.Lookup(name).DefValue = ud
			} else {
				fs.Lookup(name).DefValue = value
			}
			continue
		}

		switch dv := initialValue.(type) {
		case int:
			ptr := (*int)(unsafe.Pointer(fieldValue.Addr().Pointer()))
			fs.IntVar(ptr, name, dv, description)
		case int64:
			ptr := (*int64)(unsafe.Pointer(fieldValue.Addr().Pointer()))
			fs.Int64Var(ptr, name, dv, description)
		case uint:
			ptr := (*uint)(unsafe.Pointer(fieldValue.Addr().Pointer()))
			fs.UintVar(ptr, name, dv, description)
		case uint64:
			ptr := (*uint64)(unsafe.Pointer(fieldValue.Addr().Pointer()))
			fs.Uint64Var(ptr, name, dv, description)
		case bool:
			ptr := (*bool)(unsafe.Pointer(fieldValue.Addr().Pointer()))
			fs.BoolVar(ptr, name, dv, description)
		case float64:
			ptr := (*float64)(unsafe.Pointer(fieldValue.Addr().Pointer()))
			fs.Float64Var(ptr, name, dv, description)
		case string:
			ptr := (*string)(unsafe.Pointer(fieldValue.Addr().Pointer()))
			fs.StringVar(ptr, name, dv, description)
		case time.Duration:
			ptr := (*time.Duration)(unsafe.Pointer(fieldValue.Addr().Pointer()))
			fs.DurationVar(ptr, name, dv, description)
		default:
			// should never reach here.
			panic(fmt.Sprintf("%v flag: field %v, flag %v: unsupported type %T", fieldTypeName, fieldName, name, initialValue))
		}
	}

	for k := range valueDefaults {
		if fs.Lookup(k) == nil {
			return fmt.Errorf("flag %v does not exist but specified as a value default", k)
		}
	}

	for k, v := range usageDefaults {
		if fs.Lookup(k) == nil {
			return fmt.Errorf("flag %v does not exist but specified as a usage default", k)
		}
		fs.Lookup(k).DefValue = v
	}

	return nil
}
