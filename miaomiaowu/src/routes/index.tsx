import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Link, redirect } from '@tanstack/react-router'
import {
  Bell,
  CalendarClock,
  CalendarDays,
  ChevronLeft,
  ChevronRight,
  CircleParking,
  Coins,
  MessageCircle,
  RefreshCw,
  Send,
  TriangleAlert,
  UserRoundCheck,
  CircleDashed,
  type LucideIcon,
} from 'lucide-react'
import { useAuthStore } from '@/stores/auth-store'
import { api } from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import { Skeleton } from '@/components/ui/skeleton'

type Summary = {
  total_slots?: number
  active_members?: number
  available_slots?: number
  expiring_7_days?: number
  expiring_30_days?: number
  expired?: number
  monthly_revenue?: number | string
}

type Member = {
  id: number
  space_id: number
  space_name?: string
  slot_label?: string
  expire_date?: string
  amount?: number | string
  status?: string
  contact?: string
  telegram?: string
  contact_type?: string
}

type Space = {
  id: number
  platform_id?: number
  platform?: string
  platform_icon?: string
  icon?: string
  total_slots?: number
  member_count?: number
}

type Renewal = {
  created_at?: string
  amount?: number | string
}

type CalendarItem = {
  date: string
  amount: number
}

type PlatformUsage = {
  key: string
  platform: string
  icon?: string
  used: number
  total: number
}

export const Route = createFileRoute('/')({
  beforeLoad: () => {
    const token = useAuthStore.getState().auth.accessToken
    if (!token) throw redirect({ to: '/login' })
  },
  component: DashboardPage,
})

