package db

import (
	"log"
)

// Position 机构级持仓数据舱 (极简无状态版)
type Position struct {
	ID         int     `json:"id"`
	TSCode     string  `json:"ts_code"`
	StockName  string  `json:"stock_name"`
	HoldVolume int     `json:"hold_volume"` // 持仓股数
	CostPrice  float64 `json:"cost_price"`  // 建仓成本价
	BuyDate    string  `json:"buy_date"`    // 建仓日期 (YYYYMMDD)
	Strategy   string  `json:"strategy"`    // 建仓策略 (MACB / CBBM / DSS)
}

// AddPosition 录入新持仓 (前端买入后调用)
func AddPosition(pos Position) error {
	query := `
		INSERT INTO my_positions (ts_code, stock_name, hold_volume, cost_price, buy_date, strategy)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	_, err := DB.Exec(query, pos.TSCode, pos.StockName, pos.HoldVolume, pos.CostPrice, pos.BuyDate, pos.Strategy)
	if err != nil {
		log.Printf("❌ 写入持仓失败 [%s]: %v\n", pos.TSCode, err)
		return err
	}
	return nil
}

// GetAllPositions 提取全量持仓，交由策略引擎审判
func GetAllPositions() ([]Position, error) {
	var positions []Position
	query := `SELECT id, ts_code, stock_name, hold_volume, cost_price, buy_date, COALESCE(strategy, '') FROM my_positions ORDER BY id DESC`
	rows, err := DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var p Position
		if err := rows.Scan(&p.ID, &p.TSCode, &p.StockName, &p.HoldVolume, &p.CostPrice, &p.BuyDate, &p.Strategy); err == nil {
			positions = append(positions, p)
		}
	}
	return positions, nil
}

// DeletePosition 移除指定持仓记录
func DeletePosition(id int) error {
	_, err := DB.Exec(`DELETE FROM my_positions WHERE id = ?`, id)
	return err
}
