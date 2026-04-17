function StrategyPagination({ currentPage, totalPages, onPrev, onNext }) {
  return (
    <div style={{ margin: '20px 0', color: '#fff', fontSize: '1.1rem', display: 'flex', justifyContent: 'center', alignItems: 'center' }}>
      <button
        disabled={currentPage === 1}
        onClick={onPrev}
        style={{
          padding: '8px 20px',
          marginRight: '15px',
          cursor: currentPage === 1 ? 'not-allowed' : 'pointer',
          backgroundColor: currentPage === 1 ? '#333' : '#1890ff',
          color: '#fff',
          border: 'none',
          borderRadius: '5px',
          fontWeight: 'bold',
        }}
      >
        ⬅️ 上一页
      </button>
      <span> 第 <b style={{ color: '#ffeb3b' }}>{currentPage}</b> / {totalPages} 页 </span>
      <button
        disabled={currentPage === totalPages}
        onClick={onNext}
        style={{
          padding: '8px 20px',
          marginLeft: '15px',
          cursor: currentPage === totalPages ? 'not-allowed' : 'pointer',
          backgroundColor: currentPage === totalPages ? '#333' : '#1890ff',
          color: '#fff',
          border: 'none',
          borderRadius: '5px',
          fontWeight: 'bold',
        }}
      >
        下一页 ➡️
      </button>
    </div>
  )
}

export default StrategyPagination