function DashboardPage() {
  const [membersDialogOpen, setMembersDialogOpen] = useState(false)
  const [selectedMonth, setSelectedMonth] = useState(() => {
    const now = new Date()
    return new Date(now.getFullYear(), now.getMonth(), 1)
  })

  const summaryQuery = useQuery<Summary>({
    queryKey: ['parking-summary'],
    queryFn: async () =>
      (await api.get('/api/admin/parking/summary')).data.summary,
    staleTime: 30 * 1000,
  })
  const membersQuery = useQuery<Member[]>({
    queryKey: ['parking-members'],
    queryFn: async () =>
      (await api.get('/api/admin/parking/members')).data.members ?? [],
    staleTime: 30 * 1000,
  })
  const renewalsQuery = useQuery<Renewal[]>({
    queryKey: ['parking-renewals'],
    queryFn: async () =>
      (await api.get('/api/admin/parking/renewals')).data.renewals ?? [],
    staleTime: 30 * 1000,
  })
  const spacesQuery = useQuery<Space[]>({
    queryKey: ['parking-spaces'],
    queryFn: async () =>
      (await api.get('/api/admin/parking/spaces')).data.spaces ?? [],
    staleTime: 30 * 1000,
  })
  const summary = summaryQuery.data ?? {}
  const members = useMemo(() => membersQuery.data ?? [], [membersQuery.data])
  const renewals = useMemo(() => renewalsQuery.data ?? [], [renewalsQuery.data])
  const spaces = spacesQuery.data ?? []
  const dashboardError =
    summaryQuery.isError ||
    membersQuery.isError ||
    renewalsQuery.isError ||
    spacesQuery.isError

  const sortedMembers = useMemo(
    () =>
      [...members]
        .filter((member) => member.status === 'active')
        .sort(
          (a, b) => sortableDate(a.expire_date) - sortableDate(b.expire_date)
        ),
    [members]
  )
  const platformUsage = useMemo(() => summarizePlatforms(spaces), [spaces])

  const calendarData = useMemo<CalendarItem[]>(() => {
    const start = new Date(
      selectedMonth.getFullYear(),
      selectedMonth.getMonth(),
      1
    )
    const end = new Date(
      selectedMonth.getFullYear(),
      selectedMonth.getMonth() + 1,
      0
    )
    const dayCount =
      Math.floor((startOfDay(end).getTime() - start.getTime()) / 86400000) + 1
    const amountsByDate = new Map<string, number>(
      Array.from({ length: dayCount }, (_, index) => {
        const date = new Date(start)
        date.setDate(start.getDate() + index)
        return [localDateKey(date), 0]
      })
    )

    renewals.forEach((record) => {
      if (!record.created_at) return
      const date = new Date(record.created_at)
      if (Number.isNaN(date.getTime())) return
      const key = localDateKey(date)
      if (amountsByDate.has(key)) {
        amountsByDate.set(
          key,
          (amountsByDate.get(key) ?? 0) + finiteNumber(record.amount)
        )
      }
    })

    return Array.from(amountsByDate, ([date, amount]) => ({
      date,
      amount: Number(amount.toFixed(2)),
    })).sort((a, b) => a.date.localeCompare(b.date))
  }, [renewals, selectedMonth])

  const retryDashboard = () => {
    summaryQuery.refetch()
    membersQuery.refetch()
    renewalsQuery.refetch()
    spacesQuery.refetch()
  }

  return (
    <div className='dashboard-page min-h-svh'>
      <main className='dashboard-main mx-auto w-full px-4 pt-[72px] pb-6 sm:px-6'>
        <section
          aria-label='车位统计'
          className='dashboard-stats grid grid-cols-2 gap-3 min-[601px]:grid-cols-3 min-[901px]:grid-cols-6'
        >
          {summaryQuery.isLoading ? (
            Array.from({ length: 6 }).map((_, index) => (
              <MetricSkeleton key={index} />
            ))
          ) : (
            <>
              <Metric
                title='总车位'
                value={summary.total_slots ?? 0}
                icon={CircleParking}
              />
              <Metric
                title='已占用'
                value={summary.active_members ?? 0}
                icon={UserRoundCheck}
              />
              <Metric
                title='剩余'
                value={summary.available_slots ?? 0}
                icon={CircleDashed}
              />
              <Metric
                title='7 天内到期'
                value={summary.expiring_7_days ?? 0}
                icon={CalendarClock}
              />
              <Metric title='已过期' value={summary.expired ?? 0} icon={Bell} />
              <Metric
                title='本月收入'
                value={`¥ ${finiteNumber(summary.monthly_revenue).toFixed(2)}`}
                icon={Coins}
              />
            </>
          )}
        </section>

        {dashboardError && (
          <div
            role='alert'
            className='mt-3 flex flex-wrap items-center justify-between gap-3 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-500/25 dark:bg-red-500/10 dark:text-red-300'
          >
            <span className='flex items-center gap-2'>
              <TriangleAlert className='size-4' />
              部分信息加载失败，请重新获取。
            </span>
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={retryDashboard}
              disabled={
                summaryQuery.isFetching ||
                membersQuery.isFetching ||
                renewalsQuery.isFetching ||
                spacesQuery.isFetching
              }
            >
              <RefreshCw className='size-4' />
              重试
            </Button>
          </div>
        )}

        <Card className='dashboard-calendar-card mt-3 py-0'>
          <RevenueCalendar
            data={calendarData}
            selectedMonth={selectedMonth}
            onMonthChange={setSelectedMonth}
          />
        </Card>

        <section className='dashboard-upcoming mt-3 grid grid-cols-1 gap-3 md:grid-cols-2'>
          <ExpiringMembers
            members={sortedMembers.slice(0, 4)}
            spaces={spaces}
          />
          <PlatformOverview platforms={platformUsage.slice(0, 4)} />
        </section>

        <Card className='dashboard-members mt-3 py-0'>
          <CardHeader className='dashboard-section-header flex flex-row items-center justify-between px-5 py-1'>
            <CardTitle className='text-sm'>成员</CardTitle>
            <button
              type='button'
              className='dashboard-members-all -mr-2 inline-flex min-h-8 items-center gap-0.5 px-2 text-[11px] text-[var(--text-secondary)] transition-colors hover:text-[var(--text-primary)]'
              onClick={() => setMembersDialogOpen(true)}
            >
              全部
              <ChevronRight className='size-3' />
            </button>
          </CardHeader>
          <CardContent className='px-0 pb-0'>
            {sortedMembers.length ? (
              <MemberGrid members={sortedMembers.slice(0, 6)} spaces={spaces} />
            ) : (
              <Empty text='暂无成员账号' />
            )}
          </CardContent>
        </Card>
      </main>

      <Dialog open={membersDialogOpen} onOpenChange={setMembersDialogOpen}>
        <DialogContent className='dashboard-members-dialog flex max-h-[76vh] w-[min(900px,88vw)] max-w-[min(900px,88vw)] flex-col gap-0 overflow-hidden p-0 sm:!max-w-[min(900px,88vw)]'>
          <DialogHeader className='shrink-0 border-b border-[var(--divider)] px-5 py-4 text-left'>
            <DialogTitle className='text-sm'>全部成员</DialogTitle>
            <DialogDescription className='sr-only'>
              全部在位成员列表
            </DialogDescription>
          </DialogHeader>
          <div className='dashboard-members-dialog-scroll min-h-0 overflow-y-auto'>
            {sortedMembers.length ? (
              <MemberGrid members={sortedMembers} spaces={spaces} />
            ) : (
              <Empty text='暂无成员账号' />
            )}
          </div>
        </DialogContent>
      </Dialog>
    </div>
  )
}

