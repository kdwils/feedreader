package poller

import (
	"context"
	"time"

	"github.com/kdwils/feedreader/service"
	"go.uber.org/zap"
)

// Poller checks rss feeds for new articles on a given interval
type Poller struct {
	service service.Service
	ticker  *time.Ticker
	logger  *zap.Logger
}

func New(ticker *time.Ticker, service service.Service, logger *zap.Logger) Poller {
	return Poller{
		ticker:  ticker,
		service: service,
		logger:  logger,
	}
}

func (p Poller) Poll(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-p.ticker.C:
			feeds, err := p.service.ListFeeds(ctx, nil)
			if err != nil {
				return err
			}

			for _, f := range feeds {
				new, err := p.service.RefreshFeedArticles(ctx, f)
				if err != nil {
					p.logger.Error("failed to refreshed feed articles", zap.Error(err), zap.Any("feed", f.Title))
					continue
				}

				p.logger.Info("successfully refreshed feed", zap.String("feed", f.Title), zap.Int("articles added", len(new)))
			}
		}
	}
}
