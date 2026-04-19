function OverviewSection({ onJump }) {
  // 总览卡片作为导航中枢，快速跳转到高频操作模块。
  return (
    <section className="workspace-card">
      <h2 className="workspace-card-title">模块总览</h2>
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
      <div className="overview-actions">
        <button className="overview-btn" onClick={() => onJump('pipeline')}>进入数据管线</button>
        <button className="overview-btn" onClick={() => onJump('automation')}>查看自动任务</button>
        <button className="overview-btn" onClick={() => onJump('strategy')}>开始策略扫描</button>
        <button className="overview-btn" onClick={() => onJump('position')}>管理持仓风控</button>
      </div>
    </section>
  )
}

export default OverviewSection
