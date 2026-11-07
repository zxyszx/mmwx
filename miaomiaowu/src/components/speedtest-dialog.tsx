// @ts-nocheck
import { useEffect, useMemo, useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Gauge, Loader2, History, ArrowLeft, RefreshCw, Settings2, Plus, Trash2, Copy, ExternalLink, Zap } from 'lucide-react'
import { api } from '@/lib/api'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Checkbox } from '@/components/ui/checkbox'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from '@/components/ui/dialog'

const PROTOCOL_COLORS: Record<string, string> = {
  vmess: 'bg-blue-500/10 text-blue-700 dark:text-blue-400',
  vless: 'bg-purple-500/10 text-purple-700 dark:text-purple-400',
  trojan: 'bg-red-500/10 text-red-700 dark:text-red-400',
  ss: 'bg-green-500/10 text-green-700 dark:text-green-400',
  shadowsocks: 'bg-green-500/10 text-green-700 dark:text-green-400',
  hysteria2: 'bg-indigo-500/10 text-indigo-700 dark:text-indigo-400',
  hysteria: 'bg-indigo-500/10 text-indigo-700 dark:text-indigo-400',
  tuic: 'bg-cyan-500/10 text-cyan-700 dark:text-cyan-400',
  snell: 'bg-lime-500/10 text-lime-700 dark:text-lime-400',
}

function relTime(t: string) {
  const ms = Date.now() - new Date(t).getTime()
  const s = Math.floor(ms / 1000)
  if (s < 60) return '刚刚'
  const m = Math.floor(s / 60)
  if (m < 60) return `${m}分钟前`
  const h = Math.floor(m / 60)
  if (h < 24) return `${h}小时前`
  return `${Math.floor(h / 24)}天前`
}

// 前端 running → timeout 阈值:超过 15s 还在 running 视为卡住,允许重测
const RUNNING_TIMEOUT_MS = 15_000
function isStaleRunning(r: any): boolean {
  if (r?.status !== 'running') return false
  const started = r?.created_at ? new Date(r.created_at).getTime() : 0
  if (!started) return false
  return Date.now() - started > RUNNING_TIMEOUT_MS
}

function useLatestSpeedResults(enabled: boolean) {
  return useQuery({
    queryKey: ['speedtest-latest'],
    queryFn: async () => {
      const res = await api.get('/api/admin/speedtest/results?latest=1')
      const map: Record<number, any> = {}
      for (const r of res.data?.results || []) map[r.node_id] = r
      return map
    },
    enabled,
    refetchInterval: (q) =>
      Object.values(q.state.data || {}).some((r: any) => r?.status === 'running') ? 1500 : false,
  })
}

function SpeedCell({ r }: { r: any }) {
  if (!r) return <span className='text-muted-foreground text-xs'>—</span>
  if (r.status === 'running') {
    if (isStaleRunning(r)) {
      return <span className='text-orange-600 dark:text-orange-400 whitespace-nowrap text-xs' title='15 秒未返回结果,点击重测'>超时</span>
    }
    return (
      <span className='text-muted-foreground inline-flex items-center gap-1 whitespace-nowrap text-xs'>
        <Loader2 className='h-3 w-3 animate-spin' />测速中
      </span>
    )
  }
  if (r.status === 'failed')
    return <span className='text-red-600 dark:text-red-400 whitespace-nowrap text-xs' title={r.error}>失败</span>
  return (
    <span className='text-emerald-600 dark:text-emerald-400 font-mono whitespace-nowrap text-xs'>
      ↓ {Number(r.down_mbps).toFixed(1)} Mbps
    </span>
  )
}

