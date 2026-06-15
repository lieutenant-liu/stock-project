// ============================================================
// src/components/OverviewSection.jsx - 总览面板组件
// ============================================================
// 这是应用的首页组件，提供模块导航和快捷入口。
//
// 【React 知识点】
// - 函数组件: 使用函数定义的 React 组件
// - Props: 父组件传递给子组件的数据
// - 事件处理: onClick 绑定点击事件
// - 箭头函数: () => {} 用于内联函数
//
// 【组件职责】
// 1. 展示系统概览信息
// 2. 提供快捷导航按钮
// 3. 点击按钮跳转到对应模块
// ============================================================

// OverviewSection 总览面板组件。
// 【Props】
// - onJump: 跳转函数，接收模块 ID 作为参数
//   例如：onJump('pipeline') 跳转到数据管线模块
function OverviewSection({ onJump }) {
  // 总览卡片作为导航中枢，快速跳转到高频操作模块。
  return (
    <section className="workspace-card">
      {/* 模块标题 */}
      <h2 className="workspace-card-title">模块总览</h2>

      {/* 概览网格：展示 4 个核心模块 */}
      <div className="overview-grid">
        <div className="overview-item">
          <p className="overview-label">数据配置与同步</p>
          <p className="overview-value">管线</p>
        </div>
        <div className="overview-item">
          <p className="overview-label">自动更新与日志</p>
          <p className="overview-value">调度</p>
        </div>
        <div className="overview-item">
          <p className="overview-label">交易信号筛选</p>
          <p className="overview-value">策略</p>
        </div>
        <div className="overview-item">
          <p className="overview-label">持仓管理与风险</p>
          <p className="overview-value">风控</p>
        </div>
      </div>

      {/* 快捷操作按钮 */}
      <div className="overview-actions">
        {/* 【事件处理】 */}
        {/* onClick={() => onJump('pipeline')} */}
        {/* 使用箭头函数包裹，避免立即执行 */}
        {/* 点击时调用 onJump 函数，传入模块 ID */}
        <button className="overview-btn" onClick={() => onJump('pipeline')}>进入数据管线</button>
        <button className="overview-btn" onClick={() => onJump('automation')}>查看自动任务</button>
        <button className="overview-btn" onClick={() => onJump('strategy')}>开始策略扫描</button>
        <button className="overview-btn" onClick={() => onJump('position')}>管理持仓风控</button>
      </div>
    </section>
  )
}

// 导出组件，供其他文件导入使用
export default OverviewSection