function ExpiringMembers({
  members,
  spaces,
}: {
  members: Member[]
  spaces: Space[]
}) {
  return (
    <Card className='dashboard-upcoming-card gap-0 overflow-hidden py-0'>
      <CardHeader className='dashboard-section-header flex flex-row items-center justify-between px-5 py-2'>
        <div>
          <CardTitle className='text-sm'>即将到期</CardTitle>
          <p className='mt-0.5 text-[10px] text-[var(--text-muted)]'>
            最近需要处理的席位
          </p>
        </div>
        <Link
          to='/parking'
          search={{ tab: 'members' }}
          className='inline-flex min-h-8 items-center gap-0.5 text-[11px] text-[var(--text-secondary)] hover:text-[var(--text-primary)]'
        >
          全部 <ChevronRight className='size-3' />
        </Link>
      </CardHeader>
      <div className='dashboard-upcoming-list'>
        {members.length ? (
          members.map((member) => {
            const space = spaces.find((item) => item.id === member.space_id)
            return (
              <div key={member.id} className='dashboard-upcoming-row'>
                <PlatformIcon platform={space} className='size-5' />
                <div className='min-w-0'>
                  <p className='truncate text-xs font-medium text-[var(--text-primary)]'>
                    {space?.platform || '未分组'} /{' '}
                    {member.slot_label || member.contact || '-'}
                  </p>
                  <p className='truncate text-[10px] text-[var(--text-muted)]'>
                    {formatDate(member.expire_date)} · ¥
                    {finiteNumber(member.amount).toFixed(2)}
                  </p>
                </div>
                <ExpiryMeta date={member.expire_date} showDate={false} />
              </div>
            )
          })
        ) : (
          <CompactEmpty text='暂无待续费成员' />
        )}
      </div>
    </Card>
  )
}

