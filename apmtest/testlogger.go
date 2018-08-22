package apmtest

import "testing"

type TestLogger struct {
	t *testing.T
}

func NewTestLogger(t *testing.T) TestLogger {
	return TestLogger{t: t}
}

func (t TestLogger) Debugf(format string, args ...interface{}) {
	t.t.Logf("[DEBUG] "+format, args...)
}

func (t TestLogger) Errorf(format string, args ...interface{}) {
	t.t.Logf("[ERROR] "+format, args...)
}
