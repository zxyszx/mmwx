// @ts-nocheck
import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createFileRoute, redirect } from '@tanstack/react-router'
import { Ban, RefreshCw, ShieldCheck } from 'lucide-react'
import { toast } from 'sonner'
import { api } from '@/lib/api'
import { handleServerError } from '@/lib/handle-server-error'
import { profileQueryFn } from '@/lib/profile'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

export const Route = createFileRoute('/logs')({
  beforeLoad: async ({ context }) => {
    let profile
    try {
      profile = await context.queryClient.fetchQuery({
        queryKey: ['profile'],
        queryFn: profileQueryFn,
      })
    } catch {
      throw redirect({ to: '/login' })
    }
    if (!profile?.is_admin) throw redirect({ to: '/' })
  },
  component: LogsPage,
})

function LogsPage() {
  return (
    <div className='bg-background min-h-svh'>
      <main className='w-full px-4 pt-[72px] pb-8 sm:px-6 lg:px-8 xl:px-10'>
        <div className='mb-5'>
          <h1 className='text-xl font-semibold text-[var(--text-primary)]'>
            日志
          </h1>
          <p className='mt-1 text-sm text-[var(--text-secondary)]'>
            查看安全事件、操作记录和后台任务执行结果。
          </p>
        </div>
        <Tabs defaultValue='security'>
          <TabsList className='log-tabs h-10 rounded-none border-b border-[var(--divider)] bg-transparent p-0'>
            <TabsTrigger value='security'>安全日志</TabsTrigger>
            <TabsTrigger value='operations'>操作日志</TabsTrigger>
            <TabsTrigger value='tasks'>任务日志</TabsTrigger>
          </TabsList>
          <TabsContent value='security' className='mt-4'>
            <SecurityPanel />
          </TabsContent>
          <TabsContent value='operations' className='mt-4'>
            <OperationPanel />
          </TabsContent>
          <TabsContent value='tasks' className='mt-4'>
            <TaskPanel />
          </TabsContent>
        </Tabs>
      </main>
    </div>
  )
}