// 延迟单元格:可点击。无数据→Zap+"测延迟";测速中→spinner;超 15s→橙色"超时"可重测;已测→Zap+ms;失败→红 Zap+"失败"
function LatencyCell({ r, onProbe, busy }: { r: any; onProbe: () => void; busy: boolean }) {
  const running = (r?.status === 'running' && !isStaleRunning(r)) || busy
  if (running) {
    return (
      <button className='inline-flex items-center gap-1 text-muted-foreground text-xs font-mono' disabled>
        <Loader2 className='h-3 w-3 animate-spin' />
      </button>
    )
  }
  if (r?.status === 'running' && isStaleRunning(r)) {
    return (
      <button
        type='button'
        onClick={onProbe}
        title='15 秒未返回结果,点击重测'
        className='inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-xs text-orange-600 dark:text-orange-400 hover:bg-orange-500/10'
      >
        <Zap className='h-3 w-3' />超时
      </button>
    )
  }
  if (r?.status === 'failed') {
    return (
      <button
        type='button'
        onClick={onProbe}
        title={'点击重测延迟' + (r?.error ? `: ${r.error}` : '')}
        className='inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-xs text-red-600 dark:text-red-400 hover:bg-red-500/10'
      >
        <Zap className='h-3 w-3' />失败
      </button>
    )
  }
  if (r?.status === 'ok' && typeof r?.latency_ms === 'number' && r.latency_ms >= 0) {
    return (
      <button
        type='button'
        onClick={onProbe}
        title='点击重测延迟'
        className='inline-flex items-center gap-1 rounded px-1.5 py-0.5 font-mono text-xs hover:bg-muted'
      >
        <Zap className='h-3 w-3 text-amber-500' />
        {r.latency_ms} ms
      </button>
    )
  }
  return (
    <button
      type='button'
      onClick={onProbe}
      title='只测真连接延迟(Cloudflare 204 多采样)'
      className='inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-xs text-amber-600 hover:bg-amber-500/10'
    >
      <Zap className='h-3 w-3' />测延迟
    </button>
  )
}

function EgressIPCell({ r }: { r: any }) {
  if (!r || !r.egress_ip) return <span className='text-muted-foreground text-xs'>—</span>
  return <span className='font-mono whitespace-nowrap text-xs'>{r.egress_ip}</span>
}

