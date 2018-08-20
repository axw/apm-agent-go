package transport

var (
	// Default is the default StreamSender, using the
	// ELASTIC_APM_* environment variables.
	//
	// If ELASTIC_APM_SERVER_URL is set to an invalid
	// location, Default will be set to a StreamSender
	// returning an error for every operation.
	Default StreamSender

	// Discard is a StreamSender on which all operations
	// succeed without doing anything.
	Discard = discardStreamSender{}
)

func init() {
	_, _ = InitDefault()
}

// InitDefault (re-)initializes Default, the default StreamSender, returning
// its new value along with the error that will be returned by the StreamSender
// if the environment variable configuration is invalid. The result is always
// non-nil.
func InitDefault() (StreamSender, error) {
	t, err := getDefault()
	//if apmdebug.TraceTransport {
	//t = &debugTransport{transport: t}
	//}
	Default = t
	return t, err
}

func getDefault() (StreamSender, error) {
	s, err := NewHTTPTransport("", "")
	if err != nil {
		return discardStreamSender{err}, err
	}
	return s, nil
}
