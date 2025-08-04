package logger

import "testing"

func TestLogger(t *testing.T) {
	logger := Default()
	logger.Info("test", "attr1", "test", "attr2", "test2", "attr3")
}