function SecurityPanel() {
  const client = useQueryClient()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [ip, setIP] = useState('')
  const [banType, setBanType] = useState('temporary')
  const bans = useQuery({
    queryKey: ['security-bans'],
    queryFn: async () =>
      (await api.get('/api/admin/security/bans')).data.bans ?? [],
    refetchInterval: 15000,
  })
  const events = useQuery({
    queryKey: ['security-events'],
    queryFn: async () =>
      (await api.get('/api/admin/security/events?limit=200')).data.events ?? [],
    refetchInterval: 15000,
  })
  const refresh = () =>
    client
      .invalidateQueries({ queryKey: ['security-bans'] })
      .then(() => client.invalidateQueries({ queryKey: ['security-events'] }))
  const ban = useMutation({
    mutationFn: async () =>
      api.post('/api/admin/security/bans', {
        ip,
        permanent: banType === 'permanent',
      }),
    onSuccess: () => {
      setIP('')
      setDialogOpen(false)
      toast.success('IP 已封禁')
      refresh()
    },
    onError: handleServerError,
  })
  const unban = useMutation({
    mutationFn: async (value) =>
      api.delete(`/api/admin/security/bans/${encodeURIComponent(value)}`),
    onSuccess: () => {
      toast.success('IP 已解封')
      refresh()
    },
    onError: handleServerError,
  })

  return (
    <div className='space-y-4'>
      <Card>
        <CardHeader className='flex-row items-center justify-between border-b border-[var(--divider)]'>
          <CardTitle>
            当前封禁{' '}
            <span className='ml-1 text-sm font-normal text-[var(--text-secondary)]'>
              {bans.data?.length ?? 0}
            </span>
          </CardTitle>
          <div className='flex items-center gap-2'>
            <Button
              size='icon'
              variant='ghost'
              aria-label='刷新安全日志'
              onClick={refresh}
            >
              <RefreshCw
                className={`size-4 ${bans.isFetching || events.isFetching ? 'animate-spin' : ''}`}
              />
            </Button>
            <Button size='sm' onClick={() => setDialogOpen(true)}>
              <Ban className='size-4' />
              封禁 IP
            </Button>
          </div>
        </CardHeader>
        <CardContent className='px-0 pb-0'>
          <LogTable
            empty='暂无活动封禁'
            headers={['IP', '原因', '到期时间', '操作者', '操作']}
            rows={(bans.data ?? []).map((item) => [
              <span className='font-mono'>{item.ip}</span>,
              item.reason || '-',
              item.permanent ? (
                <Badge variant='secondary'>永久</Badge>
              ) : (
                formatTime(item.expires_at)
              ),
              item.actor || '-',
              <Button
                size='sm'
                variant='outline'
                disabled={unban.isPending}
                onClick={() => unban.mutate(item.ip)}
              >
                <ShieldCheck className='size-4' />
                解封
              </Button>,
            ])}
          />
        </CardContent>
      </Card>
      <Card>
        <CardHeader className='border-b border-[var(--divider)]'>
          <CardTitle>安全事件</CardTitle>
        </CardHeader>
        <CardContent className='px-0 pb-0'>
          <LogTable
            empty='暂无安全事件'
            headers={['时间', '类型', 'IP', '路径 / 详情']}
            rows={(events.data ?? []).map((item) => [
              formatTime(item.at),
              <Badge variant='outline'>{item.kind}</Badge>,
              <span className='font-mono'>{item.ip}</span>,
              item.path || item.detail || '-',
            ])}
          />
        </CardContent>
      </Card>
      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>封禁 IP</DialogTitle>
            <DialogDescription>
              阻止指定地址继续访问后台服务。
            </DialogDescription>
          </DialogHeader>
          <div className='space-y-2'>
            <Label htmlFor='ban-ip'>IP 地址</Label>
            <Input
              id='ban-ip'
              value={ip}
              onChange={(event) => setIP(event.target.value)}
              placeholder='IPv4 或 IPv6'
            />
          </div>
          <div className='space-y-2'>
            <Label>封禁类型</Label>
            <Select value={banType} onValueChange={setBanType}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value='temporary'>临时封禁</SelectItem>
                <SelectItem value='permanent'>永久封禁</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <DialogFooter>
            <Button variant='outline' onClick={() => setDialogOpen(false)}>
              取消
            </Button>
            <Button
              variant={banType === 'permanent' ? 'destructive' : 'default'}
              disabled={!ip.trim() || ban.isPending}
              onClick={() => ban.mutate()}
            >
              确认封禁
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}

function TaskPanel() {
  const runs = useQuery({
    queryKey: ['task-runs'],
    queryFn: async () =>
      (await api.get('/api/admin/tasks/runs?limit=200')).data.runs ?? [],
    refetchInterval: 15000,
  })
  return (
    <Card>
      <CardHeader className='flex-row items-center justify-between border-b border-[var(--divider)]'>
        <CardTitle>任务执行记录</CardTitle>
        <Button
          size='icon'
          variant='ghost'
          aria-label='刷新任务日志'
          onClick={() => runs.refetch()}
        >
          <RefreshCw
            className={`size-4 ${runs.isFetching ? 'animate-spin' : ''}`}
          />
        </Button>
      </CardHeader>
      <CardContent className='px-0 pb-0'>
        <LogTable
          empty='暂无任务记录'
          headers={['开始时间', '任务', '状态', '耗时', '详情']}
          rows={(runs.data ?? []).map((item) => [
            formatTime(item.started_at),
            item.task_name,
            <Badge
              variant={item.status === 'error' ? 'destructive' : 'outline'}
            >
              {item.status}
            </Badge>,
            `${item.duration_ms} ms`,
            item.detail || '-',
          ])}
        />
      </CardContent>
    </Card>
  )
}

function OperationPanel() {
  const logs = useQuery({
    queryKey: ['operation-logs'],
    queryFn: async () =>
      (await api.get('/api/admin/operations?limit=200')).data.logs ?? [],
    refetchInterval: 15000,
  })
  return (
    <Card>
      <CardHeader className='flex-row items-center justify-between border-b border-[var(--divider)]'>
        <CardTitle>管理员操作记录</CardTitle>
        <Button
          size='icon'
          variant='ghost'
          aria-label='刷新操作日志'
          onClick={() => logs.refetch()}
        >
          <RefreshCw
            className={`size-4 ${logs.isFetching ? 'animate-spin' : ''}`}
          />
        </Button>
      </CardHeader>
      <CardContent className='px-0 pb-0'>
        <LogTable
          empty='暂无操作记录'
          headers={['时间', '操作者', '操作', '路径', '状态', '来源 IP']}
          rows={(logs.data ?? []).map((item) => [
            formatTime(item.at),
            item.actor || '-',
            <Badge variant='outline'>{item.method}</Badge>,
            <span className='font-mono'>{item.path}</span>,
            item.status,
            <span className='font-mono'>{item.ip}</span>,
          ])}
        />
      </CardContent>
    </Card>
  )
}

function LogTable({ headers, rows, empty }) {
  if (!rows.length)
    return (
      <div className='flex min-h-32 items-center justify-center text-sm text-[var(--text-secondary)]'>
        {empty}
      </div>
    )
  return (
    <div className='overflow-x-auto'>
      <table className='w-full min-w-[760px] text-sm'>
        <thead>
          <tr className='border-b border-[var(--divider)] text-left text-[var(--text-secondary)]'>
            {headers.map((header) => (
              <th key={header} className='px-5 py-3 font-medium'>
                {header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row, rowIndex) => (
            <tr
              key={rowIndex}
              className='border-b border-[var(--divider)] transition-colors last:border-0 hover:bg-[var(--bg-hover)]'
            >
              {row.map((cell, cellIndex) => (
                <td key={cellIndex} className='px-5 py-3.5'>
                  {cell}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function formatTime(value) {
  return value ? new Date(value).toLocaleString() : '-'
}
