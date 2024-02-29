package cmd

import (
	"log"
	"net/http"

	"github.com/kdwils/feedreader/config"
	"github.com/kdwils/feedreader/pkg/parser"
	"github.com/kdwils/feedreader/server"
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

		parser := parser.New(http.DefaultClient)

		s := server.New(store, parser, logger)
		s.Serve(c.Port)
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().StringVarP(&cfgFile, "config", "c", "config.yaml", "config file path")
}