function PlatformOverview({ platforms }: { platforms: PlatformUsage[] }) {
  return (
    <Card className='dashboard-upcoming-card gap-0 overflow-hidden py-0'>
      <CardHeader className='dashboard-section-header flex flex-row items-center justify-between px-5 py-2'>
        <div>
          <CardTitle className='text-sm'>平台概览</CardTitle>
          <p className='mt-0.5 text-[10px] text-[var(--text-muted)]'>
            各平台席位使用情况
          </p>
        </div>
        <Link
          to='/parking'
          search={{ tab: 'platforms' }}
          className='inline-flex min-h-8 items-center gap-0.5 text-[11px] text-[var(--text-secondary)] hover:text-[var(--text-primary)]'
        >
          全部 <ChevronRight className='size-3' />
        </Link>
      </CardHeader>
      <div className='dashboard-upcoming-list'>
        {platforms.length ? (
          platforms.map((platform) => {
            const percentage = Math.min(
              100,
              Math.round((platform.used / Math.max(platform.total, 1)) * 100)
            )
            return (
              <div key={platform.key} className='dashboard-platform-row'>
                <PlatformIcon
                  platform={{
                    id: 0,
                    platform: platform.platform,
                    platform_icon: platform.icon,
                  }}
                  className='size-5'
                />
                <div className='min-w-0'>
                  <div className='flex items-center justify-between gap-3'>
                    <p className='truncate text-xs font-medium text-[var(--text-primary)]'>
                      {platform.platform}
                    </p>
                    <span className='shrink-0 text-[10px] text-[var(--text-secondary)] tabular-nums'>
                      {platform.used} / {platform.total}
                    </span>
                  </div>
                  <div className='dashboard-platform-track mt-1.5 h-1 overflow-hidden rounded-full'>
                    <span
                      className='dashboard-platform-progress block h-full rounded-full'
                      style={{ width: `${percentage}%` }}
                    />
                  </div>
                </div>
                <span className='shrink-0 text-[10px] text-[var(--text-muted)] tabular-nums'>
                  剩 {Math.max(platform.total - platform.used, 0)} 位
                </span>
              </div>
            )
          })
        ) : (
          <CompactEmpty text='暂无平台数据' />
        )}
      </div>
    </Card>
  )
}

function ExpiryMeta({
  date,
  showDate = true,
}: {
  date?: string
  showDate?: boolean
}) {
  const days = daysUntil(date)
  const tone =
    days < 0
      ? 'text-red-400'
      : days <= 7
        ? 'text-[var(--accent-brand)]'
        : 'text-[var(--text-secondary)]'
  return (
    <div className='shrink-0 text-right'>
      <p className={`text-[10px] font-medium ${tone}`}>{expiryLabel(days)}</p>
      {showDate && (
        <p className='text-[10px] text-[var(--text-muted)] tabular-nums'>
          {formatDate(date)}
        </p>
      )}
    </div>
  )
}

function CompactEmpty({ text }: { text: string }) {
  return (
    <div className='flex min-h-[104px] items-center justify-center text-xs text-[var(--text-muted)]'>
      {text}
    </div>
  )
}

function MemberGrid({
  members,
  spaces,
}: {
  members: Member[]
  spaces: Space[]
}) {
  const rows = Array.from(
    { length: Math.ceil(members.length / 2) },
    (_, rowIndex) => members.slice(rowIndex * 2, rowIndex * 2 + 2)
  )

  return (
    <div className='dashboard-member-grid'>
      {rows.map((row, rowIndex) => (
        <div
          key={row.map((member) => member.id).join('-')}
          className={`dashboard-member-grid-row grid grid-cols-1 md:grid-cols-2 ${
            rowIndex === rows.length - 1 ? 'dashboard-member-grid-row-last' : ''
          }`}
        >
          {row.map((member, columnIndex) => {
            const space = spaces.find((item) => item.id === member.space_id)
            return (
              <article
                key={member.id}
                className={`dashboard-member-cell grid min-h-[52px] min-w-0 grid-cols-[auto_minmax(0,1fr)_auto] grid-rows-2 items-center gap-x-2 px-5 py-2 ${columnIndex === 1 ? 'dashboard-member-cell-right' : ''}`}
              >
                <ContactIcon member={member} />
                <div className='flex min-w-0 items-center gap-2 self-end text-xs'>
                  <span className='min-w-0 shrink truncate text-[var(--text-primary)]'>
                    {member.contact || member.telegram || '-'}
                  </span>
                  <span className='h-3 w-px shrink-0 bg-[var(--divider)]' />
                  <PlatformIcon platform={space} className='size-5' />
                  <span className='min-w-0 truncate font-medium text-[var(--text-primary)]'>
                    {space?.platform || '未分组'}
                    {member.slot_label ? ` / ${member.slot_label}` : ''}
                  </span>
                </div>
                <span className='self-end justify-self-end text-xs font-semibold text-[var(--text-primary)] tabular-nums'>
                  ¥{finiteNumber(member.amount).toFixed(2)}
                </span>
                <div className='flex min-w-0 items-center gap-2 self-start text-[10.5px] text-[var(--text-muted)]'>
                  <span className='min-w-0 truncate'>
                    {member.space_name || '-'} ·{' '}
                    {formatDate(member.expire_date)}
                  </span>
                </div>
                <span className='dashboard-member-status inline-flex shrink-0 self-start justify-self-end rounded-full bg-emerald-500/10 px-1.5 py-0.5 text-[10px] font-medium text-emerald-600 dark:text-emerald-400'>
                  在位
                </span>
              </article>
            )
          })}
        </div>
      ))}
    </div>
  )
}

