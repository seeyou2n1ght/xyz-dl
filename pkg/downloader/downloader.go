// Package downloader 提供高并发控制、HTTP Range 断点续传，以及 Shownotes 的 Markdown 导出引擎。
package downloader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"xyz-dl/pkg/parser"
	"xyz-dl/pkg/utils"

	"github.com/schollz/progressbar/v3"
)

// Downloader 承载了批量下载器的全局配置与核心引擎。
type Downloader struct {
	OutputDir   string // 音频与 Shownotes 的根输出目录
	Concurrency int    // 最大并发协程数限制
	Limit       int    // 下载最新单集的限制数量 (0 表示全部下载)
	SaveMeta    bool   // 是否保存 Markdown Shownotes
}

// NewDownloader 实例化并配置一个下载器，同时提供边界校验与默认值兜底。
func NewDownloader(outputDir string, concurrency int, limit int, saveMeta bool) *Downloader {
	// 防错兜底
	if outputDir == "" {
		outputDir = "./Downloads"
	}
	if concurrency <= 0 {
		concurrency = 3
	}
	return &Downloader{
		OutputDir:   outputDir,
		Concurrency: concurrency,
		Limit:       limit,
		SaveMeta:    saveMeta,
	}
}

// DownloadPodcast 批量并发下载播客频道的音频和 Shownotes，支持 Range 断点续传与终端进度条渲染。
func (d *Downloader) DownloadPodcast(ctx context.Context, podcast *parser.Podcast) error {
	// 1. 创建播客专属的物理子目录
	podcastFolder := filepath.Join(d.OutputDir, utils.SanitizeFilename(podcast.Title))
	if err := os.MkdirAll(podcastFolder, 0755); err != nil {
		return fmt.Errorf("failed to create podcast directory %s: %w", podcastFolder, err)
	}

	// 2. 限制最新单集下载数量
	episodes := podcast.Episodes
	if d.Limit > 0 && d.Limit < len(episodes) {
		episodes = episodes[:d.Limit]
	}

	fmt.Printf("\n[开始下载] 播客: %s | 主持/作者: %s | 待下载集数: %d\n", podcast.Title, podcast.Author, len(episodes))
	fmt.Printf("[保存路径] %s\n", podcastFolder)
	fmt.Printf("[并发上限] %d 任务同行\n\n", d.Concurrency)

	// 智能去重机制 (Smart Deduplication)
	seenTitles := make(map[string]int)
	type DownloadTask struct {
		Episode     parser.Episode
		UniqueTitle string
	}
	var tasks []DownloadTask
	for _, ep := range episodes {
		cleanTitle := utils.SanitizeFilename(ep.Title)
		count := seenTitles[cleanTitle]
		seenTitles[cleanTitle]++

		uniqueTitle := cleanTitle
		if count > 0 {
			uniqueTitle = fmt.Sprintf("%s (%d)", cleanTitle, count)
		}
		tasks = append(tasks, DownloadTask{Episode: ep, UniqueTitle: uniqueTitle})
	}

	// 3. 构建有缓冲通道控制协程并发数，使用 WaitGroup 同步
	sem := make(chan struct{}, d.Concurrency)
	var wg sync.WaitGroup

	for _, task := range tasks {
		wg.Add(1)
		
		select {
		case sem <- struct{}{}: // 抢占槽位
		case <-ctx.Done(): // 接收到中断信号
			wg.Done()
			continue
		}

		go func(task DownloadTask) {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("[致命异常] 单集 '%s' 内部发生崩溃并被拦截: %v\n", task.UniqueTitle, r)
				}
				<-sem // 释放槽位
				wg.Done()
			}()

			// 执行单任务物理下载
			if err := d.downloadEpisode(ctx, podcast.Title, podcastFolder, task.Episode, task.UniqueTitle); err != nil {
				// Avoid printing interruption errors when user hits Ctrl+C
				if err != context.Canceled {
					fmt.Printf("[下载失败] %s: %v\n", task.UniqueTitle, err)
				}
			}
		}(task)
	}

	wg.Wait()
	fmt.Println("\n[任务完成] 所有播客音频与 Shownotes 文字稿已全部处理完毕！")
	return nil
}

