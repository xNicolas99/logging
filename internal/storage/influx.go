package storage

import (
	"context"
	"fmt"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/jules/http-monitor/internal/model"
)

// InfluxStorage implements Storage using InfluxDB 2.0.
type InfluxStorage struct {
	client   influxdb2.Client
	writeAPI api.WriteAPIBlocking
	queryAPI api.QueryAPI
	org      string
	bucket   string
}

// NewInfluxStorage initializes the InfluxDB storage.
func NewInfluxStorage(url, token, org, bucket string) (*InfluxStorage, error) {
	client := influxdb2.NewClient(url, token)
	// Check connection
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	ok, err := client.Ping(ctx)
	if err != nil {
		return nil, fmt.Errorf("ping failed: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("influxdb connection failed")
	}

	writeAPI := client.WriteAPIBlocking(org, bucket)
	queryAPI := client.QueryAPI(org)

	return &InfluxStorage{
		client:   client,
		writeAPI: writeAPI,
		queryAPI: queryAPI,
		org:      org,
		bucket:   bucket,
	}, nil
}

func (s *InfluxStorage) SaveMeasurement(m model.Measurement) error {
	p := influxdb2.NewPointWithMeasurement("http_download").
		AddTag("target", m.Target).
		AddField("duration", m.Duration).
		AddField("size", m.Size).
		AddField("speed", m.Speed).
		AddField("status", m.Status).
		SetTime(m.Timestamp)

	return s.writeAPI.WritePoint(context.Background(), p)
}

func (s *InfluxStorage) GetMeasurements(targetName string, limit int) ([]model.Measurement, error) {
	query := fmt.Sprintf(`from(bucket: "%s")
	|> range(start: -24h)
	|> filter(fn: (r) => r["_measurement"] == "http_download")
	|> filter(fn: (r) => r["target"] == "%s")
	|> pivot(rowKey:["_time"], columnKey: ["_field"], valueColumn: "_value")
	|> sort(columns: ["_time"], desc: true)
	|> limit(n: %d)`, s.bucket, targetName, limit)

	result, err := s.queryAPI.Query(context.Background(), query)
	if err != nil {
		return nil, err
	}

	var measurements []model.Measurement
	for result.Next() {
		r := result.Record()
		m := model.Measurement{
			Timestamp: r.Time(),
			Target:    targetName,
		}

		if v, ok := r.ValueByKey("duration").(float64); ok {
			m.Duration = v
		}
		if v, ok := r.ValueByKey("size").(int64); ok {
			m.Size = v
		}
		if v, ok := r.ValueByKey("speed").(float64); ok {
			m.Speed = v
		}
		if v, ok := r.ValueByKey("status").(string); ok {
			m.Status = v
		}
		measurements = append(measurements, m)
	}
	if result.Err() != nil {
		return nil, result.Err()
	}

	return measurements, nil
}

func (s *InfluxStorage) Close() error {
	s.client.Close()
	return nil
}
