import useTokenManager from '../hooks/useTokenManager'
import TokenCreateForm from './TokenCreateForm'
import TokenTable from './TokenTable'

function TokenManagerPanel({ onTokenActivated }) {
  const {
    loading,
    tokens,
    msg,
    form,
    setForm,
    loadTokens,
    handleCreate,
    handleActivate,
    handleToggleEnabled,
    handleDelete,
  } = useTokenManager({ onTokenActivated })

  return (
    <div style={{ border: '1px solid #444', borderRadius: '10px', padding: '20px', maxWidth: '1040px', margin: '0 auto 30px auto', backgroundColor: '#15202b' }}>
      <h2 style={{ marginTop: 0, color: '#4fc3f7' }}>🔐 Token 管理中心</h2>
      <p style={{ color: '#9aa4b2', marginTop: 0 }}>
        管理多个 Tushare token，支持启停、切换当前生效 token。
      </p>

      <TokenCreateForm
        form={form}
        setForm={setForm}
        loading={loading}
        onCreate={handleCreate}
        onRefresh={loadTokens}
      />

      {msg && <p style={{ color: '#ffd166', marginTop: 0 }}>{msg}</p>}

      <TokenTable
        tokens={tokens}
        loading={loading}
        onActivate={handleActivate}
        onToggleEnabled={handleToggleEnabled}
        onDelete={handleDelete}
      />
    </div>
  )
}

export default TokenManagerPanel