export function SpeedTestDialog({
  open, onMinimize, onClose, nodes,
}: {
  open: boolean
  onMinimize: () => void
  onClose: () => void
  nodes: any[]
}) {
  const queryClient = useQueryClient()

  const [source, setSource] = useState<number | 'master'>(() => {
    const cached = localStorage.getItem('mmw-speedtest-source')
    if (cached && cached !== 'master') {
      const num = Number(cached)
      if (!isNaN(num)) return num
    }
    return 'master'
  })
  const [threads, setThreads] = useState<1 | 8>(() => {
    return localStorage.getItem('mmw-speedtest-threads') === '8' ? 8 : 1
  })
  const [selected, setSelected] = useState<Set<number>>(new Set())
  const [historyNode, setHistoryNode] = useState<{ id: number; name: string } | null>(null)
  const [manageTesters, setManageTesters] = useState(false)
  // 点离线测速端 → 进 TestersView 自动重发安装命令的 tester id
  const [autoRotateTesterId, setAutoRotateTesterId] = useState<number | null>(null)

  const { data: testersData } = useQuery({
    queryKey: ['speed-testers'],
    queryFn: async () => (await api.get('/api/admin/speedtest/testers')).data as { testers: any[] },
    enabled: open,
    staleTime: 10000,
  })
  const testers = testersData?.testers || []

  useEffect(() => { localStorage.setItem('mmw-speedtest-source', String(source)) }, [source])
  useEffect(() => { localStorage.setItem('mmw-speedtest-threads', String(threads)) }, [threads])
  useEffect(() => {
    if (source !== 'master' && testers.length > 0 && !testers.some((t: any) => t.id === source)) {
      setSource('master')
    }
  }, [testers, source])

  const { data: latestMap } = useLatestSpeedResults(open)

  const rows = useMemo(() => {
    return (nodes || []).map((n: any) => {
      let server = '', port = 0
      try {
        const c = JSON.parse(n.clash_config || '{}')
        server = c.server || ''
        port = Number(c.port) || 0
      } catch { /* ignore */ }
      return { id: n.id, name: n.node_name, protocol: (n.protocol || '').toLowerCase(), server, port }
    })
  }, [nodes])

  const runTest = async (nodeIds: number[], latencyOnly = false) => {
    if (nodeIds.length === 0) return
    try {
      const body: any = { threads }
      if (source !== 'master') body.tester_id = source
      if (latencyOnly) body.latency_only = true
      await Promise.all(nodeIds.map((id) => api.post('/api/admin/speedtest/run', { ...body, node_id: id })))
      if (!latencyOnly) {
        toast.success(nodeIds.length === 1 ? `已开始测速: ${rows.find((r) => r.id === nodeIds[0])?.name || ''}` : `已开始批量测速 (${nodeIds.length} 个节点)`)
      }
      queryClient.invalidateQueries({ queryKey: ['speedtest-latest'] })
    } catch (e: any) {
      toast.error(e?.response?.data?.error || '发起测速失败')
    }
  }

  const allSelected = rows.length > 0 && selected.size === rows.length
  const toggleAll = () => setSelected(allSelected ? new Set() : new Set(rows.map((r) => r.id)))
  const toggleOne = (id: number) =>
    setSelected((prev) => {
      const next = new Set(prev)
      next.has(id) ? next.delete(id) : next.add(id)
      return next
    })

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) { setManageTesters(false); setHistoryNode(null); setAutoRotateTesterId(null); onClose() } }}>
      <DialogContent
        className='w-[95vw] sm:w-auto sm:!max-w-[95vw] max-h-[88vh] flex flex-col'
        // 点外面 / 按 Esc:在子视图里只退回主视图,不要最小化整个 dialog
        onInteractOutside={(e) => {
          e.preventDefault()
          if (manageTesters) { setManageTesters(false); setAutoRotateTesterId(null); return }
          if (historyNode) { setHistoryNode(null); return }
          onMinimize()
        }}
        onEscapeKeyDown={(e) => {
          e.preventDefault()
          if (manageTesters) { setManageTesters(false); setAutoRotateTesterId(null); return }
          if (historyNode) { setHistoryNode(null); return }
          onMinimize()
        }}
      >
        {historyNode ? (
          <HistoryView node={historyNode} onBack={() => setHistoryNode(null)} />
        ) : manageTesters ? (
          <TestersView onBack={() => { setManageTesters(false); setAutoRotateTesterId(null) }} autoRotateId={autoRotateTesterId} />
        ) : (
          <>
            <DialogHeader>
              <DialogTitle>节点测速</DialogTitle>
              <DialogDescription>通过代理下载测速文件，测量节点实际速度与延迟</DialogDescription>
            </DialogHeader>

            <div className='flex flex-wrap items-center gap-2'>
              <span className='text-muted-foreground text-sm'>测速来源:</span>
              <Button size='sm' variant={source === 'master' ? 'default' : 'outline'} onClick={() => setSource('master')}>
                主控本机
              </Button>
              {testers.map((x: any) => (
                <Button
                  key={x.id}
                  size='sm'
                  variant={source === x.id ? 'default' : 'outline'}
                  onClick={() => {
                    if (x.online) {
                      setSource(x.id)
                    } else {
                      // 离线测速端 → 进入管理视图并自动轮换 token、重发安装命令
                      setAutoRotateTesterId(x.id)
                      setManageTesters(true)
                    }
                  }}
                  title={x.online ? '' : '测速端离线,点击重装'}
                  className={x.online ? '' : 'opacity-60'}
                >
                  {x.name}{x.online ? '' : ' (离线)'}
                </Button>
              ))}
              <span className='text-muted-foreground ml-3 text-sm'>线程:</span>
              <Button size='sm' variant={threads === 1 ? 'default' : 'outline'} onClick={() => setThreads(1)}>
                单线程
              </Button>
              <Button size='sm' variant={threads === 8 ? 'default' : 'outline'} onClick={() => setThreads(8)}>
                多线程
              </Button>
              <div className='ml-auto flex items-center gap-2'>
                {selected.size > 0 && (
                  <Button size='sm' onClick={() => runTest(Array.from(selected))}>
                    <Gauge className='mr-1 h-4 w-4' />
                    批量测速 ({selected.size})
                  </Button>
                )}
                <Button size='sm' variant='outline' onClick={() => setManageTesters(true)}>
                  <Settings2 className='mr-1 h-4 w-4' />
                  测速端管理
                </Button>
              </div>
            </div>

            {rows.length === 0 ? (
              <div className='text-muted-foreground rounded border py-10 text-center'>暂无节点</div>
            ) : (
              <>
                {/* 桌面端表格 */}
                <div className='hidden max-h-[60vh] overflow-auto rounded border md:block'>
                  <table className='text-sm w-full'>
                    <thead className='bg-muted/50 text-muted-foreground sticky top-0 text-xs'>
                      <tr>
                        <th className='w-8 p-2'><Checkbox checked={allSelected} onCheckedChange={toggleAll} /></th>
                        <th className='p-2 text-left font-normal'>协议</th>
                        <th className='p-2 text-left font-normal'>节点</th>
                        <th className='p-2 text-left font-normal'>服务器</th>
                        <th className='p-2 text-left font-normal'>速度</th>
                        <th className='p-2 text-left font-normal'>延迟</th>
                        <th className='p-2 text-left font-normal'>出口IP</th>
                        <th className='p-2 text-center font-normal'>操作</th>
                      </tr>
                    </thead>
                    <tbody>
                      {rows.map((r) => {
                        const res = latestMap?.[r.id]
                        const running = res?.status === 'running' && !isStaleRunning(res)
                        return (
                          <tr key={r.id} className='border-t'>
                            <td className='p-2 text-center'>
                              <Checkbox checked={selected.has(r.id)} onCheckedChange={() => toggleOne(r.id)} />
                            </td>
                            <td className='p-2'>
                              <Badge variant='secondary' className={`text-[10px] ${PROTOCOL_COLORS[r.protocol] || ''}`}>
                                {r.protocol.toUpperCase() || '?'}
                              </Badge>
                            </td>
                            <td className='p-2'><div className='max-w-[280px] truncate' title={r.name}>{r.name}</div></td>
                            <td className='text-muted-foreground p-2 font-mono text-xs whitespace-nowrap'>{r.server}:{r.port}</td>
                            <td className='p-2'><SpeedCell r={res} /></td>
                            <td className='p-2'><LatencyCell r={res} onProbe={() => runTest([r.id], true)} busy={running} /></td>
                            <td className='p-2'><EgressIPCell r={res} /></td>
                            <td className='p-2'>
                              <div className='flex items-center justify-center gap-1'>
                                <Button variant='ghost' size='icon' className='size-7 text-muted-foreground hover:text-foreground' title='历史' onClick={() => setHistoryNode({ id: r.id, name: r.name })}>
                                  <History className='size-4' />
                                </Button>
                                <Button variant='ghost' size='icon' className='size-7 text-[#d97757] hover:text-[#c66647]' title='测速' disabled={running} onClick={() => runTest([r.id])}>
                                  {running ? <Loader2 className='size-4 animate-spin' /> : <Gauge className='size-4' />}
                                </Button>
                              </div>
                            </td>
                          </tr>
                        )
                      })}
                    </tbody>
                  </table>
                </div>

                {/* 移动端卡片 */}
                <div className='max-h-[60vh] space-y-2 overflow-auto md:hidden'>
                  {rows.map((r) => {
                    const res = latestMap?.[r.id]
                    const running = res?.status === 'running' && !isStaleRunning(res)
                    return (
                      <div key={r.id} className='rounded-lg border p-3'>
                        <div className='flex items-start gap-2'>
                          <Checkbox className='mt-0.5' checked={selected.has(r.id)} onCheckedChange={() => toggleOne(r.id)} />
                          <div className='min-w-0 flex-1'>
                            <div className='flex items-center gap-2'>
                              <Badge variant='secondary' className={`text-[10px] ${PROTOCOL_COLORS[r.protocol] || ''}`}>
                                {r.protocol.toUpperCase() || '?'}
                              </Badge>
                              <span className='truncate font-medium' title={r.name}>{r.name}</span>
                            </div>
                            <div className='text-muted-foreground mt-1 font-mono text-xs break-all'>{r.server}:{r.port}</div>
                            <div className='mt-2 flex flex-wrap items-center gap-x-4 gap-y-1'>
                              <span className='text-muted-foreground text-[10px]'>速度</span>
                              <SpeedCell r={res} />
                              <span className='text-muted-foreground text-[10px]'>延迟</span>
                              <LatencyCell r={res} onProbe={() => runTest([r.id], true)} busy={running} />
                              <span className='text-muted-foreground text-[10px]'>出口IP</span>
                              <EgressIPCell r={res} />
                            </div>
                          </div>
                          <div className='flex shrink-0 flex-col gap-1'>
                            <Button variant='ghost' size='icon' className='size-7 text-muted-foreground hover:text-foreground' title='历史' onClick={() => setHistoryNode({ id: r.id, name: r.name })}>
                              <History className='size-4' />
                            </Button>
                            <Button variant='ghost' size='icon' className='size-7 text-[#d97757] hover:text-[#c66647]' title='测速' disabled={running} onClick={() => runTest([r.id])}>
                              {running ? <Loader2 className='size-4 animate-spin' /> : <Gauge className='size-4' />}
                            </Button>
                          </div>
                        </div>
                      </div>
                    )
                  })}
                </div>
              </>
            )}
          </>
        )}
      </DialogContent>
    </Dialog>
  )
}