function RevenueCalendar({
  data,
  selectedMonth,
  onMonthChange,
}: {
  data: CalendarItem[]
  selectedMonth: Date
  onMonthChange: (date: Date) => void
}) {
  const today = localDateKey(new Date())
  const [pickerOpen, setPickerOpen] = useState(false)
  const [pickerYear, setPickerYear] = useState(selectedMonth.getFullYear())
  const firstDay = new Date(
    selectedMonth.getFullYear(),
    selectedMonth.getMonth(),
    1
  ).getDay()
  const leadingBlanks = (firstDay + 6) % 7
  const trailingBlanks = (7 - ((leadingBlanks + data.length) % 7)) % 7

  const moveMonth = (offset: number) => {
    onMonthChange(
      new Date(
        selectedMonth.getFullYear(),
        selectedMonth.getMonth() + offset,
        1
      )
    )
  }
  const openPicker = (open: boolean) => {
    setPickerOpen(open)
    if (open) setPickerYear(selectedMonth.getFullYear())
  }
  const selectMonth = (month: number) => {
    onMonthChange(new Date(pickerYear, month, 1))
    setPickerOpen(false)
  }

  return (
    <div className='min-w-0 px-5 py-2.5'>
      <div className='mb-1 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'>
        <h2 className='text-sm leading-tight font-semibold'>本月收入</h2>
        <div className='flex items-center justify-center gap-1 sm:justify-end'>
          <Button
            type='button'
            variant='ghost'
            size='icon'
            className='dashboard-month-arrow size-8 rounded-lg'
            aria-label='上个月'
            title='上个月'
            onClick={() => moveMonth(-1)}
          >
            <ChevronLeft className='size-4' />
          </Button>
          <Popover open={pickerOpen} onOpenChange={openPicker}>
            <PopoverTrigger asChild>
              <Button
                type='button'
                variant='outline'
                className='dashboard-month-picker h-8 min-w-32 px-2.5 text-xs font-semibold shadow-none'
                aria-label='选择年份和月份'
              >
                <CalendarDays className='size-4 text-[var(--text-muted)]' />
                {selectedMonth.getFullYear()} 年 {selectedMonth.getMonth() + 1}{' '}
                月
              </Button>
            </PopoverTrigger>
            <PopoverContent className='w-72 rounded-xl p-3' align='center'>
              <div className='mb-3 flex items-center justify-between'>
                <Button
                  type='button'
                  variant='ghost'
                  size='icon'
                  aria-label='上一年'
                  onClick={() => setPickerYear((year) => year - 1)}
                >
                  <ChevronLeft className='size-4' />
                </Button>
                <strong className='text-sm font-medium'>{pickerYear} 年</strong>
                <Button
                  type='button'
                  variant='ghost'
                  size='icon'
                  aria-label='下一年'
                  onClick={() => setPickerYear((year) => year + 1)}
                >
                  <ChevronRight className='size-4' />
                </Button>
              </div>
              <div className='grid grid-cols-3 gap-2'>
                {Array.from({ length: 12 }, (_, month) => {
                  const selected =
                    pickerYear === selectedMonth.getFullYear() &&
                    month === selectedMonth.getMonth()
                  return (
                    <Button
                      type='button'
                      key={month}
                      size='sm'
                      variant={selected ? 'default' : 'ghost'}
                      aria-pressed={selected}
                      onClick={() => selectMonth(month)}
                    >
                      {month + 1} 月
                    </Button>
                  )
                })}
              </div>
            </PopoverContent>
          </Popover>
          <Button
            type='button'
            variant='ghost'
            size='icon'
            className='dashboard-month-arrow size-8 rounded-lg'
            aria-label='下个月'
            title='下个月'
            onClick={() => moveMonth(1)}
          >
            <ChevronRight className='size-4' />
          </Button>
        </div>
      </div>

      <div className='dashboard-calendar-weekdays mb-1 grid grid-cols-7 text-center text-[10px] font-medium text-[var(--text-muted)]'>
        {['一', '二', '三', '四', '五', '六', '日'].map((day) => (
          <span key={day}>周{day}</span>
        ))}
      </div>
      <div className='dashboard-calendar-grid grid grid-cols-7 gap-x-1 gap-y-1 overflow-hidden rounded-2xl'>
        {Array.from({ length: leadingBlanks }).map((_, index) => (
          <span
            key={`blank-${index}`}
            className='dashboard-calendar-blank h-14 min-[601px]:h-11'
          />
        ))}
        {data.map((item) => {
          const date = new Date(`${item.date}T00:00:00`)
          const hasIncome = item.amount > 0
          const isToday = item.date === today
          return (
            <div
              key={item.date}
              title={`${date.getMonth() + 1}月${date.getDate()}日，收入 ¥${item.amount.toFixed(2)}`}
              className='dashboard-calendar-day flex h-14 min-w-0 flex-col items-center justify-between rounded-xl p-0.5 text-center min-[601px]:h-11 min-[601px]:p-1.5'
            >
              <span
                className={`dashboard-calendar-date inline-flex size-6 items-center justify-center rounded-lg text-xs font-semibold text-[var(--text-primary)] tabular-nums ${isToday ? 'dashboard-calendar-date-today' : ''}`}
              >
                {date.getDate()}
              </span>
              <span
                className={`max-w-full text-[9px] whitespace-nowrap tabular-nums min-[601px]:text-[10px] ${
                  hasIncome
                    ? 'font-medium text-[var(--accent-brand)]'
                    : 'dashboard-calendar-zero'
                }`}
              >
                ¥{item.amount.toFixed(2)}
              </span>
            </div>
          )
        })}
        {Array.from({ length: trailingBlanks }).map((_, index) => (
          <span
            key={`trailing-${index}`}
            className='dashboard-calendar-blank h-14 min-[601px]:h-11'
          />
        ))}
      </div>
    </div>
  )
}

