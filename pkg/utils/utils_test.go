package utils

import (
	"strings"
	"testing"
)

// TestSanitizeFilename 验证文件名清洗函数在各类边缘和非法输入下的行为
func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Normal filename",
			input:    "podcast_episode_1",
			expected: "podcast_episode_1",
		},
		{
			name:     "Filename with Windows illegal chars",
			input:    `Vol/88: 跌了你不敢/涨了你眼馋? *牛市* <进宝山> | "别空手"`,
			expected: "Vol 88 跌了你不敢 涨了你眼馋 牛市 进宝山 别空手",
		},
		{
			name:     "Filename with leading/trailing spaces and dots",
			input:    "  .Vol.88.  ",
			expected: ".Vol.88",
		},
		{
			name:     "Empty input",
			input:    "",
			expected: "untitled_episode",
		},
		{
			name:     "All illegal chars",
			input:    `\/:*?"<>|`,
			expected: "untitled_episode", // 清洗完后变空，走兜底防御
		},
		{
			name:     "Super long filename with Chinese characters",
			input:    strings.Repeat("这里是一个非常长非常长的中文测试标题用来测试多字节字符截断", 20),
			expected: "", // 在代码中单独验证长度
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := SanitizeFilename(tt.input)
			if tt.name == "Super long filename with Chinese characters" {
				// 针对超长标题，验证 rune 长度精准截断为 120 且无多字节乱码
				runes := []rune(actual)
				if len(runes) != 120 {
					t.Errorf("SanitizeFilename() length = %v, expected 120", len(runes))
				}
				// 确保不包含非法字符
				for _, r := range `\/:*?"<>|` {
					if strings.ContainsRune(actual, r) {
						t.Errorf("SanitizeFilename() returned string contains illegal character: %c", r)
					}
				}
			} else {
				if actual != tt.expected {
					t.Errorf("SanitizeFilename(%q) = %q, expected %q", tt.input, actual, tt.expected)
				}
			}
		})
	}
}