function HistoryView({ node, onBack }: { node: { id: number; name: string }; onBack: () => void }) {
  const { data, isFetching, refetch } = useQuery({
    queryKey: ['speedtest-history', node.id],
    queryFn: async () => (await api.get(`/api/admin/speedtest/results?node_id=${node.id}&limit=100`)).data?.results || [],
    refetchInterval: (q) => (q.state.data || []).some((r: any) => r?.status === 'running') ? 4000 : false,
  })
  const rows = (data || []) as any[]
  return (
    <>
      <DialogHeader>
        <DialogTitle className='flex items-center gap-2'>
          <Button variant='ghost' size='icon' className='size-7' onClick={onBack}><ArrowLeft className='size-4' /></Button>
          {node.name} - 测速历史
        </DialogTitle>
        <DialogDescription>该节点的历史测速记录</DialogDescription>
      </DialogHeader>
      <div className='flex justify-end'>
        <Button variant='ghost' size='sm' onClick={() => refetch()} disabled={isFetching}>
          <RefreshCw className={`h-4 w-4 ${isFetching ? 'animate-spin' : ''}`} />
        </Button>
      </div>
      <div className='max-h-[55vh] overflow-auto'>
        {rows.length === 0 ? (
          <div className='text-muted-foreground py-12 text-center text-sm'>暂无测速记录</div>
        ) : (
          <table className='w-full text-sm'>
            <thead className='text-muted-foreground sticky top-0 bg-background text-xs'>
              <tr className='border-b'>
                <th className='py-2 text-right font-normal'>速度</th>
                <th className='py-2 text-right font-normal'>延迟</th>
                <th className='py-2 text-left font-normal pl-3'>出口IP</th>
                <th className='py-2 text-center font-normal'>来源</th>
                <th className='py-2 text-right font-normal'>时间</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((r) => (
                <tr key={r.id} className='border-b last:border-0'>
                  <td className='py-2 text-right font-mono'>
                    {r.status === 'running' ? (
                      <span className='text-muted-foreground inline-flex items-center gap-1'><Loader2 className='h-3 w-3 animate-spin' />测速中</span>
                    ) : r.status === 'failed' ? (
                      <span className='text-red-600 dark:text-red-400'>失败</span>
                    ) : (
                      <span className='text-emerald-600 dark:text-emerald-400'>↓ {Number(r.down_mbps).toFixed(1)} M</span>
                    )}
                  </td>
                  <td className='py-2 text-right font-mono'>{r.status === 'ok' ? `${r.latency_ms}ms` : '-'}</td>
                  <td className='py-2 pl-3 font-mono text-xs whitespace-nowrap'>{r.egress_ip || <span className='text-muted-foreground'>—</span>}</td>
                  <td className='py-2 text-center'>
                    <Badge variant='outline' className='text-[10px]'>{r.source === 'home_tester' ? '测速端' : '主控'}</Badge>
                  </td>
                  <td className='py-2 text-right text-muted-foreground text-xs whitespace-nowrap' title={new Date(r.created_at).toLocaleString()}>{relTime(r.created_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </>
  )
}

// autoRotateId:打开时自动为该 tester 轮换 token、展示安装命令(从 source selector 点离线测速端时用)
function TestersView({ onBack, autoRotateId }: { onBack: () => void; autoRotateId?: number | null }) {
  const qc = useQueryClient()
  const [name, setName] = useState('')
  const [newCred, setNewCred] = useState<{ token: string; name: string } | null>(null)
  const masterURL = typeof window !== 'undefined' ? window.location.origin : ''

  const { data, isLoading } = useQuery({
    queryKey: ['speed-testers'],
    queryFn: async () => (await api.get('/api/admin/speedtest/testers')).data as { testers: any[] },
    refetchInterval: 5000,
  })
  const createMut = useMutation({
    mutationFn: async () => {
      const finalName = name.trim() || 'mmwx-speedtester'
      const d = (await api.post('/api/admin/speedtest/testers/create', { name: finalName })).data
      return { ...d, name: finalName }
    },
    onSuccess: (d: any) => { setNewCred({ token: d.token, name: d.name }); qc.invalidateQueries({ queryKey: ['speed-testers'] }); toast.success('测速端创建成功') },
    onError: (e: any) => toast.error(e?.response?.data?.error || '创建失败'),
  })
  const revokeMut = useMutation({
    mutationFn: async (id: number) => (await api.post('/api/admin/speedtest/testers/revoke', { id })).data,
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['speed-testers'] }); toast.success('测速端已删除') },
    onError: () => toast.error('删除失败'),
  })
  // 离线测速端轮换 token:库里只存哈希,原 token 不可恢复 → 必须生成新的让用户重跑安装命令
  const rotateMut = useMutation({
    mutationFn: async (tester: { id: number; name: string }) => {
      const d = (await api.post('/api/admin/speedtest/testers/rotate-token', { id: tester.id })).data
      return { token: d.token, name: tester.name }
    },
    onSuccess: (d) => { setNewCred(d); qc.invalidateQueries({ queryKey: ['speed-testers'] }); toast.success('已生成新令牌,请重新部署测速端') },
    onError: (e: any) => toast.error(e?.response?.data?.error || '轮换令牌失败'),
  })

  useEffect(() => {
    if (!autoRotateId) return
    const tester = (data?.testers || []).find((x: any) => x.id === autoRotateId)
    if (tester && !tester.online) {
      rotateMut.mutate({ id: tester.id, name: tester.name || `tester-${tester.id}` })
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [autoRotateId, data?.testers])

  const copy = (s: string) => navigator.clipboard?.writeText(s).then(() => toast.success('已复制'), () => {})
  // tester 名称在创建时就已存到主控库,二进制不需要 -name 参数
  const scriptBaseURL = 'https://raw.githubusercontent.com/MMWOrg/mmwX-plugins/refs/heads/main/speedtest/scripts'
  const linuxCmd = newCred ? `curl -fsSL ${scriptBaseURL}/install.sh | bash -s -- -master ${masterURL} -token ${newCred.token}` : ''
  const windowsCmd = newCred ? `irm ${scriptBaseURL}/install.ps1 -OutFile install.ps1; .\\install.ps1 -Master ${masterURL} -Token ${newCred.token}` : ''

  // 从 source selector 点离线测速端进来:只展示该 tester 的安装命令,隐藏新建表单和列表
  const compactMode = autoRotateId != null

  return (
    <>
      <DialogHeader>
        <DialogTitle className='flex items-center gap-2'>
          <Button variant='ghost' size='icon' className='size-7' onClick={onBack}><ArrowLeft className='size-4' /></Button>
          测速端管理
        </DialogTitle>
        <DialogDescription>配置家用测速端，通过反向 WebSocket 连接到主控进行远程测速</DialogDescription>
      </DialogHeader>
      <div className='flex-1 space-y-4 overflow-y-auto py-2'>
        {!compactMode && (
          <>
            <a
              href='https://github.com/MMWOrg/mmwX-plugins/releases/latest'
              target='_blank'
              rel='noopener noreferrer'
              className='flex items-center gap-1.5 text-xs text-primary hover:underline'
            >
              <ExternalLink className='size-3.5' />
              在此下载测速端程序
            </a>
            <div className='flex items-end gap-2'>
              <div className='flex-1 space-y-1'>
                <Label className='text-xs'>测速端名称</Label>
                <Input value={name} onChange={(e) => setName(e.target.value)} placeholder='mmwx-speedtester' className='text-xs' />
              </div>
              <Button size='sm' onClick={() => createMut.mutate()} disabled={createMut.isPending}>
                <Plus className='mr-1 size-4' />创建
              </Button>
            </div>
          </>
        )}

        {newCred && (
          <div className='border-primary/40 bg-primary/5 space-y-2 rounded-md border p-3'>
            <Label className='text-primary text-xs flex items-center gap-2'>
              Token（仅显示一次）
              <span className='text-muted-foreground font-mono text-[10px]'>{newCred.name}</span>
            </Label>
            <div className='flex gap-2'>
              <Input readOnly value={newCred.token} className='font-mono text-xs' />
              <Button variant='outline' size='icon' className='shrink-0' onClick={() => copy(newCred.token)}><Copy className='h-4 w-4' /></Button>
            </div>
            <Label className='mt-1.5 text-xs'>Linux / macOS 一键运行</Label>
            <div className='flex gap-2'>
              <Input readOnly value={linuxCmd} className='font-mono text-[11px]' />
              <Button variant='outline' size='icon' className='shrink-0' onClick={() => copy(linuxCmd)}><Copy className='h-4 w-4' /></Button>
            </div>
            <Label className='mt-1.5 text-xs'>Windows PowerShell 一键运行</Label>
            <div className='flex gap-2'>
              <Input readOnly value={windowsCmd} className='font-mono text-[11px]' />
              <Button variant='outline' size='icon' className='shrink-0' onClick={() => copy(windowsCmd)}><Copy className='h-4 w-4' /></Button>
            </div>
            <p className='text-muted-foreground text-[11px]'>复制命令到终端执行，自动下载测速端并连接主控</p>
          </div>
        )}

        {!compactMode && <div className='space-y-1.5'>
          <Label className='flex items-center gap-1 text-xs'>已配置测速端{isLoading && <RefreshCw className='size-3 animate-spin' />}</Label>
          {(data?.testers || []).length === 0 ? (
            <p className='text-muted-foreground text-xs'>暂无测速端</p>
          ) : (data?.testers || []).map((x: any) => (
            <div key={x.id} className='flex items-center justify-between rounded-md border px-3 py-1.5 text-xs'>
              <div className='min-w-0'>
                <span className='font-medium'>{x.name || `#${x.id}`}</span>
                <Badge variant={x.online ? 'default' : 'secondary'} className='ml-2 text-[10px]'>{x.online ? '在线' : '离线'}</Badge>
              </div>
              <div className='flex items-center gap-1 shrink-0'>
                {!x.online && (
                  <Button
                    variant='ghost'
                    size='sm'
                    className='h-6 text-primary hover:text-primary'
                    onClick={() => rotateMut.mutate({ id: x.id, name: x.name || `tester-${x.id}` })}
                    disabled={rotateMut.isPending}
                    title='重新生成令牌并展示安装命令(原令牌立即失效)'
                  >
                    <RefreshCw className='size-3.5 mr-1' />重装
                  </Button>
                )}
                <Button variant='ghost' size='sm' className='h-6 text-red-600 hover:text-red-700' onClick={() => revokeMut.mutate(x.id)} disabled={revokeMut.isPending}>
                  <Trash2 className='size-3.5' />
                </Button>
              </div>
            </div>
          ))}
        </div>}
      </div>
    </>
  )
}
