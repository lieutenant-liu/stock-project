import PipelineConfigSection from './PipelineConfigSection'
import PipelineLogsSection from './PipelineLogsSection'
import PipelineTaskGrid from './PipelineTaskGrid'

function DataPipelinePanel(props) {
  return (
    <>
      <PipelineConfigSection
        inputCode={props.inputCode}
        setInputCode={props.setInputCode}
        syncStart={props.syncStart}
        setSyncStart={props.setSyncStart}
        syncEnd={props.syncEnd}
        setSyncEnd={props.setSyncEnd}
        dataSource={props.dataSource}
        setDataSource={props.setDataSource}
        tushareToken={props.tushareToken}
        setTushareToken={props.setTushareToken}
        requestSpeed={props.requestSpeed}
        setRequestSpeed={props.setRequestSpeed}
        updateToken={props.updateToken}
        updateSpeed={props.updateSpeed}
        triggerSyncCalendar={props.triggerSyncCalendar}
        triggerSyncBasic={props.triggerSyncBasic}
      />

      <PipelineTaskGrid
        dataSource={props.dataSource}
        triggerSyncKline={props.triggerSyncKline}
        triggerSyncFund={props.triggerSyncFund}
        triggerSyncAdj={props.triggerSyncAdj}
        triggerSyncIndex={props.triggerSyncIndex}
        triggerSyncMoneyFlow={props.triggerSyncMoneyFlow}
        triggerSyncFina={props.triggerSyncFina}
        triggerSyncLimit={props.triggerSyncLimit}
        syncMsgKline={props.syncMsgKline}
        syncMsgFund={props.syncMsgFund}
        syncMsgAdj={props.syncMsgAdj}
        syncMsgIndex={props.syncMsgIndex}
        syncMsgMoney={props.syncMsgMoney}
        syncMsgFina={props.syncMsgFina}
        syncMsgLimit={props.syncMsgLimit}
      />

      <PipelineLogsSection sysLogs={props.sysLogs} />
    </>
  )
}

export default DataPipelinePanel
