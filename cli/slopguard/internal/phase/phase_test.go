package phase

import (
	"context"
	"testing"
	"time"
)

func TestRecorderUsesInjectedClockAndEndsOnce(t *testing.T) {
	t.Parallel()

	times := []time.Time{time.Unix(0, 0), time.Unix(0, int64(25*time.Millisecond))}
	index := 0
	var measurements []Measurement
	recorder := New(func() time.Time {
		value := times[index]
		index++
		return value
	}, func(measurement Measurement) {
		measurements = append(measurements, measurement)
	})
	span := recorder.Start(Config)
	span.End()
	span.End()
	if index != 2 || len(measurements) != 1 || measurements[0].Name != Config || measurements[0].Duration != 25*time.Millisecond {
		t.Fatalf("index=%d measurements=%+v", index, measurements)
	}
}

func TestContextRecorderClampsClockRegression(t *testing.T) {
	t.Parallel()

	times := []time.Time{time.Unix(1, 0), time.Unix(0, 0)}
	index := 0
	var measurement Measurement
	recorder := New(func() time.Time {
		value := times[index]
		index++
		return value
	}, func(value Measurement) {
		measurement = value
	})
	ctx := WithRecorder(context.Background(), recorder)
	span := Start(ctx, ReportWrite)
	span.End()
	if measurement.Name != ReportWrite || measurement.Duration != 0 {
		t.Fatalf("measurement = %+v", measurement)
	}
}

func TestObserverMayEndAnotherSpan(t *testing.T) {
	t.Parallel()

	current := time.Unix(0, 0)
	var nested *Span
	var measurements []Measurement
	recorder := New(func() time.Time {
		value := current
		current = current.Add(10 * time.Millisecond)
		return value
	}, func(measurement Measurement) {
		measurements = append(measurements, measurement)
		if measurement.Name == Config {
			nested.End()
		}
	})
	outer := recorder.Start(Config)
	nested = recorder.Start(ReportWrite)
	done := make(chan struct{})
	go func() {
		outer.End()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("re-entrant observer deadlocked")
	}
	if len(measurements) != 2 || measurements[0].Name != Config || measurements[1].Name != ReportWrite {
		t.Fatalf("measurements = %+v", measurements)
	}
}
