package storage

import (
	"github.com/jules/http-monitor/internal/model"
)

// Storage defines the interface for persisting measurements.
type Storage interface {
	SaveMeasurement(m model.Measurement) error
	GetMeasurements(targetName string, limit int) ([]model.Measurement, error)
	GetMeasurementsWithRange(targetName string, rangeStart string, limit int, aggregate bool, window string) ([]model.Measurement, error)
	Close() error
}
