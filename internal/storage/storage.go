package storage

import (
	"github.com/jules/http-monitor/internal/model"
)

// Storage defines the interface for persisting measurements.
type Storage interface {
	SaveMeasurement(m model.Measurement) error
	GetMeasurements(targetName string, limit int) ([]model.Measurement, error)
	Close() error
}
