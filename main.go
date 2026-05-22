// xyz-dl 是一款高并发、零依赖、跨平台的播客批量下载与 Shownotes Markdown 导出的命令行利器。
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"

	"xyz-dl/pkg/downloader"
	"xyz-dl/pkg/parser"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
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
  小宇宙 RSS 命令行批量下载利器 (v2.0.0)
`
	outputDir   string
	concurrency int
	limit       int
	asJSON      bool
	saveMeta    bool
)

func main() {
	// 1. 全局上下文安全中断 (Ctrl+C)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		fmt.Println("\n\n[中断信号] 接收到 Ctrl+C，正在安全结束并清理网络连接...")
		cancel()
	}()

	// 2. 双击无参极简模式 (Interactive TUI Wizard)
	if len(os.Args) == 1 {
		runInteractiveWizard(ctx)
		return
	}

	// 3. 带参命令行黑盒模式 (Automated CLI)
	rootCmd := &cobra.Command{
		Use:   "xyz-dl",
		Short: "xyz-dl 是一款跨平台、零依赖的高并发播客 RSS 批量下载工具",
		Long:  banner + "\n支持高并发多线程拉取、自适应 Range 断点续传，以及一键导出 YAML Front Matter 封装的 Markdown Shownotes 稿件。",
	}

	infoCmd := &cobra.Command{
		Use:   "info <RSS_URL>",
		Short: "解析并展示播客订阅源的详细元数据与最新单集清单",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			feedURL := args[0]
			podcast, err := parser.FetchAndParseRSS(ctx, feedURL)
			if err != nil {
				return err
			}

			if asJSON {
				encoder := json.NewEncoder(os.Stdout)
				encoder.SetIndent("", "  ")
				return encoder.Encode(podcast)
			}

			printPodcastInfo(podcast)
			return nil
		},
	}
	infoCmd.Flags().BoolVarP(&asJSON, "json", "j", false, "以格式化的 JSON 形式在控制台输出，便于第三方工具集成")

	downloadCmd := &cobra.Command{
		Use:   "download <RSS_URL>",
		Short: "静默批量并发下载播客音频及 Shownotes，纯粹的自动化黑盒",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			feedURL := args[0]
			podcast, err := parser.FetchAndParseRSS(ctx, feedURL)
			if err != nil {
				return err
			}

			dl := downloader.NewDownloader(outputDir, concurrency, limit, saveMeta)
			return dl.DownloadPodcast(ctx, podcast)
		},
	}
	downloadCmd.Flags().StringVarP(&outputDir, "output", "o", "./Downloads", "音频与 Shownotes 的根输出目录")
	downloadCmd.Flags().BoolVarP(&saveMeta, "meta", "m", true, "是否同时导出 Shownotes 元数据为 Markdown 文件")
	downloadCmd.Flags().IntVarP(&concurrency, "concurrency", "p", 3, "批量下载的最大并发协程数上限")
	downloadCmd.Flags().IntVarP(&limit, "limit", "l", 0, "仅下载最新发布的前 N 期单集 (0 表示下载全部)")

	rootCmd.AddCommand(infoCmd, downloadCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// printPodcastInfo 打印播客元数据
func printPodcastInfo(podcast *parser.Podcast) {
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
}

// runInteractiveWizard 执行无参双击交互向导
func runInteractiveWizard(ctx context.Context) {
	fmt.Println(banner)
	
	for {
		if ctx.Err() != nil {
			return
		}

		var action string
		prompt := &survey.Select{
			Message: "请选择您要执行的操作:",
			Options: []string{"🚀 下载播客", "📖 查看帮助信息", "🚪 退出"},
		}
		survey.AskOne(prompt, &action)

		switch action {
		case "🚪 退出":
			fmt.Println("\n感谢使用 xyz-dl，再见！")
			return
		case "📖 查看帮助信息":
			fmt.Println("\n========================= 📖 使用指南 =========================")
			fmt.Println("xyz-dl 是一款跨平台、零依赖的高并发播客 RSS 批量下载工具。")
			fmt.Println("\n1. 【双击极简模式】(当前所处模式)：")
			fmt.Println("   直接双击运行程序，跟随向导输入 RSS 链接，即可轻松下载节目。")
			fmt.Println("\n2. 【自动化黑盒模式】：")
			fmt.Println("   在命令行或脚本中带参数调用，非常适合 CI/CD 定时任务！")
			fmt.Println("   获取信息: xyz-dl info \"<RSS_URL>\"")
			fmt.Println("   批量下载: xyz-dl download \"<RSS_URL>\" -o \"./MyAudio\" -p 5")
			fmt.Println("==================================================================")
			fmt.Println()
			fmt.Println("按回车键返回主菜单...")
			bufio.NewReader(os.Stdin).ReadBytes('\n')
		case "🚀 下载播客":
			err := runInteractiveDownload(ctx)
			if err != nil {
				if err != context.Canceled && !errors.Is(err, terminal.InterruptErr) {
					fmt.Printf("\n[发生错误] %v\n", err)
				}
			}
			fmt.Println("\n按回车键退出程序...")
			bufio.NewReader(os.Stdin).ReadBytes('\n')
			return
		default:
			return
		}
	}
}

// runInteractiveDownload 处理交互式的下载流程
func runInteractiveDownload(ctx context.Context) error {
	var feedURL string
	promptURL := &survey.Input{
		Message: "请输入播客 RSS 订阅链接 (支持右键粘贴):",
	}
	if err := survey.AskOne(promptURL, &feedURL); err != nil {
		return err
	}

	feedURL = strings.TrimSpace(feedURL)
	if feedURL == "" {
		return fmt.Errorf("RSS 链接不能为空")
	}

	fmt.Println("\n⏳ 正在解析 RSS 订阅源，请稍候...")
	podcast, err := parser.FetchAndParseRSS(ctx, feedURL)
	if err != nil {
		return err
	}

	fmt.Println("==================================================================")
	fmt.Printf(" 正在拉取播客 《%s》 的节目清单，请选择...\n", podcast.Title)
	fmt.Println("==================================================================")

	selectAllStr := "[🌟 全选下载所有单集]"
	options := make([]string, len(podcast.Episodes)+1)
	options[0] = selectAllStr
	for i, ep := range podcast.Episodes {
		options[i+1] = fmt.Sprintf("[%d] %s (%.2f MB)", i+1, ep.Title, float64(ep.Length)/(1024*1024))
	}

	var selected []string
	promptSelect := &survey.MultiSelect{
		Message:  "使用上下键移动，空格键勾选/取消，回车键确认开始下载:",
		Options:  options,
		PageSize: 15,
	}

	if err := survey.AskOne(promptSelect, &selected); err != nil {
		return err
	}

	if len(selected) == 0 {
		fmt.Println("\n[操作取消] 您未勾选任何单集，下载进程已停止。")
		return nil
	}

	var selectedEpisodes []parser.Episode
	
	// 判断是否包含全选
	isSelectAll := false
	for _, sel := range selected {
		if sel == selectAllStr {
			isSelectAll = true
			break
		}
	}

	if isSelectAll {
		selectedEpisodes = podcast.Episodes
		fmt.Println("\n[🌟 已全选] 将为您下载所有单集！")
	} else {
		for _, sel := range selected {
			if sel == selectAllStr {
				continue
			}
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
	}

	podcast.Episodes = selectedEpisodes

	// 交互模式下询问输出目录、并发数
	var outDir string
	survey.AskOne(&survey.Input{
		Message: "请输入保存目录 (默认为 ./Downloads):",
		Default: "./Downloads",
	}, &outDir)

	// 交互模式下询问是否保存元数据
	survey.AskOne(&survey.Confirm{
		Message: "是否同时下载并保存单集元数据为 Markdown 文件 (Shownotes)?",
		Default: true,
	}, &saveMeta)

	dl := downloader.NewDownloader(outDir, 3, 0, saveMeta)
	return dl.DownloadPodcast(ctx, podcast)
}
