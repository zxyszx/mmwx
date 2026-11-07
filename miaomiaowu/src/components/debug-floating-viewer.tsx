import { useState, useEffect, useRef, useCallback } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Bug, X, Download } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Sheet, SheetContent, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { Badge } from '@/components/ui/badge'
import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

const AUTO_CLOSE_SECONDS = 5 * 60

function formatElapsed(seconds: number): string {
  if (seconds < 60) return `${seconds}s`
  const min = Math.floor(seconds / 60)
  const sec = seconds % 60
  if (min < 60) return sec === 0 ? `${min}m` : `${min}m${sec}s`
  const hour = Math.floor(min / 60)
  return `${hour}h${min % 60}m`
}

export function DebugFloatingViewer() {
  const [sheetOpen, setSheetOpen] = useState(false)
  const [elapsed, setElapsed] = useState('')
  const logRef = useRef<HTMLPreElement>(null)
  const autoScrollRef = useRef(true)
  const { auth } = useAuthStore()
  const queryClient = useQueryClient()

  const { data: debugStatus } = useQuery({
    queryKey: ['debug-status'],
    queryFn: async () => {
      const response = await api.get('/api/user/debug/status')
      return response.data as {
        enabled: boolean
        started_at?: string
        file_size?: string
        duration?: string
        duration_seconds?: number
      }
    },
    enabled: Boolean(auth.accessToken),
    refetchInterval: (query) => query.state.data?.enabled ? 5000 : false,
  })

  const { data: tailData } = useQuery({
    queryKey: ['debug-tail'],
    queryFn: async () => {
      const response = await api.get('/api/user/debug/tail?lines=200')
      return response.data as { lines: string; total_size: number }
    },
    enabled: Boolean(auth.accessToken) && debugStatus?.enabled === true && sheetOpen,
    refetchInterval: 2000,
  })

  const disableMutation = useMutation({
    mutationFn: async () => {
      const response = await api.post('/api/user/debug/disable')
      return response.data as { download_url: string }
    },
    onSuccess: async (data) => {
      queryClient.invalidateQueries({ queryKey: ['debug-status'] })
      queryClient.invalidateQueries({ queryKey: ['debug-tail'] })
      setSheetOpen(false)

      if (data.download_url) {
        try {
          const response = await api.get(data.download_url, { responseType: 'blob' })
          const url = window.URL.createObjectURL(new Blob([response.data]))
          const link = document.createElement('a')
          link.href = url
          link.setAttribute('download', data.download_url.split('file=')[1] || 'debug.log')
          document.body.appendChild(link)
          link.click()
          link.remove()
          window.URL.revokeObjectURL(url)
        } catch {
          toast.error('下载日志文件失败')
        }
      }
    },
  })

  const handleClose = useCallback(() => {
    disableMutation.mutate()
    toast.success('Debug 日志已关闭')
  }, [disableMutation])

  // 客户端每秒计时 + 5分钟自动关闭
  useEffect(() => {
    if (!debugStatus?.enabled || !debugStatus.started_at) {
      setElapsed('')
      return
    }
    const startTime = new Date(debugStatus.started_at).getTime()
    const tick = () => {
      const seconds = Math.floor((Date.now() - startTime) / 1000)
      setElapsed(formatElapsed(seconds))
      if (seconds >= AUTO_CLOSE_SECONDS) {
        disableMutation.mutate()
        toast.info('Debug 已自动关闭（超过5分钟）')
      }
    }
    tick()
    const timer = setInterval(tick, 1000)
    return () => clearInterval(timer)
  }, [debugStatus?.enabled, debugStatus?.started_at])

  // 自动滚动到底部
  useEffect(() => {
    if (logRef.current && autoScrollRef.current) {
      logRef.current.scrollTop = logRef.current.scrollHeight
    }
  }, [tailData?.lines])

  const handleScroll = () => {
    if (!logRef.current) return
    const { scrollTop, scrollHeight, clientHeight } = logRef.current
    autoScrollRef.current = scrollHeight - scrollTop - clientHeight < 50
  }

  if (!debugStatus?.enabled) return null

  return (
    <>
      {/* 浮动图标 - 右侧垂直居中 */}
      <button
        onClick={() => setSheetOpen(true)}
        className='fixed top-1/2 -translate-y-1/2 right-3 z-50 flex items-center gap-1.5 rounded-full bg-orange-500 px-3 py-2 text-white shadow-lg hover:bg-orange-600 transition-colors cursor-pointer'
        title='查看 Debug 日志'
      >
        <Bug className='size-4 animate-pulse' />
        <Badge variant='secondary' className='bg-white/20 text-white text-xs px-1.5 py-0'>
          {elapsed || '0s'}
        </Badge>
      </button>

      {/* 日志查看器 Sheet */}
      <Sheet open={sheetOpen} onOpenChange={setSheetOpen}>
        <SheetContent side='right' className='w-[600px] sm:max-w-[600px] flex flex-col p-0'>
          <SheetHeader className='px-4 py-3 border-b shrink-0'>
            <div className='flex items-center justify-between'>
              <SheetTitle className='flex items-center gap-2 text-base'>
                <Bug className='size-4 text-orange-500' />
                Debug 日志
                {debugStatus.file_size && (
                  <Badge variant='secondary' className='text-xs'>{debugStatus.file_size}</Badge>
                )}
                {elapsed && (
                  <Badge variant='outline' className='text-xs'>{elapsed}</Badge>
                )}
              </SheetTitle>
              <div className='flex items-center gap-1'>
                <Button
                  variant='destructive'
                  size='sm'
                  className='h-7 text-xs'
                  onClick={handleClose}
                  disabled={disableMutation.isPending}
                >
                  <Download className='size-3 mr-1' />
                  关闭并下载
                </Button>
                <Button variant='ghost' size='icon' className='size-7' onClick={() => setSheetOpen(false)}>
                  <X className='size-4' />
                </Button>
              </div>
            </div>
          </SheetHeader>
          <pre
            ref={logRef}
            onScroll={handleScroll}
            className='flex-1 overflow-auto p-4 text-xs font-mono leading-relaxed bg-muted/30 whitespace-pre-wrap break-all'
          >
            {tailData?.lines || '等待日志...'}
          </pre>
        </SheetContent>
      </Sheet>
    </>
  )
}
