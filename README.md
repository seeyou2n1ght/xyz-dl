# xyz-dl

```text
__   __ __   __  ______        _____   _      
\ \ / / \ \ / / |___  /  ___  |  __ \ | |     
 \ V /   \ V /     / /  |___| | |  | || |     
  > <     | |     / /         | |  | || |     
 / ^ \    | |    / /__        | |__| || |____ 
/_/ \_\   |_|   /_____|       |_____/ |______|
```

`xyz-dl` 是一个跨平台、零依赖的命令行播客批量下载工具。
由 Go 语言编写，编译为单一静态文件，开箱即用。支持多线程并发、HTTP Range 断点续传，以及一键导出播客的 Shownotes 为 Markdown 文本。

---

## 核心特性

### 高效的下载引擎
- **断点续传与大小校验**：下载前通过 HEAD 请求获取文件真实的 Content-Length，避免因 RSS 元数据不准确导致的进度计算错误。支持对未完成的 `.downloading` 临时文件进行断点续传。
- **限流退避重试 (Jitter Backoff)**：内置针对 `429 Too Many Requests` 和 `403 Forbidden` 的重试机制，结合随机毫秒级 Jitter 休眠，提高在严格限流环境下的下载成功率。
- **网络连接复用**：基于自定义的 HTTP Transport 控制连接池与读写超时，支持 TCP 长连接与 TLS 握手复用，优化大文件及多任务并发时的网络开销。

### 数据解析与元数据处理
- **ID3v2 标签写入**：下载完成后，工具会自动获取播客封面图，并将其连同播客频道、单集标题、主播名称等信息作为 ID3v2 标签写入音频文件（`.m4a`/`.mp3`），以兼容各类本地播放器的图文展示需求。
- **Markdown Shownotes 导出**：自动解析 RSS 中的 `<description>` 节点，清理残留的 Web 标签并转换为 Markdown 格式，生成包含 YAML Front Matter 的规范 `.md` 文档。
- **路径长度与特殊字符兼容**：不仅过滤各操作系统的非法路径字符，同时内置了文件名长度截断机制（限制 120 字符），避免在 Windows 等系统中因路径超长（如 260 字节限制）导致的文件创建错误。

### 终端与自动化支持
- **多任务进度展示**：采用专用的终端进度条组件，在多并发下载时保持界面整洁防重叠。在下载任务结束后，会提供交互式界面提示失败的单集，并支持一键重试。
- **双模式运行**：无参数直接运行或双击执行文件，将启动基于方向键和空格操作的 TUI 向导界面；带命令行参数执行时则作为静默后台任务运行，便于与 CI/CD 管道或定时脚本集成。

---

## 安装

您可以将下载的二进制文件添加到系统环境变量中以便全局调用。

**Windows:**
将 `xyz-dl.exe` 放入任意目录（如 `C:\Tools\`），以管理员身份打开 PowerShell 执行：
```powershell
[System.Environment]::SetEnvironmentVariable("Path", $env:Path + ";C:\Tools", "User")
```

**Linux / macOS:**
```bash
sudo mv xyz-dl /usr/local/bin/
sudo chmod +x /usr/local/bin/xyz-dl
```

---

## 获取播客 RSS 链接

本工具需要提供播客的原始 RSS 订阅链接。推荐通过 **NeoDB** 获取：

1. 访问 [NeoDB](https://neodb.social/)，搜索目标播客（筛选分类为“播客”）。
2. 进入播客详情页。
3. 在信息展示区域找到 **RSS** 图标或 **订阅源 / Feed** 链接。
4. 右键选择 **复制链接地址**。

---

## 使用说明

### 1. 交互模式 (向导界面)

在 Windows 环境下，直接双击运行 `xyz-dl.exe`（或者在终端无参数运行 `xyz-dl`）。
程序会启动交互向导，按提示粘贴 RSS 链接，即可通过上下键和空格勾选要下载的单集（支持一键全选），并可选择是否同时导出 Markdown 元数据。

### 2. 自动化模式 (命令行)

#### 获取信息 (`info`)
解析播客源并展示作者、简介和最新单集列表。支持输出为 JSON。
```bash
xyz-dl info "https://feed.xyzfm.space/7neh8whbtc9w"
xyz-dl info "https://feed.xyzfm.space/7neh8whbtc9w" --json
```

#### 批量下载 (`download`)
执行无干预的静默并发下载，适合脚本调用。
```bash
# 默认配置下载全部
xyz-dl download "https://feed.xyzfm.space/7neh8whbtc9w"

# 自定义参数下载（例如并发数为 5，只下载最新 5 期，不导出 shownotes）
xyz-dl download "https://feed.xyzfm.space/7neh8whbtc9w" -o "./MyPodcasts" -p 5 -l 5 -m=false
```

**参数说明**：
- `-o`, `--output`: 音频与 Shownotes 的根输出目录（默认 `./Downloads`）
- `-p`, `--concurrency`: 并发下载的协程数上限（默认 3）
- `-l`, `--limit`: 仅下载最新发布的前 N 期单集（默认 0，表示下载全部）
- `-m`, `--meta`: 是否同时下载并保存单集的 Markdown 元数据（默认 true）

---

## 下载目录结构

下载完成后，工具会以播客真实标题创建子目录，结构如下：

```text
D:/MyPodcasts/
└── 播客名称/
    ├── 单集标题.m4a
    ├── 单集标题.md
    ├── 重名单集标题 (1).m4a
    └── 重名单集标题 (1).md
```

## 许可
MIT License
