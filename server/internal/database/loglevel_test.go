package database

import (
	"testing"

	"github.com/rs/zerolog"
	"gorm.io/gorm/logger"
)

func TestGormLogLevelFromZerolog(t *testing.T) {
	tests := []struct {
		in   zerolog.Level
		want logger.LogLevel
	}{
		{zerolog.TraceLevel, logger.Info},
		{zerolog.DebugLevel, logger.Info},
		{zerolog.InfoLevel, logger.Warn},
		{zerolog.WarnLevel, logger.Warn},
		{zerolog.ErrorLevel, logger.Error},
		{zerolog.Disabled, logger.Error},
	}
	for _, tt := range tests {
		got := gormLogLevelFromZerolog(tt.in)
		if got != tt.want {
			t.Errorf("gormLogLevelFromZerolog(%s) = %v, want %v", tt.in, got, tt.want)
		}
	}
}