function Metric({
  title,
  value,
  desc,
  icon: Icon,
}: {
  title: string
  value: string | number
  desc?: string
  icon: LucideIcon
}) {
  return (
    <Card className='dashboard-stat min-h-[84px] justify-center gap-0 py-2'>
      <CardHeader className='flex flex-row items-start justify-between space-y-0 pb-1'>
        <CardTitle className='text-xs font-medium text-[var(--text-secondary)]'>
          {title}
        </CardTitle>
        <Icon
          aria-hidden='true'
          className='dashboard-stat-icon size-4 shrink-0 text-[var(--text-secondary)]'
        />
      </CardHeader>
      <CardContent>
        <div className='dashboard-stat-value truncate text-[26px] leading-none font-semibold text-[var(--text-primary)] tabular-nums'>
          {value}
        </div>
        {desc && (
          <p className='mt-1.5 text-xs text-[var(--text-muted)]'>{desc}</p>
        )}
      </CardContent>
    </Card>
  )
}

function PlatformIcon({
  platform,
  className = 'size-7',
}: {
  platform?: Space
  className?: string
}) {
  const icon = platform?.icon || platform?.platform_icon
  if (icon) {
    return (
      <img
        src={icon}
        alt=''
        className={`${className} shrink-0 rounded-md object-contain p-0.5`}
      />
    )
  }
  const name = platform?.platform || '?'
  return (
    <span
      aria-hidden='true'
      className={`${className} flex shrink-0 items-center justify-center rounded-md bg-[var(--bg-hover)] text-[11px] font-semibold text-[var(--text-secondary)]`}
    >
      {String(name).trim().slice(0, 1).toUpperCase()}
    </span>
  )
}

