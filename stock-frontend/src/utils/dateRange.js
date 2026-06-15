// ============================================================
// src/utils/dateRange.js - 日期工具函数
// ============================================================
// 这个文件提供日期相关的工具函数。
//
// 【JavaScript 知识点】
// - Date 对象: JavaScript 内置的日期处理类
// - getFullYear(): 获取年份（4 位数）
// - getMonth(): 获取月份（0-11，需要 +1）
// - getDate(): 获取日期（1-31）
// - padStart(2, '0'): 字符串填充，确保 2 位数
// - 模板字符串: `hello ${name}` 格式化字符串
// - export: 导出函数供其他文件使用
// ============================================================

// formatLocalDate 格式化本地日期为 YYYY-MM-DD 格式。
// 【为什么需要这个？】
// JavaScript 的 Date 对象默认格式不统一，需要手动格式化。
// 例如：new Date() 可能返回 "Mon Jan 15 2024 10:30:00 GMT+0800"
// 我们需要 "2024-01-15" 这样的格式。
//
// 【参数】
// - date: Date 对象
//
// 【返回值】
// string: 格式化后的日期字符串，如 "2024-01-15"
export function formatLocalDate(date) {
  const year = date.getFullYear()                    // 获取年份：2024
  const month = `${date.getMonth() + 1}`.padStart(2, '0')  // 获取月份：01-12
  const day = `${date.getDate()}`.padStart(2, '0')          // 获取日期：01-31

  // 拼接为 YYYY-MM-DD 格式
  return `${year}-${month}-${day}`
}

// getDefaultDateRange 获取默认日期范围（最近一年）。
// 【功能说明】
// 所有需要日期区间的模块统一默认"最近一年"，减少重复手动选择。
//
// 【返回值】
// object: {start: "2023-01-15", end: "2024-01-15"}
export function getDefaultDateRange() {
  const today = new Date()          // 当前日期
  const oneYearAgo = new Date(today) // 复制当前日期
  oneYearAgo.setFullYear(oneYearAgo.getFullYear() - 1)  // 减去一年

  return {
    start: formatLocalDate(oneYearAgo),  // 一年前的日期
    end: formatLocalDate(today)          // 今天的日期
  }
}
