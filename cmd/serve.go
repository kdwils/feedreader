package cmd

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/kdwils/feedreader/config"
	"github.com/kdwils/feedreader/pkg/parser"
	"github.com/kdwils/feedreader/poller"
	"github.com/kdwils/feedreader/server"
	"github.com/kdwils/feedreader/service"
	"github.com/kdwils/feedreader/storage"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "serve the feedreader api",
	Long:  `serve the feedreader api`,
	Run: func(cmd *cobra.Command, args []string) {
		logger, err := server.NewLogger()
		if err != nil {
			log.Fatal(err)
		}

		c, err := config.Init(cfgFile)
		if err != nil {
			logger.Fatal("unable to load configs", zap.Error(err))
		}

		store := storage.NewSQLiteStorage(c.SQLite.FilePath)
		err = store.Connect()
		if err != nil {
			logger.Fatal("failed to connect to storage", zap.Error(err))
		}
		defer store.Close()

		interval := c.Poller.Interval
		if interval == 0 {
			interval = time.Hour * 1
		}

		parser := parser.New(http.DefaultClient)
		service := service.New(store, parser)

		if c.Poller.Enabled {
			ticker := time.NewTicker(interval)
			poller := poller.New(ticker, service, logger)
			go poller.Poll(context.TODO())
		}

		s := server.New(service, logger)
		s.Serve(c.Port)
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().StringVarP(&cfgFile, "config", "c", "config.yaml", "config file path")
}
