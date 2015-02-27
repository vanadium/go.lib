package vlog

import (
	"fmt"
	"path"
	"reflect"
	"runtime"
	"sync/atomic"

	"github.com/cosmosnicolaou/llog"
)

// logCallLogLevel is the log level beyond which calls are logged.
const logCallLogLevel = 1

func callerFuncName() string {
	var funcName string
	pc, _, _, ok := runtime.Caller(stackSkip + 1)
	if ok {
		function := runtime.FuncForPC(pc)
		if function != nil {
			funcName = path.Base(function.Name())
		}
	}
	return funcName
}

// LogCall logs that its caller has been called given the arguments
// passed to it.  It returns a function that is supposed to be called
// when the caller returns, logging the caller’s return along with the
// arguments it is provided with.
// File name and line number of the call site and a randomly generated
// invocation identifier is logged automatically.  The path through which
// the caller function returns will be logged automatically too.
//
// The canonical way to use LogCall is along the lines of the following:
//
//     func Function(a Type1, b Type2) ReturnType {
//         defer vlog.LogCall(a, b)()
//         // ... function body ...
//         return retVal
//     }
//
// To log the return value as the function returns, the following
// pattern should be used.  Note that pointers to the output
// variables should be passed to the returning function, not the
// variables themselves:
//
//     func Function(a Type1, b Type2) (r ReturnType) {
//         defer vlog.LogCall(a, b)(&r)
//         // ... function body ...
//         return computeReturnValue()
//     }
//
// Note that when using this pattern, you do not need to actually
// assign anything to the named return variable explicitly.  A regular
// return statement would automatically do the proper return variable
// assignments.
//
// The log injector tool will automatically insert a LogCall invocation
// into all implementations of the public API it runs, unless a Valid
// Log Construct is found.  A Valid Log Construct is defined as one of
// the following at the beginning of the function body (i.e. should not
// be preceded by any non-whitespace or non-comment tokens):
//     1. defer vlog.LogCall(optional arguments)(optional pointers to return values)
//     2. defer vlog.LogCallf(argsFormat, optional arguments)(returnValuesFormat, optional pointers to return values)
//     3. // nologcall
//
// The comment "// nologcall" serves as a hint to log injection and
// checking tools to exclude the function from their consideration.
// It is used as follows:
//
//     func FunctionWithoutLogging(args ...interface{}) {
//         // nologcall
//         // ... function body ...
//     }
//
func LogCall(v ...interface{}) func(...interface{}) {
	if !V(logCallLogLevel) {
		return func(...interface{}) {}
	}
	callerFuncName := callerFuncName()
	invocationId := newInvocationIdentifier()
	if len(v) > 0 {
		Log.log.Printf(llog.InfoLog, "call[%s %s]: args:%v", callerFuncName, invocationId, v)
	} else {
		Log.log.Printf(llog.InfoLog, "call[%s %s]", callerFuncName, invocationId)
	}
	return func(v ...interface{}) {
		if len(v) > 0 {
			Log.log.Printf(llog.InfoLog, "return[%s %s]: %v", callerFuncName, invocationId, derefSlice(v))
		} else {
			Log.log.Printf(llog.InfoLog, "return[%s %s]", callerFuncName, invocationId)
		}
	}
}

// LogCallf behaves identically to LogCall, except it lets the caller to
// customize the log messages via format specifiers, like the following:
//
//     func Function(a Type1, b Type2) (r, t ReturnType) {
//         defer vlog.LogCallf("a: %v, b: %v", a, b)("(r,t)=(%v,%v)", &r, &t)
//         // ... function body ...
//         return finalR, finalT
//     }
//
func LogCallf(format string, v ...interface{}) func(string, ...interface{}) {
	if !V(logCallLogLevel) {
		return func(string, ...interface{}) {}
	}
	callerFuncName := callerFuncName()
	invocationId := newInvocationIdentifier()
	Log.log.Printf(llog.InfoLog, "call[%s %s]: %s", callerFuncName, invocationId, fmt.Sprintf(format, v...))
	return func(format string, v ...interface{}) {
		Log.log.Printf(llog.InfoLog, "return[%s %s]: %v", callerFuncName, invocationId, fmt.Sprintf(format, derefSlice(v)...))
	}
}

func derefSlice(slice []interface{}) []interface{} {
	o := make([]interface{}, 0, len(slice))
	for _, x := range slice {
		o = append(o, reflect.Indirect(reflect.ValueOf(x)).Interface())
	}
	return o
}

var invocationCounter uint64 = 0

// newInvocationIdentifier generates a unique identifier for a method invocation
// to make it easier to match up log lines for the entry and exit of a function
// when looking at a log transcript.
func newInvocationIdentifier() string {
	const (
		charSet    = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcdefghijklmnopqrstuvwxyz"
		charSetLen = uint64(len(charSet))
	)
	r := []byte{'@'}
	for n := atomic.AddUint64(&invocationCounter, 1); n > 0; n /= charSetLen {
		r = append(r, charSet[n%charSetLen])
	}
	return string(r)
}
