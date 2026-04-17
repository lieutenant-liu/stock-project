import StrategyEmailSection from './StrategyEmailSection'
import StrategyPagination from './StrategyPagination'
import StrategyResultCard from './StrategyResultCard'
import StrategySummarySection from './StrategySummarySection'

function StrategyScanPanel({
  inputCode,
  loading,
  hasScanned,
  stockList,
  selectedStrategy,
  setSelectedStrategy,
  currentPage,
  setCurrentPage,
  pageSize,
  fetchStockData,
  recipients,
  selectedRecipients,
  setSelectedRecipients,
  emailSending,
  emailMessage,
  scanMsg,
  refreshEmailTargets,
  sendScanEmail,
}) {
  const filteredList = selectedStrategy === 'ALL'
    ? stockList
    : stockList.filter((item) => item.strategy_name === selectedStrategy)

  const totalPages = Math.max(1, Math.ceil(filteredList.length / pageSize))
  const currentDisplayList = filteredList.slice((currentPage - 1) * pageSize, currentPage * pageSize)
  const buyCount = filteredList.filter((item) => String(item.signal || '').includes('买入')).length
  const waitCount = filteredList.length - buyCount

  return (
    <div style={{ border: '1px solid #333', borderRadius: '10px', padding: '20px', maxWidth: '1200px', margin: '0 auto', backgroundColor: '#1e1e1e' }}>
      <h2 style={{ marginTop: 0 }}>📡 多策略扫描矩阵</h2>

      <div style={{ marginBottom: '20px', display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '15px' }}>
        <button
          onClick={fetchStockData}
          disabled={loading}
          style={{
            padding: '15px 40px',
            fontSize: '1.3rem',
            color: 'white',
            border: 'none',
            borderRadius: '8px',
            cursor: 'pointer',
            fontWeight: 'bold',
            boxShadow: '0 4px 14px rgba(0,0,0,0.3)',
            backgroundColor: inputCode === '' ? '#d32f2f' : '#1890ff',
          }}
        >
          {loading ? '⚡ 正在筛选本地数据库...' : (inputCode === '' ? '🔥 扫描全市场买点' : '🎯 扫描 [目标代码输入框] 指定标的')}
        </button>
      </div>

      {!loading && hasScanned && stockList.length === 0 && (
        <h3 style={{ color: '#666', marginTop: '40px' }}>📉 当前区间暂无符合策略的股票...</h3>
      )}

      <StrategySummarySection
        stockList={stockList}
        selectedStrategy={selectedStrategy}
        setSelectedStrategy={setSelectedStrategy}
        setCurrentPage={setCurrentPage}
        buyCount={buyCount}
        waitCount={waitCount}
        filteredCount={filteredList.length}
      />

      <StrategyEmailSection
        hasScanned={hasScanned}
        inputCode={inputCode}
        scanMsg={scanMsg}
        recipients={recipients}
        selectedRecipients={selectedRecipients}
        setSelectedRecipients={setSelectedRecipients}
        emailSending={emailSending}
        loading={loading}
        refreshEmailTargets={refreshEmailTargets}
        sendScanEmail={sendScanEmail}
        emailMessage={emailMessage}
      />

      {filteredList.length > 0 && (
        <StrategyPagination
          currentPage={currentPage}
          totalPages={totalPages}
          onPrev={() => setCurrentPage((prev) => prev - 1)}
          onNext={() => setCurrentPage((prev) => prev + 1)}
        />
      )}

      <div style={{ display: 'flex', flexWrap: 'wrap', justifyContent: 'center', gap: '30px', marginTop: '20px' }}>
        {currentDisplayList.map((stock, index) => (
          <StrategyResultCard key={`${stock.code || 'unknown'}-${index}`} stock={stock} />
        ))}
      </div>

      {filteredList.length > 20 && (
        <StrategyPagination
          currentPage={currentPage}
          totalPages={totalPages}
          onPrev={() => setCurrentPage((prev) => prev - 1)}
          onNext={() => setCurrentPage((prev) => prev + 1)}
        />
      )}
    </div>
  )
}

export default StrategyScanPanel
