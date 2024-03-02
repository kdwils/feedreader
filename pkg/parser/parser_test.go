package parser

import (
	"bytes"
	"net/http"
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewFeedParser(t *testing.T) {
	type args struct {
		http HTTP
	}
	tests := []struct {
		args args
		want Parser
		name string
	}{
		{
			name: "new parser",
			args: args{
				http: http.DefaultClient,
			},
			want: FeedParser{
				http: http.DefaultClient,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := New(tt.args.http); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("New() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFeedReader_Parse(t *testing.T) {
	t.Run("error parsing xml", func(t *testing.T) {
		parser := New(http.DefaultClient)
		feed, err := parser.Parse(bytes.NewReader([]byte(`<`)))
		if err == nil {
			t.Error("expected err: err is nil")
		}

		if feed != nil {
			t.Errorf("TestFeedReader_Parse() = %v, expected %v", feed, nil)
		}
	})

	t.Run("success", func(t *testing.T) {
		b, err := os.ReadFile("testing/feed.rss")
		if err != nil {
			t.Error(err)
		}

		parser := New(http.DefaultClient)
		feed, err := parser.Parse(bytes.NewReader(b))
		if err != nil {
			t.Error(err)
		}

		wantFeed := &RSSFeed{
			Channel: Channel{
				Title:         "blog.kyledev.co",
				Link:          "https://blog.kyledev.co/",
				Generator:     "Hugo -- gohugo.io",
				Description:   "Recent content on blog.kyledev.co",
				LastBuildDate: "Tue, 25 Apr 2023 00:00:00 +0000",
				Items: []Item{
					{
						Title:       "Deploying applications to my cluster using Github Actions and ArgoCD",
						Author:      "Kyle Wilson",
						Link:        "https://blog.kyledev.co/posts/ci-and-argocd/",
						PubDate:     "Tue, 25 Apr 2023 00:00:00 +0000",
						GUID:        "https://blog.kyledev.co/posts/ci-and-argocd/",
						Description: "Check out how I created a reusable github action for building, pushing, and signing docker images. ArgoCD then syncs changes to my homelab.",
					},
					{
						Title:       "Exposing services in my cluster using cloudflare tunnels",
						Author:      "Kyle Wilson",
						Link:        "https://blog.kyledev.co/posts/cloudflared-tunnel/",
						PubDate:     "Thu, 23 Feb 2023 00:00:00 +0000",
						GUID:        "https://blog.kyledev.co/posts/cloudflared-tunnel/",
						Description: "Exposing my blog using a cloudflare tunnel without needing to port forward or expose my local network.",
					},
				},
			},
		}

		if !assert.Equal(t, wantFeed, feed) {
			t.Errorf("TestFeedReader_Parse() = %+v, want %+v", feed, wantFeed)
		}
	})
}
