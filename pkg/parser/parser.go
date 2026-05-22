// Package parser 提供 RSS XML 的动态拉取、解析与元数据提取功能。
package parser

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Podcast 承载了播客频道的元数据及单集列表。
type Podcast struct {
	Title       string
	Author      string
	Description string
	ImageURL    string
	Episodes    []Episode
}

// Episode 承载了单集的元数据及音频直链。
type Episode struct {
	Title       string
	PubDate     string
	AudioURL    string
	Length      int64
	Description string
}

// XMLRSS 是 RSS 2.0 规范的 XML 根解析结构体。
type XMLRSS struct {
	XMLName xml.Name   `xml:"rss"`
	Channel XMLChannel `xml:"channel"`
}

// XMLChannel 对应 <channel> 标签。
type XMLChannel struct {
	Title       string    `xml:"title"`
	Description string    `xml:"description"`
	Authors     []string  `xml:"author"` // 免疫 Namespace 设计：一并收集 <author> 和 <itunes:author>
	Images      []XMLImage `xml:"image"`  // 免疫 Namespace 设计：一并收集 <image> 和 <itunes:image>
	Items       []XMLItem `xml:"item"`
}

// XMLImage 能够同时兼容标准 <image><url></url></image> 和命名空间 <itunes:image href="..." />。
type XMLImage struct {
	URL  string `xml:"url"`       // 对应普通封面子标签
	Href string `xml:"href,attr"` // 对应 iTunes 封面属性
}

// XMLItem 对应单集的 <item> 标签。
type XMLItem struct {
	Title       string       `xml:"title"`
	Description string       `xml:"description"`
	PubDate     string       `xml:"pubDate"`
	Enclosure   XMLEnclosure `xml:"enclosure"`
}

// XMLEnclosure 对应音频附件 <enclosure> 标签。
type XMLEnclosure struct {
	URL    string `xml:"url,attr"`
	Length int64  `xml:"length,attr"`
	Type   string `xml:"type,attr"`
}

// FetchAndParseRSS 动态拉取指定 URL 的 RSS XML 源，并将其解析为规范化的 Podcast 结构体。
// 它具备 HTTP 超时保护、UA防拦截、状态码校验、数据流限制防内存溢出，以及完美的 Namespace 扁平化免疫过滤。
func FetchAndParseRSS(ctx context.Context, feedURL string) (*Podcast, error) {
	// 1. 边界校验：验证 URL 的有效性
	if feedURL == "" {
		return nil, fmt.Errorf("feed URL cannot be empty")
	}

	u, err := url.Parse(feedURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("invalid feed URL: %s", feedURL)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("unsupported URL scheme: %s, only http/https are supported", u.Scheme)
	}

	// 2. 强防御：设置 30 秒 HTTP 超时的 Client
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", feedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	// 设置标准的 UA 假装是现代浏览器或专业播客拉取端，防止被防爬防火墙拦截
	req.Header.Set("User-Agent", "xyz-dl/1.0.0 (Go-HTTP-Client; cross-platform)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch RSS feed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch RSS feed, HTTP status: %d %s", resp.StatusCode, resp.Status)
	}

	// 3. 强防御：限制最大读取大小为 30MB，防止恶意大 XML 占用宿主所有内存
	// 播客 RSS 通常在 1MB~10MB 之间，30MB 属于极其安全的余裕上限
	limitedReader := io.LimitReader(resp.Body, 30*1024*1024)

	var xmlRSS XMLRSS
	decoder := xml.NewDecoder(limitedReader)
	if err := decoder.Decode(&xmlRSS); err != nil {
		return nil, fmt.Errorf("failed to decode RSS XML: %w", err)
	}

	// 4. 规范化与兜底处理 (Normalizing & Fallbacks)
	podcast := &Podcast{
		Title:       strings.TrimSpace(xmlRSS.Channel.Title),
		Description: strings.TrimSpace(xmlRSS.Channel.Description),
	}

	// 4.1 提取作者：遍历 Authors 数组，优先提取第一个非空的作者
	var author string
	for _, a := range xmlRSS.Channel.Authors {
		a = strings.TrimSpace(a)
		if a != "" {
			author = a
			break
		}
	}
	if author == "" {
		author = "Unknown Author"
	}
	podcast.Author = author

	// 4.2 提取封面：遍历 Images 数组，优先取带 Href 的（itunes:image），其次是普通的 URL 封面
	var imageURL string
	for _, img := range xmlRSS.Channel.Images {
		if img.Href != "" {
			imageURL = strings.TrimSpace(img.Href)
			break
		}
	}
	if imageURL == "" {
		// 降级使用普通 image 标签
		for _, img := range xmlRSS.Channel.Images {
			if img.URL != "" {
				imageURL = strings.TrimSpace(img.URL)
				break
			}
		}
	}
	podcast.ImageURL = imageURL

	// 4.3 遍历单集：过滤掉没有有效音频直链的 item
	var episodes []Episode
	for _, item := range xmlRSS.Channel.Items {
		audioURL := strings.TrimSpace(item.Enclosure.URL)
		if audioURL == "" {
			// 防御：忽略没有 enclosure 音频下载地址的 item
			continue
		}

		episodes = append(episodes, Episode{
			Title:       strings.TrimSpace(item.Title),
			PubDate:     strings.TrimSpace(item.PubDate),
			AudioURL:    audioURL,
			Length:      item.Enclosure.Length,
			Description: strings.TrimSpace(item.Description),
		})
	}

	podcast.Episodes = episodes

	// 4.4 检查整个播客是否完全没有解析出有效的单集
	if len(podcast.Episodes) == 0 {
		return nil, fmt.Errorf("parsed RSS feed contains zero valid download-capable episodes")
	}

	return podcast, nil
}