function ContactIcon({ member }: { member: Member }) {
  const type = member.contact_type || (member.telegram ? 'telegram' : 'wechat')
  const icon =
    type === 'telegram' ? (
      <Send className='size-4 text-[#229ed9]' />
    ) : type === 'nodeseek' ? (
      <span className='flex size-4 items-center justify-center rounded-full bg-slate-800 text-[9px] font-semibold text-white'>
        N
      </span>
    ) : (
      <MessageCircle className='size-4 text-[#07c160]' />
    )
  return (
    <span className='row-span-2 flex items-center justify-center self-stretch'>
      {icon}
    </span>
  )
}

function MetricSkeleton() {
  return (
    <Card className='dashboard-stat min-h-[84px] justify-center gap-0 py-2'>
      <CardHeader>
        <Skeleton className='h-4 w-24' />
      </CardHeader>
      <CardContent>
        <Skeleton className='h-8 w-28' />
        <Skeleton className='mt-3 h-3 w-32' />
      </CardContent>
    </Card>
  )
}

function Empty({ text }: { text: string }) {
  return (
    <div className='m-5 rounded-xl border border-dashed border-[var(--border-subtle)] py-12 text-center text-sm text-[var(--text-muted)]'>
      {text}
    </div>
  )
}

function sortableDate(value?: string) {
  if (!value) return Number.MAX_SAFE_INTEGER
  const timestamp = new Date(value).getTime()
  return Number.isNaN(timestamp) ? Number.MAX_SAFE_INTEGER : timestamp
}

function finiteNumber(value?: number | string) {
  const number = Number(value)
  return Number.isFinite(number) ? number : 0
}

function formatDate(value?: string) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleDateString('zh-CN')
}

function daysUntil(value?: string) {
  if (!value) return Number.MAX_SAFE_INTEGER
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return Number.MAX_SAFE_INTEGER
  return Math.ceil(
    (startOfDay(date).getTime() - startOfDay(new Date()).getTime()) / 86400000
  )
}

function expiryLabel(days: number) {
  if (days === Number.MAX_SAFE_INTEGER) return '未设置'
  if (days < 0) return `已过期 ${Math.abs(days)} 天`
  if (days === 0) return '今天到期'
  return `剩 ${days} 天`
}

function startOfDay(value: Date) {
  return new Date(value.getFullYear(), value.getMonth(), value.getDate())
}

function localDateKey(value: Date) {
  const year = value.getFullYear()
  const month = String(value.getMonth() + 1).padStart(2, '0')
  const day = String(value.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

function summarizePlatforms(spaces: Space[]): PlatformUsage[] {
  const platforms = new Map<string, PlatformUsage>()

  spaces.forEach((space) => {
    const platform = space.platform?.trim() || '未分组'
    const key = String(space.platform_id || platform)
    const current = platforms.get(key) ?? {
      key,
      platform,
      icon: space.platform_icon || space.icon,
      used: 0,
      total: 0,
    }
    current.used += Math.max(finiteNumber(space.member_count), 0)
    current.total += Math.max(finiteNumber(space.total_slots), 0)
    platforms.set(key, current)
  })

  return Array.from(platforms.values()).sort(
    (a, b) => b.used / Math.max(b.total, 1) - a.used / Math.max(a.total, 1)
  )
}
