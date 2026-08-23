package phase

import (
	"context"
	"sync"
	"time"
)

type Name string

const (
	Config              Name = "config"
	DependencyProbes    Name = "dependency_probes"
	TargetFreeze        Name = "target_freeze"
	SecretScan          Name = "secret_scan"
	ProviderPreparation Name = "provider_preparation"
	ProviderProcess     Name = "provider_process"
	ProtocolDecode      Name = "protocol_decode"
	SourceRevalidation  Name = "source_revalidation"
	ReportWrite         Name = "report_write"
)

type Measurement struct {
	Name     Name
	Duration time.Duration
}

type Observer func(Measurement)

type Clock func() time.Time

type Recorder struct {
	now        Clock
	observe    Observer
	deliveryM  sync.Mutex
	delivering bool
	pending    []Measurement
}

func New(now Clock, observe Observer) *Recorder {
	if now == nil {
		now = time.Now
	}
	return &Recorder{now: now, observe: observe}
}

func (recorder *Recorder) Now() time.Time {
	if recorder == nil || recorder.now == nil {
		return time.Now()
	}
	return recorder.now()
}

func (recorder *Recorder) Start(name Name) *Span {
	if recorder == nil {
		return &Span{}
	}
	return &Span{recorder: recorder, name: name, started: recorder.Now()}
}

func (recorder *Recorder) record(name Name, started, ended time.Time) {
	if recorder.observe == nil {
		return
	}
	duration := ended.Sub(started)
	if ended.Before(started) {
		duration = 0
	}
	measurement := Measurement{Name: name, Duration: duration}
	recorder.deliveryM.Lock()
	recorder.pending = append(recorder.pending, measurement)
	if recorder.delivering {
		recorder.deliveryM.Unlock()
		return
	}
	recorder.delivering = true
	for len(recorder.pending) > 0 {
		measurement = recorder.pending[0]
		recorder.pending = recorder.pending[1:]
		recorder.deliveryM.Unlock()
		recorder.observe(measurement)
		recorder.deliveryM.Lock()
	}
	recorder.delivering = false
	recorder.deliveryM.Unlock()
}

type Span struct {
	recorder *Recorder
	name     Name
	started  time.Time
	once     sync.Once
}

func (span *Span) End() {
	if span == nil {
		return
	}
	span.once.Do(func() {
		if span.recorder != nil {
			span.recorder.record(span.name, span.started, span.recorder.Now())
		}
	})
}

type contextKey struct{}

func WithRecorder(ctx context.Context, recorder *Recorder) context.Context {
	if recorder == nil {
		return ctx
	}
	return context.WithValue(ctx, contextKey{}, recorder)
}

func FromContext(ctx context.Context) *Recorder {
	if ctx == nil {
		return nil
	}
	recorder, _ := ctx.Value(contextKey{}).(*Recorder)
	return recorder
}

func Start(ctx context.Context, name Name) *Span {
	return FromContext(ctx).Start(name)
}

func ElapsedMilliseconds(started, ended time.Time) int64 {
	if ended.Before(started) {
		return 0
	}
	return ended.Sub(started).Milliseconds()
}
