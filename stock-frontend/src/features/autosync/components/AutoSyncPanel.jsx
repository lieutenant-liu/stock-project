import useAutoSyncController from '../hooks/useAutoSyncController'
import AutoSyncConfigSection from './AutoSyncConfigSection'
import AutoSyncEmailConfigSection from './AutoSyncEmailConfigSection'
import AutoSyncRecipientsSection from './AutoSyncRecipientsSection'
import AutoSyncRunsSection from './AutoSyncRunsSection'

function AutoSyncPanel() {
  const {
    loading,
    running,
    msg,
    config,
    emailCfg,
    recipients,
    recipientForm,
    runs,
    selectedRunId,
    runSteps,
    setConfig,
    setEmailCfg,
    setRecipientForm,
    loadData,
    loadRunSteps,
    handleSave,
    handleSaveEmailConfig,
    handleAddRecipient,
    handleToggleRecipient,
    handleDeleteRecipient,
    handleRunNow,
    statusColor,
    renderDualTime,
  } = useAutoSyncController()

  return (
    <div style={{ border: '1px solid #444', borderRadius: '10px', padding: '20px', maxWidth: '1100px', margin: '0 auto 30px auto', backgroundColor: '#182028' }}>
      <h2 style={{ marginTop: 0, color: '#81d4fa' }}>⏱️ 自动更新任务控制台</h2>
      <p style={{ color: '#9fb3c8', marginTop: 0 }}>
        适用于 Termux 长驻场景：支持每日定时、网络重试、运行记录追踪。
      </p>

      <AutoSyncConfigSection
        config={config}
        setConfig={setConfig}
        loading={loading}
        running={running}
        onSave={handleSave}
        onRunNow={handleRunNow}
        onRefresh={loadData}
      />

      <AutoSyncEmailConfigSection
        emailCfg={emailCfg}
        setEmailCfg={setEmailCfg}
        loading={loading}
        onSave={handleSaveEmailConfig}
      />

      <AutoSyncRecipientsSection
        recipientForm={recipientForm}
        setRecipientForm={setRecipientForm}
        recipients={recipients}
        loading={loading}
        onAdd={handleAddRecipient}
        onToggle={handleToggleRecipient}
        onDelete={handleDeleteRecipient}
      />

      {msg && <p style={{ color: '#ffd54f', marginTop: 0 }}>{msg}</p>}

      <AutoSyncRunsSection
        runs={runs}
        loading={loading}
        onLoadRunSteps={loadRunSteps}
        statusColor={statusColor}
        renderDualTime={renderDualTime}
        selectedRunId={selectedRunId}
        runSteps={runSteps}
      />
    </div>
  )
}

export default AutoSyncPanel
