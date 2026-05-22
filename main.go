// xyz-dl 是一款高并发、零依赖、跨平台的播客批量下载与 Shownotes Markdown 导出的命令行利器。
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"xyz-dl/pkg/downloader"
	"xyz-dl/pkg/parser"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

var (
	banner = `
__   __ __   __  ______        _____   _      
\ \ / / \ \ / / |___  /  ___  |  __ \ | |     
 \ V /   \ V /     / /  |___| | |  | || |     
  > <     | |     / /         | |  | || |     
 / ^ \    | |    / /__        | |__| || |____ 
/_/ \_\   |_|   /_____|       |_____/ |______|
  小宇宙 RSS 命令行批量下载利器 (v1.0.0)
`
	outputDir   string
	concurrency int
	limit       int
	interactive bool
	asJSON      bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "xyz-dl",
		Short: "xyz-dl 是一款跨平台、零依赖的高并发播客 RSS 批量下载工具",
		Long:  banner + "\n支持高并发多线程拉取、自适应 Range 断点续传，以及一键导出 YAML Front Matter 封装的 Markdown Shownotes 稿件。",
	}

	// 1. info 子命令：解析并展示播客基本信息
	infoCmd := &cobra.Command{
		Use:   "info <RSS_URL>",
		Short: "解析并展示播客订阅源的详细元数据与最新单集清单",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			feedURL := args[0]
			podcast, err := parser.FetchAndParseRSS(feedURL)
			if err != nil {
				return err
			}

			// 以整洁的 JSON 打印输出，供自动化流水线对接
			if asJSON {
				encoder := json.NewEncoder(os.Stdout)
				encoder.SetIndent("", "  ")
				return encoder.Encode(podcast)
			}

			// 高颜值控制台元数据渲染
			fmt.Println(banner)
			fmt.Println("==================================================================")
			fmt.Printf("【播客标题】 %s\n", podcast.Title)
			fmt.Printf("【主持/作者】 %s\n", podcast.Author)
			fmt.Printf("【封面图片】 %s\n", podcast.ImageURL)
			fmt.Printf("【简介说明】 %s\n", podcast.Description)
			fmt.Printf("【单集总数】 %d 期\n", len(podcast.Episodes))
			fmt.Println("==================================================================")
			fmt.Println("\n【最新 10 期单集清单】")
			for i, ep := range podcast.Episodes {
				if i >= 10 {
					fmt.Printf("  ... 还有 %d 期单集未展示 ...\n", len(podcast.Episodes)-10)
					break
				}
				fmt.Printf("  [%02d] %s (%s | %.2f MB)\n", i+1, ep.Title, ep.PubDate, float64(ep.Length)/(1024*1024))
			}
			fmt.Println()
			return nil
		},
	}
	infoCmd.Flags().BoolVarP(&asJSON, "json", "j", false, "以格式化的 JSON 形式在控制台输出，便于第三方工具集成")

	// 2. download 子命令：下载音频与 Shownotes
	downloadCmd := &cobra.Command{
		Use:   "download <RSS_URL>",
		Short: "批量并发下载播客音频及 Shownotes，支持交互式勾选和断点续传",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			feedURL := args[0]
			podcast, err := parser.FetchAndParseRSS(feedURL)
			if err != nil {
				return err
			}

			dl := downloader.NewDownloader(outputDir, concurrency, limit)

			// 启用终端交互式多选勾选菜单
			if interactive {
				fmt.Println(banner)
				fmt.Println("==================================================================")
				fmt.Printf(" 正在拉取播客 《%s》 的节目清单，请选择...\n", podcast.Title)
				fmt.Println("==================================================================")

				// 防错与重名防御：通过带有序号的展示选项进行绑定
				options := make([]string, len(podcast.Episodes))
				for i, ep := range podcast.Episodes {
					options[i] = fmt.Sprintf("[%d] %s (%.2f MB)", i+1, ep.Title, float64(ep.Length)/(1024*1024))
				}

				var selected []string
				prompt := &survey.MultiSelect{
					Message:  "使用上下键移动，空格键勾选/取消，回车键确认开始下载:",
					Options:  options,
					Default:  options, // 默认全选，最符合通用下载直觉
					PageSize: 12,       // 精准限制一屏最多展示 12 行，防止溢出刷屏
				}

				if err := survey.AskOne(prompt, &selected); err != nil {
					return fmt.Errorf("interactive operation cancelled: %w", err)
				}

				if len(selected) == 0 {
					fmt.Println("\n[操作取消] 您未勾选任何单集，下载进程已停止。")
					return nil
				}

				// 将勾选项反向解析回内存中的 Episode 切片
				var selectedEpisodes []parser.Episode
				for _, sel := range selected {
					pos := strings.Index(sel, "]")
					if pos == -1 {
						continue
					}
					idxStr := sel[1:pos]
					idx, err := strconv.Atoi(idxStr)
					if err != nil {
						continue
					}
					selectedEpisodes = append(selectedEpisodes, podcast.Episodes[idx-1])
				}
				podcast.Episodes = selectedEpisodes
			}

			return dl.DownloadPodcast(podcast)
		},
	}

	// 绑定 Flags 参数
	downloadCmd.Flags().StringVarP(&outputDir, "output", "o", "./Downloads", "音频与 Shownotes 的根输出目录")
	downloadCmd.Flags().IntVarP(&concurrency, "concurrency", "p", 3, "批量下载的最大并发协程数上限")
	downloadCmd.Flags().IntVarP(&limit, "limit", "l", 0, "仅下载最新发布的前 N 期单集 (0 表示下载全部，在交互模式下会被覆盖)")
	downloadCmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "启动终端交互式复选框菜单，精准勾选想要下载的单集")

	// 注册子路由并执行
	rootCmd.AddCommand(infoCmd, downloadCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
