package parser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestFetchAndParseRSS 验证解析器在各种网络状态、畸形 XML 以及 Namespace 混用下的行为
func TestFetchAndParseRSS(t *testing.T) {
	// 1. 准备 Mock RSS XML 数据
	mockXML := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd">
  <channel>
    <title>慧客堂</title>
    <description>这是一档精美的中文播客栏目。</description>
    <itunes:author>慧客老师</itunes:author>
    <itunes:image href="https://example.com/cover.jpg" />
    <item>
      <title>Vol.88 跌了你不敢买</title>
      <pubDate>Fri, 22 May 2026 12:00:00 GMT</pubDate>
      <description>今天我们来聊聊牛市策略。</description>
      <enclosure url="https://example.com/audio/vol88.m4a" length="12345678" type="audio/x-m4a" />
    </item>
    <item>
      <title>Vol.87 纯文本占位符，不应该被解析为可下载单集</title>
      <pubDate>Thu, 21 May 2026 12:00:00 GMT</pubDate>
      <description>纯文本介绍</description>
    </item>
    <item>
      <title>Vol.86 另一期精彩内容</title>
      <pubDate>Wed, 20 May 2026 12:00:00 GMT</pubDate>
      <description>另外一些简介</description>
      <enclosure url="https://example.com/audio/vol86.mp3" length="9876543" type="audio/mpeg" />
    </item>
  </channel>
</rss>`

	// 2. 启动本地 Mock HTTP 服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, mockXML)
	}))
	defer server.Close()

	// 3. 运行测试
	t.Run("Valid RSS Feed", func(t *testing.T) {
		podcast, err := FetchAndParseRSS(context.Background(), server.URL)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if podcast.Title != "慧客堂" {
			t.Errorf("expected Title '慧客堂', got %q", podcast.Title)
		}
		if podcast.Author != "慧客老师" {
			t.Errorf("expected Author '慧客老师', got %q", podcast.Author)
		}
		if podcast.ImageURL != "https://example.com/cover.jpg" {
			t.Errorf("expected ImageURL 'https://example.com/cover.jpg', got %q", podcast.ImageURL)
		}

		// 验证无直链单集被成功过滤
		if len(podcast.Episodes) != 2 {
			t.Fatalf("expected 2 valid episodes, got %d", len(podcast.Episodes))
		}

		// 验证单集 1 元数据
		ep1 := podcast.Episodes[0]
		if ep1.Title != "Vol.88 跌了你不敢买" {
			t.Errorf("expected ep1 title 'Vol.88 跌了你不敢买', got %q", ep1.Title)
		}
		if ep1.AudioURL != "https://example.com/audio/vol88.m4a" {
			t.Errorf("expected ep1 audio url, got %q", ep1.AudioURL)
		}
		if ep1.Length != 12345678 {
			t.Errorf("expected ep1 length 12345678, got %d", ep1.Length)
		}

		// 验证单集 2 元数据
		ep2 := podcast.Episodes[1]
		if ep2.Title != "Vol.86 另一期精彩内容" {
			t.Errorf("expected ep2 title 'Vol.86 另一期精彩内容', got %q", ep2.Title)
		}
	})

	t.Run("Empty URL validation", func(t *testing.T) {
		_, err := FetchAndParseRSS(context.Background(), "")
		if err == nil {
			t.Errorf("expected error for empty URL, got nil")
		}
	})

	t.Run("Invalid HTTP Status", func(t *testing.T) {
		errServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer errServer.Close()

		_, err := FetchAndParseRSS(context.Background(), errServer.URL)
		if err == nil {
			t.Errorf("expected error for 404 status, got nil")
		}
	})

	t.Run("Malformed XML", func(t *testing.T) {
		malformedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "<rss><invalid></rss>")
		}))
		defer malformedServer.Close()

		_, err := FetchAndParseRSS(context.Background(), malformedServer.URL)
		if err == nil {
			t.Errorf("expected error for malformed XML, got nil")
		}
	})
}
