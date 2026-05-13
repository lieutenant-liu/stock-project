package stockutil

import "strings"

// IsValidMainBoardCode 判断股票代码是否属于主板（沪深主板）。
// 允许前缀：000/001/002/003 (深市主板)、600/601/603/605 (沪市主板)。
// 排除：300 (创业板)、688 (科创板)、4/8/9 开头 (北交所)。
func IsValidMainBoardCode(tsCode string) bool {
	// 取 "." 之前的部分作为纯数字代码
	code := tsCode
	if idx := strings.IndexByte(tsCode, '.'); idx >= 0 {
		code = tsCode[:idx]
	}
	if len(code) < 3 {
		return false
	}

	prefix3 := code[:3]
	switch prefix3 {
	case "000", "001", "002", "003", // 深市主板
		"600", "601", "603", "605": // 沪市主板
		return true
	}
	return false
}
