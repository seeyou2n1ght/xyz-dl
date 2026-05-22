// Package utils 提供 xyz-dl 工具所需的跨平台文件名清洗与通用系统操作辅助工具。
package utils

import (
	"strings"
)

// SanitizeFilename 清洗并格式化文件名，使其在 Windows、Linux 等主流操作系统中安全合法。
// 它会过滤 Windows 禁用的非法字符 (\, /, :, *, ?, ", <, >, |)，过滤 ASCII 控制字符，
// 将非法字符替换为“空格”并合并连续空格，去除首尾空格和句点，并对超长标题进行安全截断（限制最大 rune 长度为 120）。
// 若清洗后的文件名为空，则返回默认名称 "untitled_episode"。
func SanitizeFilename(filename string) string {
	// 1. 边界校验：处理空输入
	if filename == "" {
		return "untitled_episode"
	}

	// 2. 过滤 Windows 非法字符与 ASCII 控制字符
	// Windows 非法字符: \ / : * ? " < > |
	illegalChars := `\/:*?"<>|`
	var sb strings.Builder

	for _, r := range filename {
		// 过滤 ASCII 控制字符 (0-31 以及 127)
		if r < 32 || r == 127 {
			continue
		}
		// 替换 Windows 非法字符为“空格”，便于安全断词，且能在纯特殊字符时自然被 Trim 掉
		if strings.ContainsRune(illegalChars, r) {
			sb.WriteRune(' ')
		} else {
			sb.WriteRune(r)
		}
	}

	result := sb.String()

	// 3. 去除首尾的空格和 Windows 不允许结尾的句点 "."
	result = strings.TrimSpace(result)
	for strings.HasSuffix(result, ".") {
		result = strings.TrimSuffix(result, ".")
		result = strings.TrimSpace(result)
	}

	// 4. 合并连续的空格，让最终生成的文件名极其干净整洁
	var cleanSb strings.Builder
	lastWasSpace := false
	for _, r := range result {
		if r == ' ' {
			if !lastWasSpace {
				cleanSb.WriteRune(' ')
				lastWasSpace = true
			}
		} else {
			cleanSb.WriteRune(r)
			lastWasSpace = false
		}
	}
	result = cleanSb.String()

	// 5. 对超长文件名做安全截断（防 Windows MAX_PATH 260 字节限制）
	// 使用 rune 保证多字节字符（如中文）截断时的完整性，防止产生半个汉字的乱码
	runes := []rune(result)
	if len(runes) > 120 {
		result = string(runes[:120])
		// 截断后重新去除可能新产生的首尾空格或句点
		result = strings.TrimSpace(result)
		for strings.HasSuffix(result, ".") {
			result = strings.TrimSuffix(result, ".")
			result = strings.TrimSpace(result)
		}
	}

	// 6. 防御性校验：若清洗完后为空（例如输入全是特殊非法字符），提供默认名称
	if result == "" {
		return "untitled_episode"
	}

	return result
}
