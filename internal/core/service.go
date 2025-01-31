package core

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

type Service struct {
	quitChannel chan struct{}
	logger      *slog.Logger
	port        int
	host        string
	Available   bool
	Hostname    string
}

func CreateService(logger *slog.Logger, host string, port int) *Service {
	return &Service{
		logger:    logger,
		port:      port,
		host:      host,
		Available: false,
		Hostname:  fmt.Sprintf("%s:%d", host, port),
	}
}

func (s *Service) Start(ctx context.Context, cfg *HealthCheckConfig) {
	s.quitChannel = make(chan struct{})
	tickerChannel := time.NewTicker(cfg.IntervalInMs * time.Millisecond)
	healthChannel := make(chan bool)

	go func() {
		s.logger.Log(ctx, slog.LevelInfo, "starting healthCheck")
		for {
			select {
			case <-tickerChannel.C:
				s.logger.Log(ctx, slog.LevelDebug, "calling healthCheck endpoint")
				resp, err := http.DefaultClient.Get(fmt.Sprintf("http://%s%s", s.Hostname, cfg.Path))
				if err != nil || resp.StatusCode >= http.StatusBadRequest {
					s.logger.Log(ctx, slog.LevelWarn, "application service is down")
					healthChannel <- false
				} else {
					if !s.Available {
						s.logger.Log(ctx, slog.LevelInfo, "application service is up")
					} else {
						s.logger.Log(ctx, slog.LevelDebug, "application service is still up")
					}
					healthChannel <- true
				}
			case <-s.quitChannel:
				s.logger.Log(ctx, slog.LevelInfo, "stopping healthCheck")
				tickerChannel.Stop()
				close(healthChannel)
				return
			}
		}
	}()

	select {
	case available := <-healthChannel:
		s.Available = available
	}
}

func (s *Service) Stop() {
	s.Available = false
	close(s.quitChannel)
}