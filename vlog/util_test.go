package vlog

// SetLog allows us to override the Log global for testing purposes.
func SetLog(l Logger) {
	Log = l.(*logger)
}
