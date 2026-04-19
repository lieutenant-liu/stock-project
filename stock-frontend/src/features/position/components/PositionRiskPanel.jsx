import PositionFormSection from './PositionFormSection'
import PositionListSection from './PositionListSection'
import PositionRiskReportsSection from './PositionRiskReportsSection'

function PositionRiskPanel({
  posForm,
  setPosForm,
  handleAddPosition,
  runPositionRisk,
  riskLoading,
  riskMsg,
  deployedPositions,
  riskReports,
  handleDeletePosition,
  getActionColor,
}) {
  return (
    <div style={{ border: '2px solid #e01f54', borderRadius: '10px', padding: '20px', maxWidth: '1200px', margin: '0 auto 30px auto', backgroundColor: '#180a0a' }}>
      <h2 style={{ marginTop: 0, color: '#e01f54', display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '10px' }}>
        📊 持仓风险评估 (EOD)
      </h2>

      <PositionFormSection
        posForm={posForm}
        setPosForm={setPosForm}
        handleAddPosition={handleAddPosition}
      />

      <PositionListSection
        deployedPositions={deployedPositions}
        handleDeletePosition={handleDeletePosition}
      />

      <PositionRiskReportsSection
        riskLoading={riskLoading}
        riskMsg={riskMsg}
        runPositionRisk={runPositionRisk}
        riskReports={riskReports}
        deployedPositions={deployedPositions}
        getActionColor={getActionColor}
      />
    </div>
  )
}

export default PositionRiskPanel