// downloadEpisode 执行单个音频文件的断点续传下载，并导出 YAML 封装的 Markdown Shownotes。
func (d *Downloader) downloadEpisode(ctx context.Context, podcastTitle string, podcastFolder string, ep parser.Episode, uniqueTitle string) error {
	suffix := getSuffix(ep.AudioURL)
	audioPath := filepath.Join(podcastFolder, uniqueTitle+suffix)
	tempAudioPath := audioPath + ".downloading"
	mdPath := filepath.Join(podcastFolder, uniqueTitle+".md")

	// 1. 保存 Shownotes (文字介绍) 为 Markdown
	if d.SaveMeta {
		if err := d.saveShownotes(mdPath, ep, uniqueTitle); err != nil {
			// 容错：Shownotes 失败不应该打断音频的下载
			fmt.Printf("[警告] 保存 Shownotes 失败 %s: %v\n", ep.Title, err)
		}
	}

	// 2. 音频断点状态探测
	var localSize int64
	var fileFlags int
	var fileExists bool

	// 优先检查最终文件是否存在
	info, err := os.Stat(audioPath)
	if err == nil && info.Size() == ep.Length {
		// 态 ①：完全一致，直接跳过
		fmt.Printf("[已存在] %s%s (下载完整，已跳过)\n", uniqueTitle, suffix)
		return nil
	}

	// 检查临时断点文件
	info, err = os.Stat(tempAudioPath)
	if err == nil {
		fileExists = true
		localSize = info.Size()
		if localSize > ep.Length {
			// 态 ②：本地比服务端还大（元数据或链接变更），需删除重试
			_ = os.Remove(tempAudioPath)
			localSize = 0
			fileExists = false
		}
	}

	if fileExists && localSize > 0 {
		fileFlags = os.O_APPEND | os.O_WRONLY
	} else {
		fileFlags = os.O_CREATE | os.O_WRONLY
		localSize = 0
	}

	// 3. 构建高颜值进度条
	barDescription := fmt.Sprintf("[下载中] %s", uniqueTitle)
	if len([]rune(barDescription)) > 40 {
		barDescription = string([]rune(barDescription)[:37]) + "..."
	}

	bar := progressbar.NewOptions64(
		ep.Length,
		progressbar.OptionSetDescription(barDescription),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(12),
		progressbar.OptionThrottle(100*time.Millisecond),
		progressbar.OptionShowCount(),
		progressbar.OptionOnCompletion(func() {
			fmt.Fprint(os.Stderr, "\n")
		}),
		progressbar.OptionFullWidth(),
	)
	// 补足断点续传的初始进度
	if localSize > 0 {
		bar.Add64(localSize)
	}

	// 4. 发起 HTTP GET 请求，配置 Range 头
	client := &http.Client{
		Timeout: 10 * time.Minute, // 10分钟超时保障大音频不被掐断
	}

	req, err := http.NewRequestWithContext(ctx, "GET", ep.AudioURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create http request: %w", err)
	}
	req.Header.Set("User-Agent", "xyz-dl/0.0.1 (Go-HTTP-Client; cross-platform)")
	if localSize > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", localSize))
	}

	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("http transfer error: %w", err)
	}
	defer resp.Body.Close()

	// 5. 校验 Response 并防御性回滚
	if localSize > 0 {
		if resp.StatusCode != http.StatusPartialContent {
			// 防御性回滚：服务器不支持 Range，回滚到从 0 开始下载
			localSize = 0
			fileFlags = os.O_CREATE | os.O_WRONLY
			// 重新拉取没有 Range 头的连接
			req.Header.Del("Range")
			resp.Body.Close()
			resp, err = client.Do(req)
			if err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				return fmt.Errorf("http transfer error after rollback: %w", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("http server returned status: %d %s", resp.StatusCode, resp.Status)
			}
			bar.Reset()
		}
	} else {
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
			return fmt.Errorf("http server returned status: %d %s", resp.StatusCode, resp.Status)
		}
	}

	// 6. 写入本地文件
	out, err := os.OpenFile(tempAudioPath, fileFlags, 0644)
	if err != nil {
		return fmt.Errorf("failed to open output file: %w", err)
	}
	defer out.Close()

	// 使用 io.CopyBuffer 提升 I/O 性能 (1MB buffer)
	buf := make([]byte, 1024*1024)
	_, err = io.CopyBuffer(out, io.TeeReader(resp.Body, bar), buf)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("download interrupted: %w", err)
	}

	// 安全关闭文件句柄，确保能在 Windows 下正常 rename，并拦截磁盘 Flush 错误
	if err := out.Close(); err != nil {
		return fmt.Errorf("failed to flush and close file: %w", err)
	}

	// 7. 下载成功，原子重命名为最终扩展名
	if err := os.Rename(tempAudioPath, audioPath); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// saveShownotes 将单集的文字介绍转换为带 YAML Front Matter 的高颜值 Markdown 稿件保存。
func (d *Downloader) saveShownotes(mdPath string, ep parser.Episode, uniqueTitle string) error {
	// 防御：若 Shownotes 已经存在，则无需重复写入
	if _, err := os.Stat(mdPath); err == nil {
		return nil
	}

	content := fmt.Sprintf(`---
title: %q
pubDate: %q
audioURL: %q
length: %d bytes
exportTime: %q
---

# %s

## 单集介绍 (Shownotes)

%s
`, uniqueTitle, ep.PubDate, ep.AudioURL, ep.Length, time.Now().Format("2006-01-02 15:04:05"), uniqueTitle, ep.Description)

	return os.WriteFile(mdPath, []byte(content), 0644)
}

// getSuffix 智能分析并提取 URL 后缀名，提供严密的安全白名单保障与默认值兜底。
func getSuffix(audioURL string) string {
	u, err := url.Parse(audioURL)
	if err != nil {
		return ".m4a"
	}
	path := u.Path
	pos := strings.LastIndex(path, ".")
	if pos == -1 {
		return ".m4a"
	}
	ext := strings.ToLower(path[pos:])
	// 只允许主流的音频扩展格式，防目录遍历与非法脚本伪造
	if ext == ".mp3" || ext == ".m4a" || ext == ".mp4" || ext == ".wav" || ext == ".aac" {
		return ext
	}
	return ".m4a"
}
