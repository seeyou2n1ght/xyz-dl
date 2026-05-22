package downloader

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"xyz-dl/pkg/parser"
)

// TestDownloader 验证并发断点续传引擎在首次下载、断点拼接，以及 Shownotes Markdown 导出的行为
func TestDownloader(t *testing.T) {
	// 1. 准备 Mock 音频源数据 (100 字节的虚拟音频流)
	audioContent := strings.Repeat("A", 100)

	// 2. 启动 Mock HTTP 服务器，支持完整的 HTTP Range 断点续传
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/x-m4a")
		w.Header().Set("Accept-Ranges", "bytes")

		rangeHeader := r.Header.Get("Range")
		if rangeHeader == "" {
			// 没有 Range 头，返回 200 OK 并写入全量数据
			w.Header().Set("Content-Length", strconv.Itoa(len(audioContent)))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(audioContent))
			return
		}

		// 解析 Range 头 (格式通常为 bytes=start-)
		if !strings.HasPrefix(rangeHeader, "bytes=") {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		parts := strings.Split(rangeHeader[6:], "-")
		start, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil || start >= int64(len(audioContent)) {
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
		}

		// 返回 206 Partial Content，输出剩余切片流
		subContent := audioContent[start:]
		w.Header().Set("Content-Length", strconv.Itoa(len(subContent)))
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(audioContent)-1, len(audioContent)))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte(subContent))
	}))
	defer server.Close()

	// 创建临时测试工作区
	tempDir, err := os.MkdirTemp("", "xyz-dl-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 3. 运行测试套件
	t.Run("Full Download and Shownotes Export", func(t *testing.T) {
		dl := NewDownloader(tempDir, 1, 0)
		podcast := &parser.Podcast{
			Title:  "TestPodcast",
			Author: "Tester",
			Episodes: []parser.Episode{
				{
					Title:       "Episode 1",
					PubDate:     "Fri, 22 May 2026",
					AudioURL:    server.URL + "/ep1.m4a", // 匹配 m4a 后缀
					Length:      100,
					Description: "This is a great episode.",
				},
			},
		}

		err := dl.DownloadPodcast(context.Background(), podcast)
		if err != nil {
			t.Fatalf("unexpected download error: %v", err)
		}

		// 验证物理音频文件是否存在且大小完全为 100 字节
		audioFile := filepath.Join(tempDir, "TestPodcast", "Episode 1.m4a")
		info, err := os.Stat(audioFile)
		if err != nil {
			t.Fatalf("audio file not found: %v", err)
		}
		if info.Size() != 100 {
			t.Errorf("expected audio size 100, got %d", info.Size())
		}

		// 验证 Shownotes Markdown 文件是否存在且内容包含关键字与 YAML Front Matter
		mdFile := filepath.Join(tempDir, "TestPodcast", "Episode 1.md")
		mdContent, err := os.ReadFile(mdFile)
		if err != nil {
			t.Fatalf("shownotes markdown not found: %v", err)
		}
		mdStr := string(mdContent)
		if !strings.Contains(mdStr, "This is a great episode.") {
			t.Errorf("markdown does not contain episode description")
		}
		if !strings.Contains(mdStr, "title:") || !strings.Contains(mdStr, "audioURL:") {
			t.Errorf("markdown does not contain valid YAML Front Matter")
		}
	})

	t.Run("Resume Download (Breakpoint Resume)", func(t *testing.T) {
		// 创建独立的续传测试目录
		resumeDir := filepath.Join(tempDir, "resume")
		podcastDir := filepath.Join(resumeDir, "TestPodcast")
		_ = os.MkdirAll(podcastDir, 0755)

		// 提前模拟一个只下载了 40 字节的受损/未下载完的音频断点文件
		audioFile := filepath.Join(podcastDir, "Episode 1.m4a")
		err := os.WriteFile(audioFile, []byte(strings.Repeat("A", 40)), 0644)
		if err != nil {
			t.Fatalf("failed to create breakpoint file: %v", err)
		}

		dl := NewDownloader(resumeDir, 1, 0)
		podcast := &parser.Podcast{
			Title:  "TestPodcast",
			Author: "Tester",
			Episodes: []parser.Episode{
				{
					Title:       "Episode 1",
					PubDate:     "Fri, 22 May 2026",
					AudioURL:    server.URL + "/ep1.m4a",
					Length:      100,
					Description: "This is a great episode.",
				},
			},
		}

		// 启动续传，应该自动识别断点大小 40 字节，并发送 Range 请求追加剩余 60 字节
		err = dl.DownloadPodcast(context.Background(), podcast)
		if err != nil {
			t.Fatalf("unexpected resume error: %v", err)
		}

		// 验证续传并合成后的物理文件是否刚好拼接为 100 字节
		info, err := os.Stat(audioFile)
		if err != nil {
			t.Fatalf("audio file not found after resume: %v", err)
		}
		if info.Size() != 100 {
			t.Errorf("expected final audio size 100, got %d", info.Size())
		}
	})
}
