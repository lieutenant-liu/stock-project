package backtest

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ExportCSV 将回测结果（交易流水 + 资产曲线）写入 CSV 文件。
// 使用 UTF-8 BOM 编码，确保 Excel 打开中文不乱码。
// 返回写入文件的绝对路径。
func ExportCSV(result *BacktestResult, exportsDir string) (string, error) {
	if err := os.MkdirAll(exportsDir, 0755); err != nil {
		return "", fmt.Errorf("创建导出目录失败: %w", err)
	}

	filename := fmt.Sprintf("backtest_%s_%s.csv",
		result.Config.Strategy,
		time.Now().Format("20060102_150405"),
	)
	fullPath := filepath.Join(exportsDir, filename)

	f, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("创建CSV文件失败: %w", err)
	}
	defer f.Close()

	// UTF-8 BOM
	if _, err := f.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		return "", err
	}

	w := csv.NewWriter(f)

	// Section 1: 交易流水
	header := []string{"股票代码", "买入日期", "买入价", "卖出日期", "卖出价", "股数", "盈亏金额", "收益率", "持仓天数", "所属策略", "买入原因", "卖出原因"}
	if err := w.Write(header); err != nil {
		return "", err
	}

	for _, t := range result.TradeLog {
		row := []string{
			t.TSCode,
			formatDate(t.BuyDate),
			fmt.Sprintf("%.2f", t.BuyPrice),
			formatDate(t.SellDate),
			fmt.Sprintf("%.2f", t.SellPrice),
			fmt.Sprintf("%d", t.Shares),
			fmt.Sprintf("%.2f", t.PnL),
			fmt.Sprintf("%.2f", t.ReturnPct),
			fmt.Sprintf("%d", t.HoldDays),
			t.Strategy,
			t.BuyReason,
			t.SellReason,
		}
		if err := w.Write(row); err != nil {
			return "", err
		}
	}

	// 空行分隔
	if err := w.Write(nil); err != nil {
		return "", err
	}

	// Section 2: 资产曲线
	if err := w.Write([]string{"日期", "净值"}); err != nil {
		return "", err
	}
	for _, ep := range result.EquityCurve {
		if err := w.Write([]string{formatDate(ep.Date), fmt.Sprintf("%.2f", ep.Value)}); err != nil {
			return "", err
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return "", err
	}

	absPath, _ := filepath.Abs(fullPath)
	return absPath, nil
}

// formatDate 将 YYYYMMDD 格式转为 YYYY-MM-DD，解析失败则返回原串。
func formatDate(ymd string) string {
	t, err := time.Parse("20060102", ymd)
	if err != nil {
		return ymd
	}
	return t.Format("2006-01-02")
}
