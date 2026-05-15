package strategy

import (
	"fmt"
	"math"
	"stock-backend/tushare"
)

// CheckGapTrap 微幅高开陷阱过滤器。
// 微幅高开 (0%~2%) 往往是诱多陷阱，而非真正的突破动能。
// 大幅高开 (>4%) 则追高风险过大。
func CheckGapTrap(openPrice, preClose float64) error {
	if preClose <= 0 {
		return nil // 数据缺失，跳过检查
	}
	gapPct := (openPrice - preClose) / preClose
	if gapPct > 0 && gapPct < 0.02 {
		return fmt.Errorf("微幅高开陷阱 (缺口 %.2f%%)，疑似诱多", gapPct*100)
	}
	if gapPct > 0.04 {
		return fmt.Errorf("高开过度风险 (缺口 %.2f%%)，追高成本过大", gapPct*100)
	}
	return nil
}

// CheckVolumeTrap 量能陷阱过滤器。
// 右侧突破：天量 (>3x MA20) 可能是主力出货的诱多信号。
// 左侧回踩：成交量过低 (<1.2x 回踩均量) 说明无资金承接。
func CheckVolumeTrap(todayVol, volMa20 float64, isRightSide bool, avgPullbackVol float64) error {
	if isRightSide {
		if volMa20 > 0 && todayVol > 3.0*volMa20 {
			return fmt.Errorf("天量突破诱多 (量比 %.1f vs MA20)，疑似主力出货", todayVol/volMa20)
		}
	} else {
		if avgPullbackVol > 0 && todayVol < 1.2*avgPullbackVol {
			return fmt.Errorf("回踩无资金承接 (量比 %.1f vs 回踩均量)", todayVol/avgPullbackVol)
		}
	}
	return nil
}

// CheckBodyRatio K线实体饱满度过滤器。
// 实体占比过低 (<60%) 说明长上影或长下影线明显，抛压沉重。
func CheckBodyRatio(today tushare.DailyKLine) error {
	amplitude := today.High - today.Low
	if amplitude <= 0 {
		return nil // 一字板，跳过
	}
	body := math.Abs(today.Close - today.Open)
	ratio := body / amplitude
	if ratio < 0.60 {
		return fmt.Errorf("K线实体不饱满 (实体占比 %.0f%%)，长影线抛压", ratio*100)
	}
	return nil
}
