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

## 特性

- **反爬策略绕过**：内置标准的浏览器 User-Agent 伪装，有效绕过主流 CDN 与 WAF（如阿里云、Tengine 等）的访问控制拦截，解决 403 Forbidden 报错问题。
- **网络流控与连接复用**：全局接管 HTTP 请求连接池，支持 TCP 长连接与 TLS 握手复用。在多线程大批量下载时，大幅降低底层网络开销与内存分配。
- **真·断点续传与防伪探测**：在物理拉取前注入轻量的 HEAD 预请求，强制提取服务器真实的 Content-Length。完美解决因 RSS 节点元数据虚标导致的无限重试与卡死漏洞。
- **平滑的多路控制台视图**：基于专用的 `mpb` 渲染引擎，在执行数百期单集的高并发下载时，提供防串流、防重叠的多行进度条展示与动态 ETA 计算。
- **智能交互向导**：直接运行 `xyz-dl`（或在 Windows 下双击 `xyz-dl.exe`）即可进入控制台向导，支持批量勾选与一键全选。
- **自动化支持**：在命令行传入子命令参数时转为纯静默运行，适合集成入 CI/CD 或定时脚本。
- **安全落地**：使用 `.downloading` 临时后缀进行原子写入，确保文件完整。内置 `context` 优雅关闭机制，随时 Ctrl+C 安全中断不留脏数据。
- **智能去重命名**：按需进行文件名去重，并安全清洗系统禁用字符，保留干净整洁的原始文件名。
- **元数据导出**：自动将单集文字简介生成同名 `.md` 格式文件，并在头部嵌入 YAML Front Matter 元数据。

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
