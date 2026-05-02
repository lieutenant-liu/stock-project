import { useState } from 'react'
import api from '../../../api/client'

export default function usePermissionTest() {
  const [testing, setTesting] = useState(false)
  const [results, setResults] = useState([])
  const [error, setError] = useState('')
  const [summary, setSummary] = useState({ total: 0, successCount: 0 })

  const runPermissionTest = async () => {
    setTesting(true)
    setError('')
    setResults([])
    setSummary({ total: 0, successCount: 0 })
    try {
      const resp = await api.testTokenPermissions()
      if (resp.code === 200) {
        setResults(resp.data || [])
        setSummary({ total: resp.total || 0, successCount: resp.success_count || 0 })
      } else {
        setError(resp.msg || '权限测试失败')
      }
    } catch {
      setError('网络请求失败，请检查后端服务')
    } finally {
      setTesting(false)
    }
  }

  return {
    testing,
    results,
    error,
    summary,
    runPermissionTest,
  }
}
