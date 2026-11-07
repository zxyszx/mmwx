// @ts-nocheck
import { useDeferredValue, useEffect, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createFileRoute, redirect, useNavigate } from '@tanstack/react-router'
import {
  closestCenter,
  DndContext,
  PointerSensor,
  TouchSensor,
  useSensor,
  useSensors,
} from '@dnd-kit/core'
import {
  arrayMove,
  SortableContext,
  useSortable,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import {
  ArrowDown,
  ArrowUp,
  ArrowUpDown,
  BellRing,
  CalendarClock,
  ChevronRight,
  Copy,
  Download,
  Dices,
  Eye,
  EyeOff,
  GripVertical,
  ImagePlus,
  LogOut,
  MessageCircle,
  MoreHorizontal,
  Pencil,
  Plus,
  ReceiptText,
  RefreshCw,
  Search,
  Send,
  Trash2,
  Users,
  X,
} from 'lucide-react'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { api } from '@/lib/api'
import { handleServerError } from '@/lib/handle-server-error'
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
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Tabs, TabsContent } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'

export const Route = createFileRoute('/parking')({
  validateSearch: (search) => ({
    tab: ['platforms', 'spaces', 'members', 'renewals', 'reminders'].includes(
      search.tab
    )
      ? search.tab
      : 'spaces',
  }),
  beforeLoad: () => {
    const token = useAuthStore.getState().auth.accessToken
    if (!token) {
      throw redirect({ to: '/login' })
    }
  },
  component: ParkingPage,
})

const todayValue = () => {
  const date = new Date()
  date.setMinutes(date.getMinutes() - date.getTimezoneOffset())
  return date.toISOString().slice(0, 10)
}

const addMonthsValue = (value, months) => {
  const [year, month, day] = String(value || todayValue())
    .split('-')
    .map(Number)
  const date = new Date(year, month - 1, day)
  const originalDay = date.getDate()
  date.setDate(1)
  date.setMonth(date.getMonth() + Number(months || 0))
  const lastDay = new Date(date.getFullYear(), date.getMonth() + 1, 0).getDate()
  date.setDate(Math.min(originalDay, lastDay))
  date.setMinutes(date.getMinutes() - date.getTimezoneOffset())
  return date.toISOString().slice(0, 10)
}

const emptySpaceForm = (slots = 1) => ({
  name: '',
  password: '',
  platform_id: '',
  billing_day: '1',
  card_last4: '',
  tags: [],
  total_slots: String(slots),
  monthly_price: '',
  status: 'active',
  note: '',
})

function randomParkingPassword(length) {
  const characters = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789'
  const limit = Math.floor(256 / characters.length) * characters.length
  const result = []
  while (result.length < length) {
    const bytes = new Uint8Array(length - result.length)
    crypto.getRandomValues(bytes)
    for (const byte of bytes) {
      if (byte < limit) result.push(characters[byte % characters.length])
      if (result.length === length) break
    }
  }
  return result.join('')
}

function readPlatformIcon(file) {
  return new Promise((resolve, reject) => {
    if (!file?.type?.startsWith('image/')) {
      reject(new Error('请选择图片文件'))
      return
    }
    const reader = new FileReader()
    reader.onerror = () => reject(new Error('读取图片失败'))
    reader.onload = () => {
      const image = new Image()
      image.onerror = () => reject(new Error('图片格式不可用'))
      image.onload = () => {
        const size = 128
        const canvas = document.createElement('canvas')
        canvas.width = size
        canvas.height = size
        const context = canvas.getContext('2d')
        context.clearRect(0, 0, size, size)
        const scale = Math.min(size / image.width, size / image.height)
        const width = image.width * scale
        const height = image.height * scale
        context.drawImage(
          image,
          (size - width) / 2,
          (size - height) / 2,
          width,
          height
        )
        resolve(canvas.toDataURL('image/webp', 0.88))
      }
      image.src = String(reader.result)
    }
    reader.readAsDataURL(file)
  })
}

function assignMembersToSeats(space, members) {
  const seats = Array(Number(space.total_slots || 0)).fill(null)
  const remaining = []
  members
    .filter(
      (member) => member.space_id === space.id && member.status === 'active'
    )
    .forEach((member) => {
      const match = String(member.slot_label || '')
        .trim()
        .match(/^(\d+)号位$/)
      const index = match ? Number(match[1]) - 1 : -1
      if (index >= 0 && index < seats.length && !seats[index]) {
        seats[index] = member
      } else {
        remaining.push(member)
      }
    })
  seats.forEach((member, index) => {
    if (!member && remaining.length) seats[index] = remaining.shift()
  })
  return seats
}

function ParkingPage() {
  const queryClient = useQueryClient()
  const navigate = useNavigate({ from: '/parking' })
  const { tab } = Route.useSearch()
  const [spaceOpen, setSpaceOpen] = useState(false)
  const [platformPickerOpen, setPlatformPickerOpen] = useState(false)
  const [selectedPlatformForSpace, setSelectedPlatformForSpace] = useState(null)
  const [platformOpen, setPlatformOpen] = useState(false)
  const [editingPlatform, setEditingPlatform] = useState(null)
  const [platformSort, setPlatformSort] = useState('manual')
  const [platformSortMode, setPlatformSortMode] = useState(false)
  const [manualPlatformIds, setManualPlatformIds] = useState([])
  const [editingSpace, setEditingSpace] = useState(null)
  const [memberOpen, setMemberOpen] = useState(false)
  const [editingMember, setEditingMember] = useState(null)
  const [memberPeriod, setMemberPeriod] = useState(0)
  const [renewing, setRenewing] = useState(null)
  const [historyMember, setHistoryMember] = useState(null)
  const [spaceSearch, setSpaceSearch] = useState('')
  const [spacePlatform, setSpacePlatform] = useState('all')
  const [spaceSort, setSpaceSort] = useState('manual')
  const [spaceSortMode, setSpaceSortMode] = useState(false)
  const [spacePasswords, setSpacePasswords] = useState({})
  const [passwordLoading, setPasswordLoading] = useState(null)
  const [manualSpaceIds, setManualSpaceIds] = useState([])
  const [memberSearch, setMemberSearch] = useState('')
  const [memberStatus, setMemberStatus] = useState('all')
  const [memberExpiry, setMemberExpiry] = useState('all')
  const [renewalSearch, setRenewalSearch] = useState('')
  const [subscriptionOpen, setSubscriptionOpen] = useState(false)
  const [subscriptionForm, setSubscriptionForm] = useState({
    service_name: '',
    account_name: '',
    start_date: todayValue(),
    expire_date: '',
    renewal_months: '1',
    amount: '',
    reminder_days: '7',
    note: '',
  })
  const [spaceForm, setSpaceForm] = useState(() => emptySpaceForm())
  const [spacePasswordVisible, setSpacePasswordVisible] = useState(false)
  const [platformForm, setPlatformForm] = useState({
    name: '',
    icon: '',
    status: 'active',
    note: '',
    default_slots: '1',
    password_length: '8',
  })
  const [memberForm, setMemberForm] = useState({
    name: '',
    contact: '',
    contact_type: 'wechat',
    telegram: '',
    car_plate: '',
    space_id: '',
    slot_label: '',
    start_date: todayValue(),
    expire_date: '',
    amount: '',
    payment: '微信',
    note: '',
  })
  const [renewForm, setRenewForm] = useState({
    months: 1,
    new_expire_date: '',
    amount: '',
    payment: '微信',
    note: '',
  })

  const platformsQuery = useQuery({
    queryKey: ['parking-platforms'],
    queryFn: async () =>
      (await api.get('/api/admin/parking/platforms')).data.platforms ?? [],
    staleTime: 30 * 1000,
  })
  const spacesQuery = useQuery({
    queryKey: ['parking-spaces'],
    queryFn: async () =>
      (await api.get('/api/admin/parking/spaces')).data.spaces ?? [],
    staleTime: 30 * 1000,
  })
  const membersQuery = useQuery({
    queryKey: ['parking-members'],
    queryFn: async () =>
      (await api.get('/api/admin/parking/members')).data.members ?? [],
    staleTime: 30 * 1000,
  })
  const renewalsQuery = useQuery({
    queryKey: ['parking-renewals'],
    queryFn: async () =>
      (await api.get('/api/admin/parking/renewals')).data.renewals ?? [],
    staleTime: 30 * 1000,
  })
  const subscriptionsQuery = useQuery({
    queryKey: ['parking-subscriptions'],
    queryFn: async () =>
      (await api.get('/api/admin/parking/subscriptions')).data.subscriptions ??
      [],
    staleTime: 30 * 1000,
  })

  const invalidateParking = () => {
    queryClient.invalidateQueries({ queryKey: ['parking-summary'] })
    queryClient.invalidateQueries({ queryKey: ['parking-spaces'] })
    queryClient.invalidateQueries({ queryKey: ['parking-members'] })
    queryClient.invalidateQueries({ queryKey: ['parking-renewals'] })
    queryClient.invalidateQueries({ queryKey: ['parking-platforms'] })
    queryClient.invalidateQueries({ queryKey: ['parking-subscriptions'] })
  }

  const saveSubscription = useMutation({
    mutationFn: async () =>
      api.post('/api/admin/parking/subscriptions', {
        ...subscriptionForm,
        renewal_months: Number(subscriptionForm.renewal_months),
        amount: Number(subscriptionForm.amount || 0),
        reminder_days: Number(subscriptionForm.reminder_days),
      }),
    onSuccess: () => {
      toast.success('订阅已添加')
      setSubscriptionOpen(false)
      setSubscriptionForm({
        service_name: '',
        account_name: '',
        start_date: todayValue(),
        expire_date: '',
        renewal_months: '1',
        amount: '',
        reminder_days: '7',
        note: '',
      })
      queryClient.invalidateQueries({ queryKey: ['parking-subscriptions'] })
    },
    onError: handleServerError,
  })

  const savePlatform = useMutation({
    mutationFn: async () =>
      api.post(
        editingPlatform
          ? '/api/admin/parking/platforms/update'
          : '/api/admin/parking/platforms',
        {
          ...platformForm,
          id: editingPlatform?.id,
          default_slots: Number(platformForm.default_slots),
          password_length: Number(platformForm.password_length),
        }
      ),
    onSuccess: () => {
      toast.success(editingPlatform ? '平台已更新' : '平台已添加')
      setPlatformOpen(false)
      setEditingPlatform(null)
      setPlatformForm({
        name: '',
        icon: '',
        status: 'active',
        note: '',
        default_slots: '1',
        password_length: '8',
      })
      invalidateParking()
    },
    onError: handleServerError,
  })

  const deletePlatform = useMutation({
    mutationFn: async (platform) =>
      api.post('/api/admin/parking/platforms/delete', { id: platform.id }),
    onSuccess: () => {
      toast.success('平台已删除')
      invalidateParking()
    },
    onError: handleServerError,
  })

  const reorderPlatforms = useMutation({
    mutationFn: async (ids) =>
      api.post('/api/admin/parking/platforms/reorder', { ids }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['parking-platforms'] })
    },
    onError: (error) => {
      handleServerError(error)
      setPlatformSortMode(false)
      queryClient.invalidateQueries({ queryKey: ['parking-platforms'] })
    },
  })

  const createSpace = useMutation({
    mutationFn: async () =>
      api.post(
        editingSpace
          ? '/api/admin/parking/spaces/update'
          : '/api/admin/parking/spaces',
        {
          ...spaceForm,
          id: editingSpace?.id,
          platform_id: Number(spaceForm.platform_id),
          location: editingSpace?.location || '',
          total_slots: Number(spaceForm.total_slots),
          billing_day: Number(spaceForm.billing_day),
          monthly_price: Number(spaceForm.monthly_price || 0),
          tags: editingSpace?.tags || [],
        }
      ),
    onSuccess: () => {
      toast.success(editingSpace ? '车位已更新' : '车位已新增')
      if (editingSpace) {
        setSpacePasswords((values) => {
          const next = { ...values }
          delete next[editingSpace.id]
          return next
        })
      }
      setSpaceOpen(false)
      setPlatformPickerOpen(false)
      setEditingSpace(null)
      setSpaceForm(emptySpaceForm())
      setSpacePasswordVisible(false)
      invalidateParking()
    },
    onError: handleServerError,
  })

  const reorderSpaces = useMutation({
    mutationFn: async (ids) =>
      api.post('/api/admin/parking/spaces/reorder', { ids }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['parking-spaces'] })
    },
    onError: (error) => {
      handleServerError(error)
      setSpaceSortMode(false)
      queryClient.invalidateQueries({ queryKey: ['parking-spaces'] })
    },
  })

  const createMember = useMutation({
    mutationFn: async () =>
      api.post(
        editingMember
          ? '/api/admin/parking/members/update'
          : '/api/admin/parking/members',
        {
          ...memberForm,
          id: editingMember?.id,
          space_id: Number(memberForm.space_id),
          amount:
            editingMember && memberForm.amount === ''
              ? Number(editingMember.amount || 0)
              : Number(memberForm.amount || 0),
        }
      ),
    onSuccess: () => {
      toast.success(editingMember ? '车友已更新' : '车友已添加')
      setMemberOpen(false)
      setEditingMember(null)
      setMemberForm({
        name: '',
        contact: '',
        contact_type: 'wechat',
        telegram: '',
        car_plate: '',
        space_id: '',
        slot_label: '',
        start_date: todayValue(),
        expire_date: '',
        amount: '',
        payment: '微信',
        note: '',
      })
      invalidateParking()
    },
    onError: handleServerError,
  })

  const renewMember = useMutation({
    mutationFn: async () =>
      api.post('/api/admin/parking/members/renew', {
        member_id: renewing?.id,
        months: Number(renewForm.months),
        amount: Number(renewForm.amount),
        payment: renewForm.payment,
        note: renewForm.note,
        new_expire_date: renewForm.new_expire_date,
      }),
    onSuccess: () => {
      toast.success('续费已记录')
      setRenewing(null)
      setRenewForm({
        months: 1,
        new_expire_date: '',
        amount: '',
        payment: '微信',
        note: '',
      })
      invalidateParking()
    },
    onError: handleServerError,
  })

  const exitMember = useMutation({
    mutationFn: async (member) =>
      api.post('/api/admin/parking/members/exit', { member_id: member.id }),
    onSuccess: () => {
      toast.success('车友已标记退出')
      invalidateParking()
    },
    onError: handleServerError,
  })

  const platforms = useMemo(
    () => platformsQuery.data ?? [],
    [platformsQuery.data]
  )
  const displayedPlatforms = useMemo(() => {
    if (platformSort === 'az') {
      return [...platforms].sort((a, b) =>
        String(a.name).localeCompare(String(b.name), 'zh-CN', {
          sensitivity: 'base',
        })
      )
    }
    if (platformSortMode && manualPlatformIds.length) {
      return [...platforms].sort(
        (a, b) =>
          manualPlatformIds.indexOf(a.id) - manualPlatformIds.indexOf(b.id)
      )
    }
    return platforms
  }, [platforms, platformSort, platformSortMode, manualPlatformIds])
  const spaces = useMemo(() => spacesQuery.data ?? [], [spacesQuery.data])
  const members = useMemo(() => membersQuery.data ?? [], [membersQuery.data])
  const renewals = useMemo(() => renewalsQuery.data ?? [], [renewalsQuery.data])
  const platformSensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 6 } }),
    useSensor(TouchSensor, {
      activationConstraint: { delay: 350, tolerance: 8 },
    })
  )

  useEffect(() => {
    if (!platformSortMode)
      setManualPlatformIds(platforms.map((item) => item.id))
  }, [platforms, platformSortMode])
  const deferredSpaceSearch = useDeferredValue(spaceSearch.trim().toLowerCase())
  const platformCounts = useMemo(() => {
    const counts = new Map()
    spaces.forEach((space) => {
      const name = String(space.platform || '').trim() || '__ungrouped'
      counts.set(name, (counts.get(name) || 0) + 1)
    })
    return [...counts.entries()].sort(([a], [b]) => a.localeCompare(b, 'zh-CN'))
  }, [spaces])
  const filteredSpaces = useMemo(() => {
    const rows = spaces.filter((space) => {
      const platform = String(space.platform || '').trim() || '__ungrouped'
      const spaceMembers = members.filter(
        (member) => Number(member.space_id) === Number(space.id)
      )
      const searchable = [
        space.slot_number,
        space.name,
        space.platform,
        space.location,
        space.note,
        ...(space.tags || []),
        ...spaceMembers.flatMap((member) => [
          member.name,
          member.contact,
          member.telegram,
          member.car_plate,
          member.slot_label,
        ]),
      ]
        .map((value) => String(value || '').toLowerCase())
        .join(' ')
      return (
        (!deferredSpaceSearch || searchable.includes(deferredSpaceSearch)) &&
        (spacePlatform === 'all' || platform === spacePlatform)
      )
    })
    return [...rows].sort((a, b) => {
      if (spaceSortMode) {
        return manualSpaceIds.indexOf(a.id) - manualSpaceIds.indexOf(b.id)
      }
      if (spaceSort === 'manual') {
        return Number(a.sort_order || 0) - Number(b.sort_order || 0)
      }
      if (spaceSort === 'name-asc')
        return String(a.name).localeCompare(String(b.name), 'zh-CN')
      if (spaceSort === 'available-desc') {
        return (
          b.total_slots -
          (b.member_count || 0) -
          (a.total_slots - (a.member_count || 0))
        )
      }
      if (spaceSort === 'occupancy-desc') {
        return (
          (b.member_count || 0) / Math.max(b.total_slots, 1) -
          (a.member_count || 0) / Math.max(a.total_slots, 1)
        )
      }
      if (spaceSort === 'price-desc')
        return Number(b.monthly_price || 0) - Number(a.monthly_price || 0)
      return new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime()
    })
  }, [
    spaces,
    members,
    deferredSpaceSearch,
    spacePlatform,
    spaceSort,
    spaceSortMode,
    manualSpaceIds,
  ])
  const filteredMembers = useMemo(() => {
    const keyword = memberSearch.trim().toLowerCase()
    return members.filter((member) => {
      const days = dayDiff(member.expire_date)
      const keywordOk =
        !keyword ||
        [
          member.name,
          member.contact,
          member.telegram,
          member.car_plate,
          member.space_name,
          member.slot_label,
          member.note,
        ].some((value) =>
          String(value || '')
            .toLowerCase()
            .includes(keyword)
        )
      const statusOk = memberStatus === 'all' || member.status === memberStatus
      const expiryOk =
        memberExpiry === 'all' ||
        (memberExpiry === 'expired' && days < 0) ||
        (memberExpiry === '7' && days >= 0 && days <= 7) ||
        (memberExpiry === '30' && days >= 0 && days <= 30)
      return keywordOk && statusOk && expiryOk
    })
  }, [members, memberSearch, memberStatus, memberExpiry])
  const filteredRenewals = useMemo(() => {
    const keyword = renewalSearch.trim().toLowerCase()
    if (!keyword) return renewals
    return renewals.filter((record) =>
      [
        record.member_name,
        record.space_name,
        record.payment,
        record.operator,
        record.note,
      ].some((value) =>
        String(value || '')
          .toLowerCase()
          .includes(keyword)
      )
    )
  }, [renewals, renewalSearch])
  const historyRows = useMemo(
    () =>
      historyMember
        ? renewals.filter((record) => record.member_id === historyMember.id)
        : [],
    [renewals, historyMember]
  )

  const openSpaceDialog = (space = null) => {
    setEditingSpace(space)
    setSpacePasswordVisible(false)
    setSpaceForm(
      space
        ? {
            name: space.name || '',
            password: '',
            platform_id: String(space.platform_id || ''),
            billing_day: String(space.billing_day || 1),
            card_last4: space.card_last4 || '',
            tags: space.tags || [],
            total_slots: String(space.total_slots || 1),
            monthly_price: String(space.monthly_price ?? ''),
            status: space.status || 'active',
            note: space.note || '',
          }
        : emptySpaceForm()
    )
    setSpaceOpen(true)
  }

  const openSpaceInput = () => {
    setEditingSpace(null)
    setSelectedPlatformForSpace(null)
    setSpaceForm(emptySpaceForm())
    setSpacePasswordVisible(false)
    setPlatformPickerOpen(true)
  }

  const continueToSpaceForm = () => {
    if (!selectedPlatformForSpace) return
    setSpaceForm((form) => ({
      ...form,
      platform_id: String(selectedPlatformForSpace.id),
      total_slots: String(selectedPlatformForSpace.default_slots || 1),
    }))
    setPlatformPickerOpen(false)
    setSpaceOpen(true)
  }

  const copyText = async (value, message) => {
    try {
      await navigator.clipboard.writeText(value)
      toast.success(message)
    } catch {
      toast.error('复制失败，请检查浏览器剪贴板权限')
    }
  }

  const fetchSpacePassword = async (space) => {
    if (!space.password_set) return ''
    if (spacePasswords[space.id] !== undefined) {
      return spacePasswords[space.id]
    }
    setPasswordLoading(space.id)
    try {
      const response = await api.get('/api/admin/parking/spaces/password', {
        params: { id: space.id },
      })
      const password = response.data.password || ''
      setSpacePasswords((values) => ({
        ...values,
        [space.id]: password,
      }))
      return password
    } catch (error) {
      handleServerError(error)
      return null
    } finally {
      setPasswordLoading(null)
    }
  }

  const passwordsVisible =
    spaces.some((space) => space.password_set) &&
    spaces
      .filter((space) => space.password_set)
      .every((space) => spacePasswords[space.id] !== undefined)

  const toggleAllSpacePasswords = async () => {
    if (passwordsVisible) {
      setSpacePasswords({})
      return
    }
    setPasswordLoading('all')
    try {
      const entries = await Promise.all(
        spaces
          .filter((space) => space.password_set)
          .map(async (space) => {
            if (spacePasswords[space.id] !== undefined) {
              return [space.id, spacePasswords[space.id]]
            }
            const response = await api.get(
              '/api/admin/parking/spaces/password',
              {
                params: { id: space.id },
              }
            )
            return [space.id, response.data.password || '']
          })
      )
      setSpacePasswords(Object.fromEntries(entries))
    } catch (error) {
      handleServerError(error)
    } finally {
      setPasswordLoading(null)
    }
  }

  const copySpacePassword = async (space) => {
    const password = await fetchSpacePassword(space)
    if (password === null) return
    if (!password) {
      toast.error('该账号尚未设置密码')
      return
    }
    await copyText(password, '密码已复制')
  }

  const copySpaceCredentials = async (space) => {
    const password = await fetchSpacePassword(space)
    if (password === null) return
    await copyText(
      `平台：${space.platform || '未分组'}\n账号：${space.name}\n密码：${password || '未设置'}`,
      '账号密码已复制，可直接发送给车友'
    )
  }

  const openPlatformDialog = (platform = null) => {
    setEditingPlatform(platform)
    setPlatformForm(
      platform
        ? {
            name: platform.name || '',
            icon: platform.icon || '',
            status: platform.status || 'active',
            note: platform.note || '',
            default_slots: String(platform.default_slots || 1),
            password_length: String(platform.password_length || 8),
          }
        : {
            name: '',
            icon: '',
            status: 'active',
            note: '',
            default_slots: '1',
            password_length: '8',
          }
    )
    setPlatformOpen(true)
  }

  const togglePlatformSortMode = () => {
    if (platformSortMode) {
      setPlatformSortMode(false)
      reorderPlatforms.mutate(manualPlatformIds)
      return
    }
    setPlatformSort('manual')
    setManualPlatformIds(platforms.map((item) => item.id))
    setPlatformSortMode(true)
  }

  const handlePlatformDragEnd = ({ active, over }) => {
    if (!over || active.id === over.id) return
    setManualPlatformIds((ids) => {
      const oldIndex = ids.indexOf(Number(active.id))
      const newIndex = ids.indexOf(Number(over.id))
      if (oldIndex < 0 || newIndex < 0) return ids
      return arrayMove(ids, oldIndex, newIndex)
    })
  }

  const toggleSpaceSortMode = () => {
    if (spaceSortMode) {
      setSpaceSortMode(false)
      setSpaceSort('manual')
      return
    }
    setSpaceSearch('')
    setSpacePlatform('all')
    setManualSpaceIds(spaces.map((space) => space.id))
    setSpaceSortMode(true)
  }

  const moveSpace = (spaceID, offset) => {
    const index = manualSpaceIds.indexOf(spaceID)
    const target = index + offset
    if (index < 0 || target < 0 || target >= manualSpaceIds.length) return
    const next = [...manualSpaceIds]
    ;[next[index], next[target]] = [next[target], next[index]]
    setManualSpaceIds(next)
    reorderSpaces.mutate(next)
  }

  const openMemberDialog = (member = null, defaults = {}) => {
    setEditingMember(member)
    setMemberPeriod(member ? 0 : 1)
    setMemberForm(
      member
        ? {
            name: member.name || '',
            contact: member.contact || member.telegram || '',
            contact_type:
              member.contact_type || (member.telegram ? 'telegram' : 'wechat'),
            telegram: member.telegram || '',
            car_plate: member.car_plate || '',
            space_id: String(member.space_id || ''),
            slot_label: member.slot_label || '',
            start_date: inputDate(member.start_date),
            expire_date: inputDate(member.expire_date),
            status: member.status || 'active',
            amount: '',
            payment: member.payment || '微信',
            note: member.note || '',
          }
        : {
            name: '',
            contact: '',
            contact_type: 'wechat',
            telegram: '',
            car_plate: '',
            space_id: '',
            slot_label: '',
            start_date: todayValue(),
            expire_date: addMonthsValue(todayValue(), 1),
            status: 'active',
            amount: '',
            payment: '微信',
            note: '',
            ...defaults,
          }
    )
    setMemberOpen(true)
  }

  const openRenewDialog = (member) => {
    const currentExpire = inputDate(member.expire_date)
    const baseDate =
      currentExpire >= todayValue() ? currentExpire : todayValue()
    setRenewing(member)
    setRenewForm({
      months: 1,
      new_expire_date: addMonthsValue(baseDate, 1),
      amount: '',
      payment: member.payment || '微信',
      note: '',
    })
  }

  return (
    <div className='bg-background min-h-svh'>
      <main className='w-full px-4 pt-[72px] pb-8 sm:px-6 lg:px-8 xl:px-10'>
        <Tabs
          value={tab}
          onValueChange={(value) => navigate({ search: { tab: value } })}
          className='mt-0'
        >
          <TabsContent value='platforms' className='mt-0'>
            <Card>
              <CardHeader>
                <div className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
                  <CardTitle>平台管理</CardTitle>
                  <div className='flex flex-wrap items-center gap-2'>
                    <Select
                      value={platformSort}
                      disabled={platformSortMode}
                      onValueChange={setPlatformSort}
                    >
                      <SelectTrigger className='w-32' aria-label='平台排序方式'>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value='manual'>手动顺序</SelectItem>
                        <SelectItem value='az'>A-Z 排序</SelectItem>
                      </SelectContent>
                    </Select>
                    <Button
                      variant={platformSortMode ? 'default' : 'outline'}
                      onClick={togglePlatformSortMode}
                      disabled={!platforms.length || reorderPlatforms.isPending}
                    >
                      <ArrowUpDown className='size-4' />
                      {platformSortMode ? '完成排序' : '手动排序'}
                    </Button>
                    <Button onClick={() => openPlatformDialog()}>
                      <Plus className='size-4' />
                      添加平台
                    </Button>
                  </div>
                </div>
              </CardHeader>
              <CardContent>
                {displayedPlatforms.length ? (
                  <DndContext
                    sensors={platformSensors}
                    collisionDetection={closestCenter}
                    onDragEnd={handlePlatformDragEnd}
                  >
                    <div className='overflow-x-auto'>
                      <table className='w-full min-w-[820px] text-sm'>
                        <thead>
                          <tr className='border-b text-left'>
                            {platformSortMode && (
                              <th className='w-12 px-2 py-2' />
                            )}
                            <th className='px-3 py-2'>平台名称</th>
                            <th className='px-3 py-2'>账号数</th>
                            <th className='px-3 py-2'>总车位</th>
                            <th className='px-3 py-2'>默认车位数</th>
                            <th className='px-3 py-2'>随机密码位数</th>
                            <th className='px-3 py-2'>状态</th>
                            <th className='px-3 py-2'>备注</th>
                            <th className='px-3 py-2 text-right'>操作</th>
                          </tr>
                        </thead>
                        <SortableContext
                          items={manualPlatformIds.map(String)}
                          strategy={verticalListSortingStrategy}
                        >
                          <tbody>
                            {displayedPlatforms.map((platform) => (
                              <SortablePlatformRow
                                key={platform.id}
                                platform={platform}
                                sorting={platformSortMode}
                                onEdit={() => openPlatformDialog(platform)}
                                onDelete={() => {
                                  if (
                                    window.confirm(
                                      `确认删除平台「${platform.name}」吗？`
                                    )
                                  )
                                    deletePlatform.mutate(platform)
                                }}
                                deleting={deletePlatform.isPending}
                              />
                            ))}
                          </tbody>
                        </SortableContext>
                      </table>
                    </div>
                  </DndContext>
                ) : (
                  <div className='text-muted-foreground border border-dashed py-10 text-center text-sm'>
                    暂无平台，请先添加 Netflix 等平台
                  </div>
                )}
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value='spaces' className='mt-0'>
            <Card className='py-0'>
              <CardHeader className='border-b border-[var(--divider)] py-5'>
                <div className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
                  <CardTitle>合租</CardTitle>
                  <Button size='sm' onClick={openSpaceInput}>
                    <Plus className='size-4' /> 添加车位
                  </Button>
                </div>
              </CardHeader>
              <CardContent className='px-0 pb-0'>
                <div className='space-y-2 border-b border-[var(--divider)] px-5 py-3'>
                  {!spaceSortMode && (
                    <PlatformFilterBar
                      value={spacePlatform}
                      onChange={setSpacePlatform}
                      platforms={displayedPlatforms}
                      counts={platformCounts}
                      total={spaces.length}
                    />
                  )}
                  <div className='flex flex-wrap items-center gap-2'>
                    {!spaceSortMode && (
                      <>
                        <div className='relative min-w-44 flex-1 sm:max-w-sm'>
                          <Search className='text-muted-foreground pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2' />
                          <Input
                            value={spaceSearch}
                            onChange={(event) =>
                              setSpaceSearch(event.target.value)
                            }
                            placeholder='搜索账号、编号或车友'
                            className='h-9 pl-9'
                          />
                        </div>
                        <Select value={spaceSort} onValueChange={setSpaceSort}>
                          <SelectTrigger className='w-36 shrink-0'>
                            <ArrowUpDown className='mr-2 size-4' />
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value='manual'>编号顺序</SelectItem>
                            <SelectItem value='updated-desc'>
                              最近更新
                            </SelectItem>
                            <SelectItem value='name-asc'>账号排序</SelectItem>
                            <SelectItem value='available-desc'>
                              空位最多
                            </SelectItem>
                            <SelectItem value='occupancy-desc'>
                              占用率最高
                            </SelectItem>
                            <SelectItem value='price-desc'>价格最高</SelectItem>
                          </SelectContent>
                        </Select>
                      </>
                    )}
                    <Button
                      variant={spaceSortMode ? 'default' : 'outline'}
                      size='sm'
                      className='shrink-0'
                      title={spaceSortMode ? '完成排序' : '调整车位顺序'}
                      onClick={toggleSpaceSortMode}
                    >
                      <ArrowUpDown className='size-4' />
                      {spaceSortMode ? '完成排序' : '排序模式'}
                    </Button>
                  </div>
                </div>
                <div>
                  <SimpleTable
                    compact
                    empty={
                      spaces.length ? '没有符合筛选条件的车位' : '暂无车位'
                    }
                    rows={filteredSpaces}
                    columns={[
                      ['编号', (s, index) => s.slot_number || index + 1],
                      [
                        '平台',
                        (s) => (
                          <span className='font-medium'>
                            {s.platform || '未分组'}
                          </span>
                        ),
                      ],
                      [
                        '登录账号',
                        (s) => (
                          <button
                            type='button'
                            className='hover:text-primary max-w-44 truncate text-left font-medium underline-offset-4 hover:underline'
                            title='点击复制账号'
                            onClick={() => copyText(s.name, '账号已复制')}
                          >
                            {s.name}
                          </button>
                        ),
                      ],
                      [
                        <button
                          type='button'
                          className='hover:text-foreground inline-flex items-center gap-1'
                          title={
                            passwordsVisible ? '隐藏全部密码' : '显示全部密码'
                          }
                          disabled={passwordLoading === 'all'}
                          onClick={toggleAllSpacePasswords}
                        >
                          密码
                          {passwordsVisible ? (
                            <EyeOff className='size-3.5' />
                          ) : (
                            <Eye className='size-3.5' />
                          )}
                        </button>,
                        (s) => (
                          <button
                            type='button'
                            className='hover:text-primary font-mono text-xs underline-offset-4 hover:underline disabled:cursor-wait'
                            title='点击复制密码'
                            disabled={passwordLoading === s.id}
                            onClick={() => copySpacePassword(s)}
                          >
                            {spacePasswords[s.id] !== undefined
                              ? spacePasswords[s.id] || '未设置'
                              : s.password_set
                                ? '••••••••'
                                : '未设置'}
                          </button>
                        ),
                      ],
                      ['状态', (s) => <SeatStatus space={s} />],
                      [
                        '成员席位',
                        (s) => {
                          const seats = assignMembersToSeats(s, members)
                          return (
                            <div className='min-w-28 space-y-1'>
                              <div className='flex items-center gap-1 text-xs font-medium'>
                                <span>
                                  {s.member_count ?? 0}/{s.total_slots}
                                </span>
                                <span className='bg-muted h-1.5 flex-1 overflow-hidden rounded-full'>
                                  <span
                                    className='bg-primary block h-full rounded-full'
                                    style={{
                                      width: `${Math.min(
                                        100,
                                        (Number(s.member_count || 0) /
                                          Math.max(
                                            Number(s.total_slots || 1),
                                            1
                                          )) *
                                          100
                                      )}%`,
                                    }}
                                  />
                                </span>
                              </div>
                              <div className='flex items-center gap-1'>
                                {seats.map((member, slot) =>
                                  member ? (
                                    <button
                                      type='button'
                                      key={member.id}
                                      className='border-border hover:border-primary hover:text-primary flex h-6 min-w-6 items-center justify-center rounded-md border px-1.5 text-xs font-medium transition-colors'
                                      title={`编辑车友：${member.name}`}
                                      onClick={() => openMemberDialog(member)}
                                    >
                                      {String(member.name || slot + 1)
                                        .trim()
                                        .slice(0, 1)}
                                    </button>
                                  ) : null
                                )}
                                {Number(s.member_count || 0) <
                                  Number(s.total_slots || 0) && (
                                  <button
                                    type='button'
                                    className='border-border text-muted-foreground hover:border-primary hover:text-primary flex size-6 items-center justify-center rounded-md border border-dashed transition-colors'
                                    title='添加车友'
                                    aria-label={`为${s.name}添加车友`}
                                    onClick={() => {
                                      const openSeat =
                                        seats.findIndex((member) => !member) + 1
                                      openMemberDialog(null, {
                                        space_id: String(s.id),
                                        slot_label: `${openSeat}号位`,
                                        amount: Number(s.monthly_price || 0),
                                      })
                                    }}
                                  >
                                    <Plus className='size-3.5' />
                                  </button>
                                )}
                              </div>
                            </div>
                          )
                        },
                      ],
                      [
                        '最近到期',
                        (s) => (
                          <span className='whitespace-nowrap'>
                            {members
                              .filter(
                                (member) =>
                                  member.space_id === s.id &&
                                  member.status === 'active'
                              )
                              .sort(
                                (a, b) =>
                                  new Date(a.expire_date).getTime() -
                                  new Date(b.expire_date).getTime()
                              )[0]?.expire_date
                              ? formatDate(
                                  members
                                    .filter(
                                      (member) =>
                                        member.space_id === s.id &&
                                        member.status === 'active'
                                    )
                                    .sort(
                                      (a, b) =>
                                        new Date(a.expire_date).getTime() -
                                        new Date(b.expire_date).getTime()
                                    )[0].expire_date
                                )
                              : '-'}
                          </span>
                        ),
                      ],
                      [
                        '操作',
                        (s) => (
                          <div className='flex justify-end gap-0.5 whitespace-nowrap'>
                            {spaceSortMode ? (
                              <>
                                <Button
                                  size='icon'
                                  variant='outline'
                                  aria-label={`上移${s.name}`}
                                  disabled={
                                    manualSpaceIds.indexOf(s.id) <= 0 ||
                                    reorderSpaces.isPending
                                  }
                                  onClick={() => moveSpace(s.id, -1)}
                                >
                                  <ArrowUp className='size-3.5' />
                                </Button>
                                <Button
                                  size='icon'
                                  variant='outline'
                                  aria-label={`下移${s.name}`}
                                  disabled={
                                    manualSpaceIds.indexOf(s.id) ===
                                      manualSpaceIds.length - 1 ||
                                    reorderSpaces.isPending
                                  }
                                  onClick={() => moveSpace(s.id, 1)}
                                >
                                  <ArrowDown className='size-3.5' />
                                </Button>
                              </>
                            ) : (
                              <>
                                <Button
                                  size='icon'
                                  variant='outline'
                                  title='复制账号密码'
                                  aria-label={`复制${s.name}的账号密码`}
                                  disabled={passwordLoading === s.id}
                                  onClick={() => copySpaceCredentials(s)}
                                >
                                  <Copy className='size-3.5' />
                                </Button>
                                <Button
                                  size='icon'
                                  variant='outline'
                                  title='编辑车位'
                                  aria-label={`编辑${s.name}`}
                                  onClick={() => openSpaceDialog(s)}
                                >
                                  <Pencil className='size-3.5' />
                                </Button>
                              </>
                            )}
                          </div>
                        ),
                      ],
                    ]}
                  />
                </div>
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value='members' className='mt-0'>
            <Card>
              <CardHeader>
                <div className='flex flex-col gap-3 xl:flex-row xl:items-center xl:justify-between'>
                  <CardTitle>会员列表</CardTitle>
                  <div className='flex flex-col gap-2 sm:flex-row sm:items-center'>
                    <Button
                      size='sm'
                      disabled={!spaces.length}
                      onClick={() => openMemberDialog()}
                    >
                      <Users className='size-4' />
                      添加车友
                    </Button>
                    <div className='relative w-64 shrink-0'>
                      <Search className='text-muted-foreground pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2' />
                      <Input
                        value={memberSearch}
                        onChange={(e) => setMemberSearch(e.target.value)}
                        placeholder='搜索会员、联系方式或车位'
                        className='pl-9'
                      />
                    </div>
                    <Select
                      value={memberStatus}
                      onValueChange={setMemberStatus}
                    >
                      <SelectTrigger className='w-36'>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value='all'>全部状态</SelectItem>
                        <SelectItem value='active'>在位</SelectItem>
                        <SelectItem value='exited'>已退出</SelectItem>
                      </SelectContent>
                    </Select>
                    <Select
                      value={memberExpiry}
                      onValueChange={setMemberExpiry}
                    >
                      <SelectTrigger className='w-36'>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value='all'>全部到期</SelectItem>
                        <SelectItem value='7'>7 天内</SelectItem>
                        <SelectItem value='30'>30 天内</SelectItem>
                        <SelectItem value='expired'>已过期</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                </div>
              </CardHeader>
              <CardContent>
                <SimpleTable
                  empty='暂无符合条件的会员'
                  rows={filteredMembers}
                  columns={[
                    ['会员名称', (m) => <strong>{m.name}</strong>],
                    ['联系方式', (m) => <ContactDisplay member={m} />],
                    [
                      '车位',
                      (m) => `${m.space_name || '-'} ${m.slot_label || ''}`,
                    ],
                    [
                      '状态',
                      (m) =>
                        m.status === 'active' ? (
                          <ExpiryBadge date={m.expire_date} />
                        ) : (
                          <Badge variant='secondary'>已退出</Badge>
                        ),
                    ],
                    ['金额', (m) => `¥ ${Number(m.amount || 0).toFixed(2)}`],
                    [
                      '操作',
                      (m) => (
                        <div className='flex justify-end gap-2'>
                          <Button
                            size='sm'
                            variant='outline'
                            onClick={() => openMemberDialog(m)}
                          >
                            <Pencil className='size-3.5' />
                            编辑
                          </Button>
                          {m.status === 'active' && (
                            <Button
                              size='sm'
                              variant='outline'
                              onClick={() => openRenewDialog(m)}
                            >
                              <RefreshCw className='size-3.5' />
                              续费
                            </Button>
                          )}
                          <DropdownMenu>
                            <DropdownMenuTrigger asChild>
                              <Button
                                size='icon'
                                variant='outline'
                                aria-label={`更多${m.name}操作`}
                              >
                                <MoreHorizontal className='size-4' />
                              </Button>
                            </DropdownMenuTrigger>
                            <DropdownMenuContent align='end'>
                              <DropdownMenuItem
                                onClick={() => setHistoryMember(m)}
                              >
                                <ReceiptText className='size-4' /> 历史记录
                              </DropdownMenuItem>
                              {m.status === 'active' && (
                                <DropdownMenuItem
                                  className='text-destructive focus:text-destructive'
                                  disabled={exitMember.isPending}
                                  onClick={() => {
                                    if (
                                      window.confirm(
                                        `确认将「${m.name}」标记为退出吗？`
                                      )
                                    )
                                      exitMember.mutate(m)
                                  }}
                                >
                                  <LogOut className='size-4' /> 退出
                                </DropdownMenuItem>
                              )}
                            </DropdownMenuContent>
                          </DropdownMenu>
                        </div>
                      ),
                    ],
                  ]}
                />
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value='renewals' className='mt-0'>
            <Card>
              <CardHeader>
                <div className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
                  <CardTitle>续费记录</CardTitle>
                  <div className='flex flex-col gap-2 sm:flex-row'>
                    <Input
                      value={renewalSearch}
                      onChange={(e) => setRenewalSearch(e.target.value)}
                      placeholder='搜索车友/车位/备注'
                      className='sm:w-56'
                    />
                    <Button
                      variant='outline'
                      onClick={() => exportRenewals(filteredRenewals)}
                    >
                      <Download className='size-4' />
                      导出
                    </Button>
                  </div>
                </div>
              </CardHeader>
              <CardContent>
                <SimpleTable
                  empty='暂无续费记录'
                  rows={filteredRenewals}
                  columns={[
                    ['车友', (r) => r.member_name],
                    ['车位', (r) => r.space_name],
                    ['续费周期', (r) => `${r.months} 个月`],
                    [
                      '到期变化',
                      (r) =>
                        `${formatDate(r.old_expire_date)} 到 ${formatDate(r.new_expire_date)}`,
                    ],
                    [
                      '金额',
                      (r) => (
                        <strong>¥ {Number(r.amount || 0).toFixed(2)}</strong>
                      ),
                    ],
                    ['操作人', (r) => r.operator || '-'],
                    ['备注', (r) => r.note || '-'],
                  ]}
                />
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value='reminders' className='mt-0'>
            <Card>
              <CardHeader>
                <div className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
                  <CardTitle>我的订阅</CardTitle>
                  <div className='flex items-center gap-2'>
                    <Button
                      variant='outline'
                      size='sm'
                      onClick={() =>
                        toast.info('通知设置沿用系统 Telegram 配置')
                      }
                    >
                      <BellRing className='size-4' /> 通知设置
                    </Button>
                    <Button size='sm' onClick={() => setSubscriptionOpen(true)}>
                      <Plus className='size-4' /> 添加订阅
                    </Button>
                  </div>
                </div>
              </CardHeader>
              <CardContent>
                {subscriptionsQuery.data?.length ? (
                  <TableList
                    rows={subscriptionsQuery.data}
                    columns={[
                      ['服务', (item) => item.service_name],
                      ['账号 / 名称', (item) => item.account_name || '-'],
                      [
                        '到期时间',
                        (item) => <ExpiryBadge date={item.expire_date} />,
                      ],
                      ['续费周期', (item) => `${item.renewal_months} 个月`],
                      [
                        '金额',
                        (item) => `¥ ${Number(item.amount || 0).toFixed(2)}`,
                      ],
                      ['提前提醒', (item) => `${item.reminder_days} 天`],
                      ['备注', (item) => item.note || '-'],
                    ]}
                  />
                ) : (
                  <div className='flex min-h-36 flex-col items-center justify-center text-center'>
                    <CalendarClock className='mb-3 size-8 text-[var(--text-muted)]' />
                    <p className='font-medium text-[var(--text-primary)]'>
                      暂无订阅
                    </p>
                    <Button
                      size='sm'
                      className='mt-4'
                      onClick={() => setSubscriptionOpen(true)}
                    >
                      <Plus className='size-4' /> 添加订阅
                    </Button>
                  </div>
                )}
              </CardContent>
            </Card>
          </TabsContent>
        </Tabs>
      </main>

      <Dialog open={subscriptionOpen} onOpenChange={setSubscriptionOpen}>
        <DialogContent className='sm:max-w-2xl'>
          <DialogHeader>
            <DialogTitle>添加订阅</DialogTitle>
            <DialogDescription>
              记录你自己购买的会员、软件、域名或服务器服务。
            </DialogDescription>
          </DialogHeader>
          <FormGrid>
            <Field label='服务名称'>
              <Input
                value={subscriptionForm.service_name}
                onChange={(event) =>
                  setSubscriptionForm({
                    ...subscriptionForm,
                    service_name: event.target.value,
                  })
                }
                placeholder='例如 Netflix Premium'
              />
            </Field>
            <Field label='账号 / 名称'>
              <Input
                value={subscriptionForm.account_name}
                onChange={(event) =>
                  setSubscriptionForm({
                    ...subscriptionForm,
                    account_name: event.target.value,
                  })
                }
                placeholder='账号或便于识别的名称'
              />
            </Field>
            <Field label='开始时间'>
              <Input
                type='date'
                value={subscriptionForm.start_date}
                onChange={(event) =>
                  setSubscriptionForm({
                    ...subscriptionForm,
                    start_date: event.target.value,
                  })
                }
              />
            </Field>
            <Field label='到期时间'>
              <Input
                type='date'
                value={subscriptionForm.expire_date}
                onChange={(event) =>
                  setSubscriptionForm({
                    ...subscriptionForm,
                    expire_date: event.target.value,
                  })
                }
              />
            </Field>
            <Field label='续费周期'>
              <Select
                value={subscriptionForm.renewal_months}
                onValueChange={(value) =>
                  setSubscriptionForm({
                    ...subscriptionForm,
                    renewal_months: value,
                  })
                }
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='1'>1 个月</SelectItem>
                  <SelectItem value='3'>3 个月</SelectItem>
                  <SelectItem value='6'>6 个月</SelectItem>
                  <SelectItem value='12'>12 个月</SelectItem>
                </SelectContent>
              </Select>
            </Field>
            <Field label='续费金额'>
              <Input
                type='number'
                min='0'
                value={subscriptionForm.amount}
                onChange={(event) =>
                  setSubscriptionForm({
                    ...subscriptionForm,
                    amount: event.target.value,
                  })
                }
                placeholder='0.00'
              />
            </Field>
            <Field label='提前提醒'>
              <Select
                value={subscriptionForm.reminder_days}
                onValueChange={(value) =>
                  setSubscriptionForm({
                    ...subscriptionForm,
                    reminder_days: value,
                  })
                }
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {[1, 3, 7, 15, 30].map((days) => (
                    <SelectItem key={days} value={String(days)}>
                      提前 {days} 天
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </Field>
            <Field label='通知方式'>
              <Input value='Telegram' readOnly />
            </Field>
            <Field label='备注' wide>
              <Textarea
                value={subscriptionForm.note}
                onChange={(event) =>
                  setSubscriptionForm({
                    ...subscriptionForm,
                    note: event.target.value,
                  })
                }
                placeholder='选填'
              />
            </Field>
          </FormGrid>
          <DialogFooter>
            <Button
              variant='outline'
              onClick={() => setSubscriptionOpen(false)}
            >
              取消
            </Button>
            <Button
              disabled={
                saveSubscription.isPending ||
                !subscriptionForm.service_name.trim() ||
                !subscriptionForm.expire_date
              }
              onClick={() => saveSubscription.mutate()}
            >
              保存订阅
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog
        open={platformPickerOpen}
        onOpenChange={(open) => {
          setPlatformPickerOpen(open)
          if (!open) setSelectedPlatformForSpace(null)
        }}
      >
        <DialogContent className='sm:max-w-2xl'>
          <DialogHeader>
            <DialogTitle>选择平台</DialogTitle>
            <DialogDescription>
              请选择一个已添加的平台，然后继续创建车位。
            </DialogDescription>
          </DialogHeader>
          <div className='grid max-h-[55vh] gap-2 overflow-y-auto sm:grid-cols-2'>
            {displayedPlatforms.map((platform) => {
              const selected = selectedPlatformForSpace?.id === platform.id
              const paused = platform.status !== 'active'
              return (
                <button
                  key={platform.id}
                  type='button'
                  disabled={paused}
                  aria-pressed={selected}
                  onClick={() => setSelectedPlatformForSpace(platform)}
                  className={`flex min-h-12 items-center justify-between rounded-md border px-3 py-2 text-left text-sm transition-colors ${
                    selected
                      ? 'border-primary bg-primary/10 ring-primary/30 ring-2'
                      : 'hover:bg-muted/50'
                  } ${paused ? 'cursor-not-allowed opacity-50' : 'cursor-pointer'}`}
                >
                  <span className='flex items-center gap-2 font-medium'>
                    <PlatformIcon platform={platform} className='size-6' />
                    {platform.name}
                  </span>
                  <span className='text-muted-foreground text-xs'>
                    {paused
                      ? '已暂停'
                      : `默认 ${platform.default_slots || 1} 席 · 密码 ${platform.password_length || 8} 位`}
                  </span>
                </button>
              )
            })}
          </div>
          {!platforms.length && (
            <div className='text-muted-foreground rounded-md border border-dashed py-8 text-center text-sm'>
              暂无平台，请先到平台管理添加平台。
            </div>
          )}
          <DialogFooter>
            <Button
              variant='outline'
              onClick={() => setPlatformPickerOpen(false)}
            >
              取消
            </Button>
            <Button
              disabled={!selectedPlatformForSpace}
              onClick={continueToSpaceForm}
            >
              下一步
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog
        open={platformOpen}
        onOpenChange={(open) => {
          setPlatformOpen(open)
          if (!open) setEditingPlatform(null)
        }}
      >
        <DialogContent className='max-h-[90dvh] overflow-y-auto sm:max-w-2xl'>
          <DialogHeader>
            <DialogTitle>
              {editingPlatform ? '编辑平台' : '添加平台'}
            </DialogTitle>
            <DialogDescription>
              添加 Netflix、Disney+ 等共享账号平台。
            </DialogDescription>
          </DialogHeader>
          <FormGrid>
            <Field label='平台图标' wide>
              <div className='flex items-center gap-3'>
                <PlatformIcon
                  platform={platformForm}
                  className='bg-background size-14 border'
                />
                <label className='border-input hover:bg-accent inline-flex h-9 cursor-pointer items-center gap-2 border px-3 text-sm font-medium'>
                  <ImagePlus className='size-4' />
                  选择图片
                  <input
                    type='file'
                    accept='image/*'
                    className='sr-only'
                    onChange={async (event) => {
                      const file = event.target.files?.[0]
                      if (!file) return
                      try {
                        const icon = await readPlatformIcon(file)
                        setPlatformForm((current) => ({ ...current, icon }))
                      } catch (error) {
                        toast.error(error.message || '图标处理失败')
                      } finally {
                        event.target.value = ''
                      }
                    }}
                  />
                </label>
                {platformForm.icon && (
                  <Button
                    type='button'
                    size='icon'
                    variant='outline'
                    title='移除平台图标'
                    aria-label='移除平台图标'
                    onClick={() =>
                      setPlatformForm((current) => ({ ...current, icon: '' }))
                    }
                  >
                    <X className='size-4' />
                  </Button>
                )}
              </div>
            </Field>
            <Field label='平台名称' htmlFor='parking-platform-name'>
              <Input
                id='parking-platform-name'
                value={platformForm.name}
                onChange={(event) =>
                  setPlatformForm({
                    ...platformForm,
                    name: event.target.value,
                  })
                }
                placeholder='例如 Netflix'
              />
            </Field>
            <Field label='状态' htmlFor='parking-platform-status'>
              <Select
                value={platformForm.status}
                onValueChange={(value) =>
                  setPlatformForm({ ...platformForm, status: value })
                }
              >
                <SelectTrigger id='parking-platform-status'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='active'>启用</SelectItem>
                  <SelectItem value='paused'>暂停</SelectItem>
                </SelectContent>
              </Select>
            </Field>
            <Field label='默认车位数量' htmlFor='parking-platform-slots'>
              <Input
                id='parking-platform-slots'
                type='number'
                min='1'
                max='100'
                value={platformForm.default_slots}
                onChange={(event) =>
                  setPlatformForm({
                    ...platformForm,
                    default_slots: event.target.value,
                  })
                }
              />
            </Field>
            <Field
              label='随机密码位数'
              htmlFor='parking-platform-password-length'
            >
              <Input
                id='parking-platform-password-length'
                type='number'
                min='1'
                max='128'
                value={platformForm.password_length}
                onChange={(event) =>
                  setPlatformForm({
                    ...platformForm,
                    password_length: event.target.value,
                  })
                }
              />
            </Field>
            <Field label='备注' htmlFor='parking-platform-note' wide>
              <Textarea
                id='parking-platform-note'
                value={platformForm.note}
                onChange={(event) =>
                  setPlatformForm({
                    ...platformForm,
                    note: event.target.value,
                  })
                }
                placeholder='可填写平台说明'
              />
            </Field>
          </FormGrid>
          <DialogFooter>
            <Button variant='outline' onClick={() => setPlatformOpen(false)}>
              取消
            </Button>
            <Button
              disabled={
                !platformForm.name.trim() ||
                !Number.isInteger(Number(platformForm.default_slots)) ||
                Number(platformForm.default_slots) < 1 ||
                Number(platformForm.default_slots) > 100 ||
                !Number.isInteger(Number(platformForm.password_length)) ||
                Number(platformForm.password_length) < 1 ||
                Number(platformForm.password_length) > 128 ||
                savePlatform.isPending
              }
              onClick={() => savePlatform.mutate()}
            >
              保存
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog
        open={spaceOpen}
        onOpenChange={(open) => {
          setSpaceOpen(open)
          if (!open) setEditingSpace(null)
        }}
      >
        <DialogContent className='max-h-[90dvh] gap-3 overflow-y-auto p-5 sm:max-w-xl'>
          <DialogHeader className='gap-1'>
            <DialogTitle>{editingSpace ? '编辑车位' : '添加车位'}</DialogTitle>
            <DialogDescription>
              {editingSpace
                ? '修改共享账号、平台续费信息与席位容量。'
                : `平台：${selectedPlatformForSpace?.name || ''}，填写账号资料后即可创建。`}
            </DialogDescription>
          </DialogHeader>
          <SpaceFormFields
            form={spaceForm}
            setForm={setSpaceForm}
            platforms={platforms}
            editingSpace={editingSpace}
            passwordVisible={spacePasswordVisible}
            setPasswordVisible={setSpacePasswordVisible}
          />
          <DialogFooter className='border-border mt-1 border-t pt-4'>
            <Button variant='outline' onClick={() => setSpaceOpen(false)}>
              取消
            </Button>
            <Button
              disabled={
                createSpace.isPending ||
                !spaceForm.platform_id ||
                !spaceForm.name.trim() ||
                !Number.isInteger(Number(spaceForm.total_slots)) ||
                Number(spaceForm.total_slots) < 1 ||
                Number(spaceForm.total_slots) > 100 ||
                !Number.isInteger(Number(spaceForm.billing_day)) ||
                Number(spaceForm.billing_day) < 1 ||
                Number(spaceForm.billing_day) > 31 ||
                (spaceForm.monthly_price !== '' &&
                  (!Number.isFinite(Number(spaceForm.monthly_price)) ||
                    Number(spaceForm.monthly_price) < 0))
              }
              onClick={() => createSpace.mutate()}
            >
              {editingSpace ? '保存' : '添加'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog
        open={memberOpen}
        onOpenChange={(open) => {
          setMemberOpen(open)
          if (!open) setEditingMember(null)
        }}
      >
        <DialogContent className='max-h-[92dvh] overflow-y-auto sm:max-w-2xl'>
          <DialogHeader>
            <DialogTitle>
              {editingMember
                ? `编辑车友 · ${editingMember.name}`
                : `添加车友${spaces.find((space) => String(space.id) === String(memberForm.space_id))?.name ? ` · ${spaces.find((space) => String(space.id) === String(memberForm.space_id)).name}` : ''}`}
            </DialogTitle>
            <DialogDescription>
              记录会员名称、联系方式和会员有效期。
            </DialogDescription>
          </DialogHeader>
          {(() => {
            const selectedSpace = spaces.find(
              (space) => String(space.id) === String(memberForm.space_id)
            )
            return (
              <div className='border-border bg-muted/35 flex items-center justify-between gap-3 border px-3 py-2.5'>
                <div className='min-w-0'>
                  <div className='text-muted-foreground text-xs'>车位账号</div>
                  <div className='truncate font-medium'>
                    {selectedSpace?.name || '尚未选择车位'}
                  </div>
                </div>
                <Badge variant='outline'>
                  {memberForm.slot_label || '自动分配席位'}
                </Badge>
              </div>
            )
          })()}
          <FormGrid>
            <Field label='车位名称'>
              <Input
                value={memberForm.name}
                placeholder='会员名称或聊天网名'
                onChange={(e) =>
                  setMemberForm({ ...memberForm, name: e.target.value })
                }
              />
            </Field>
            <Field label='联系方式'>
              <div className='border-input flex h-9 overflow-hidden border'>
                <ContactTypeButtons
                  value={memberForm.contact_type}
                  onChange={(contactType) =>
                    setMemberForm({
                      ...memberForm,
                      contact_type: contactType,
                    })
                  }
                />
                <Input
                  className='h-full flex-1 border-0 shadow-none focus-visible:ring-0'
                  value={memberForm.contact}
                  placeholder='输入账号或联系方式'
                  onChange={(e) =>
                    setMemberForm({ ...memberForm, contact: e.target.value })
                  }
                />
              </div>
            </Field>
            <Field label='车位'>
              <Select
                value={String(memberForm.space_id)}
                onValueChange={(value) =>
                  setMemberForm({
                    ...memberForm,
                    space_id: value,
                    amount:
                      spaces.find((s) => String(s.id) === value)
                        ?.monthly_price ?? memberForm.amount,
                  })
                }
              >
                <SelectTrigger>
                  <SelectValue placeholder='选择车位' />
                </SelectTrigger>
                <SelectContent>
                  {spaces.map((space) => (
                    <SelectItem
                      value={String(space.id)}
                      key={space.id}
                      disabled={
                        space.status !== 'active' ||
                        ((space.member_count ?? 0) >= space.total_slots &&
                          String(space.id) !==
                            String(editingMember?.space_id || ''))
                      }
                    >
                      {space.name} · {space.member_count ?? 0}/
                      {space.total_slots}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </Field>
            <Field label='席位'>
              <Input
                value={memberForm.slot_label}
                onChange={(e) =>
                  setMemberForm({ ...memberForm, slot_label: e.target.value })
                }
                placeholder='例如 1 号位'
              />
            </Field>
            <Field label='首次续费时间'>
              <Input
                type='date'
                value={memberForm.start_date}
                onChange={(e) => {
                  const startDate = e.target.value
                  setMemberForm({
                    ...memberForm,
                    start_date: startDate,
                    expire_date: memberPeriod
                      ? addMonthsValue(startDate, memberPeriod)
                      : memberForm.expire_date,
                  })
                }}
              />
            </Field>
            <Field label='会员到期时间'>
              <Input
                type='date'
                value={memberForm.expire_date}
                onChange={(e) => {
                  setMemberPeriod(0)
                  setMemberForm({ ...memberForm, expire_date: e.target.value })
                }}
              />
            </Field>
            <Field label='续费周期' wide>
              <PeriodButtons
                value={memberPeriod}
                onChange={(months) => {
                  setMemberPeriod(months)
                  setMemberForm({
                    ...memberForm,
                    expire_date: addMonthsValue(memberForm.start_date, months),
                  })
                }}
              />
            </Field>
            <Field label='本次收款金额'>
              <Input
                type='number'
                min='0'
                value={memberForm.amount}
                placeholder='0.00'
                onChange={(e) =>
                  setMemberForm({
                    ...memberForm,
                    amount: e.target.value,
                  })
                }
              />
            </Field>
            <Field label='续费记录'>
              <Button
                type='button'
                variant='outline'
                className='w-full justify-between'
                disabled={!editingMember}
                onClick={() => {
                  setMemberOpen(false)
                  setHistoryMember(editingMember)
                }}
              >
                查看续费记录
                <ChevronRight className='size-4' />
              </Button>
            </Field>
            <Field label='备注' wide>
              <Textarea
                value={memberForm.note}
                onChange={(e) =>
                  setMemberForm({ ...memberForm, note: e.target.value })
                }
              />
            </Field>
          </FormGrid>
          <DialogFooter>
            <Button variant='outline' onClick={() => setMemberOpen(false)}>
              取消
            </Button>
            <Button
              disabled={createMember.isPending}
              onClick={() => createMember.mutate()}
            >
              保存
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog
        open={Boolean(historyMember)}
        onOpenChange={(open) => !open && setHistoryMember(null)}
      >
        <DialogContent className='sm:max-w-3xl'>
          <DialogHeader>
            <DialogTitle>续费历史</DialogTitle>
            <DialogDescription>
              {historyMember?.name || '车友'} 的历史续费记录。
            </DialogDescription>
          </DialogHeader>
          <SimpleTable
            empty='暂无续费历史'
            rows={historyRows}
            columns={[
              ['续费时间', (r) => formatDate(r.created_at)],
              ['周期', (r) => `${r.months} 个月`],
              [
                '到期变化',
                (r) =>
                  `${formatDate(r.old_expire_date)} 到 ${formatDate(r.new_expire_date)}`,
              ],
              ['金额', (r) => `¥ ${Number(r.amount || 0).toFixed(2)}`],
              ['支付', (r) => r.payment || '-'],
              ['备注', (r) => r.note || '-'],
            ]}
          />
          <DialogFooter>
            <Button variant='outline' onClick={() => setHistoryMember(null)}>
              关闭
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog
        open={Boolean(renewing)}
        onOpenChange={(open) => !open && setRenewing(null)}
      >
        <DialogContent className='sm:max-w-2xl'>
          <DialogHeader>
            <DialogTitle>记录续费</DialogTitle>
            <DialogDescription>
              确认后会写入续费历史，并自动更新车友到期时间。
            </DialogDescription>
          </DialogHeader>
          <div className='space-y-4'>
            <div className='border-border bg-muted/35 flex flex-wrap items-center gap-x-4 gap-y-1 border px-3 py-3 text-sm'>
              <span className='font-medium'>{renewing?.name}</span>
              <span className='text-muted-foreground'>
                当前到期 {formatDate(renewing?.expire_date)}
              </span>
            </div>
            <FormGrid>
              <Field label='续费周期' wide>
                <PeriodButtons
                  value={renewForm.months}
                  onChange={(months) => {
                    const currentExpire = inputDate(renewing?.expire_date)
                    const baseDate =
                      currentExpire >= todayValue()
                        ? currentExpire
                        : todayValue()
                    setRenewForm({
                      ...renewForm,
                      months,
                      new_expire_date: addMonthsValue(baseDate, months),
                    })
                  }}
                />
              </Field>
              <Field label='新到期时间'>
                <Input
                  type='date'
                  value={renewForm.new_expire_date}
                  onChange={(e) =>
                    setRenewForm({
                      ...renewForm,
                      months: 0,
                      new_expire_date: e.target.value,
                    })
                  }
                />
              </Field>
              <Field label='本次收款金额'>
                <Input
                  type='number'
                  min='0'
                  value={renewForm.amount}
                  placeholder='0.00'
                  onChange={(e) =>
                    setRenewForm({
                      ...renewForm,
                      amount: e.target.value,
                    })
                  }
                />
              </Field>
              <Field label='支付方式'>
                <Select
                  value={renewForm.payment}
                  onValueChange={(payment) =>
                    setRenewForm({ ...renewForm, payment })
                  }
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value='微信'>微信</SelectItem>
                    <SelectItem value='支付宝'>支付宝</SelectItem>
                    <SelectItem value='银行卡'>银行卡</SelectItem>
                    <SelectItem value='其他'>其他</SelectItem>
                  </SelectContent>
                </Select>
              </Field>
              <Field label='备注' wide>
                <Textarea
                  className='min-h-20'
                  value={renewForm.note}
                  onChange={(e) =>
                    setRenewForm({ ...renewForm, note: e.target.value })
                  }
                />
              </Field>
            </FormGrid>
          </div>
          <DialogFooter>
            <Button variant='outline' onClick={() => setRenewing(null)}>
              取消
            </Button>
            <Button
              disabled={renewMember.isPending || !renewForm.new_expire_date}
              onClick={() => renewMember.mutate()}
            >
              确认续费
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}

function SpaceFormFields({
  form,
  setForm,
  platforms,
  editingSpace = null,
  passwordVisible,
  setPasswordVisible,
}) {
  const [diceRolling, setDiceRolling] = useState(false)
  const idPrefix = editingSpace ? 'parking-space-edit' : 'parking-space-add'
  const selectedPlatform = platforms.find(
    (platform) => String(platform.id) === String(form.platform_id)
  )
  const passwordLength = selectedPlatform?.password_length || 8

  return (
    <FormGrid>
      <Field label='平台' htmlFor={`${idPrefix}-platform`}>
        <Select
          value={String(form.platform_id)}
          onValueChange={(value) => {
            const platform = platforms.find((item) => String(item.id) === value)
            setForm({
              ...form,
              platform_id: value,
              total_slots: editingSpace
                ? form.total_slots
                : String(platform?.default_slots || 1),
            })
          }}
        >
          <SelectTrigger id={`${idPrefix}-platform`}>
            <SelectValue placeholder='选择平台' />
          </SelectTrigger>
          <SelectContent>
            {platforms.map((platform) => (
              <SelectItem
                key={platform.id}
                value={String(platform.id)}
                disabled={
                  platform.status !== 'active' &&
                  String(platform.id) !==
                    String(editingSpace?.platform_id || '')
                }
              >
                {platform.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </Field>
      <Field label='登录账号' htmlFor={`${idPrefix}-account`}>
        <Input
          id={`${idPrefix}-account`}
          value={form.name}
          onChange={(event) => setForm({ ...form, name: event.target.value })}
          placeholder='输入平台邮箱账号'
          type='email'
        />
      </Field>
      <Field label='账号密码' htmlFor={`${idPrefix}-password`}>
        <div className='flex min-w-0 gap-2'>
          <Input
            id={`${idPrefix}-password`}
            className='min-w-0 flex-1'
            value={form.password}
            onChange={(event) =>
              setForm({ ...form, password: event.target.value })
            }
            placeholder={editingSpace ? '留空表示不修改' : '输入或生成随机密码'}
            type={passwordVisible ? 'text' : 'password'}
            autoComplete='new-password'
          />
          <Button
            type='button'
            size='icon'
            variant='outline'
            className='shrink-0'
            title={passwordVisible ? '隐藏密码' : '显示密码'}
            aria-label={passwordVisible ? '隐藏密码' : '显示密码'}
            onClick={() => setPasswordVisible(!passwordVisible)}
          >
            {passwordVisible ? (
              <EyeOff className='size-4' />
            ) : (
              <Eye className='size-4' />
            )}
          </Button>
          <Button
            type='button'
            size='icon'
            variant='outline'
            className='group shrink-0 border-violet-400/60 bg-violet-50/90 shadow-sm hover:border-fuchsia-400 hover:bg-fuchsia-50 dark:border-violet-400/40 dark:bg-violet-400/10 dark:hover:bg-fuchsia-400/15'
            title={`生成 ${passwordLength} 位随机密码`}
            aria-label={`生成 ${passwordLength} 位随机密码`}
            disabled={!selectedPlatform}
            onClick={() => {
              setDiceRolling(true)
              window.setTimeout(() => setDiceRolling(false), 450)
              setForm({
                ...form,
                password: randomParkingPassword(passwordLength),
              })
              setPasswordVisible(true)
            }}
          >
            <span
              className={`relative block size-5 transition-transform group-active:scale-90 ${diceRolling ? 'motion-safe:animate-[spin_420ms_cubic-bezier(0.22,1,0.36,1)]' : ''}`}
              aria-hidden='true'
            >
              <Dices className='absolute inset-0 size-5 text-fuchsia-500' />
              <Dices className='absolute inset-0 size-5 text-cyan-500 [clip-path:inset(0_0_48%_0)]' />
            </span>
          </Button>
        </div>
      </Field>
      <Field label='车位数量' htmlFor={`${idPrefix}-slots`}>
        <Input
          id={`${idPrefix}-slots`}
          type='number'
          min='1'
          value={form.total_slots}
          max='100'
          onChange={(event) =>
            setForm({ ...form, total_slots: event.target.value })
          }
        />
      </Field>
      <Field label='平台续费日' htmlFor={`${idPrefix}-billing-day`}>
        <Input
          id={`${idPrefix}-billing-day`}
          type='number'
          min='1'
          max='31'
          value={form.billing_day}
          onChange={(event) =>
            setForm({ ...form, billing_day: event.target.value })
          }
        />
      </Field>
      <Field label='卡尾号' htmlFor={`${idPrefix}-card-last4`}>
        <Input
          id={`${idPrefix}-card-last4`}
          value={form.card_last4}
          inputMode='numeric'
          maxLength={4}
          onChange={(event) =>
            setForm({
              ...form,
              card_last4: event.target.value.replace(/\D/g, '').slice(0, 4),
            })
          }
          placeholder='选填，4 位数字'
        />
      </Field>
      <Field label='默认月费' htmlFor={`${idPrefix}-price`}>
        <Input
          id={`${idPrefix}-price`}
          type='number'
          min='0'
          step='0.01'
          value={form.monthly_price}
          placeholder='0.00'
          onChange={(event) =>
            setForm({ ...form, monthly_price: event.target.value })
          }
        />
      </Field>
      <Field label='备注' htmlFor={`${idPrefix}-note`} wide>
        <Textarea
          id={`${idPrefix}-note`}
          className='min-h-20 resize-y'
          value={form.note}
          onChange={(event) => setForm({ ...form, note: event.target.value })}
        />
      </Field>
    </FormGrid>
  )
}

function SeatStatus({ space }) {
  if (space.status !== 'active') {
    return <Badge variant='secondary'>已停用</Badge>
  }
  const total = Math.max(Number(space.total_slots || 0), 0)
  const used = Math.max(Number(space.member_count || 0), 0)
  const remaining = Math.max(total - used, 0)
  if (remaining === 0 && total > 0) {
    return (
      <Badge className='border-emerald-500/15 bg-emerald-500/10 text-emerald-700 dark:text-emerald-400'>
        已满
      </Badge>
    )
  }
  if (used === 0) {
    return (
      <Badge className='border-red-500/15 bg-red-500/10 text-red-700 dark:text-red-400'>
        全部空闲
      </Badge>
    )
  }
  return (
    <Badge className='border-orange-500/15 bg-orange-500/10 text-orange-700 dark:text-orange-400'>
      剩余 {remaining} 位
    </Badge>
  )
}

function SimpleTable({ rows, columns, empty, compact = false }) {
  if (!rows.length)
    return (
      <div className='text-muted-foreground flex min-h-32 items-center justify-center text-center text-sm'>
        {empty}
      </div>
    )
  return (
    <div className='overflow-x-auto'>
      <table
        className={
          compact
            ? 'w-full min-w-[760px] text-xs'
            : 'w-full min-w-[820px] text-sm'
        }
      >
        <thead>
          <tr className='text-muted-foreground border-b text-left'>
            {columns.map(([header], columnIndex) => (
              <th
                key={columnIndex}
                className={
                  compact ? 'px-3 py-3 font-medium' : 'px-5 py-3 font-medium'
                }
              >
                {header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row, rowIndex) => (
            <tr
              key={row.id}
              className='hover:bg-muted/25 border-b transition-colors last:border-0'
            >
              {columns.map(([, render], columnIndex) => (
                <td
                  key={columnIndex}
                  className={
                    compact
                      ? 'px-3 py-3 align-middle'
                      : 'px-5 py-3.5 align-middle'
                  }
                >
                  {render(row, rowIndex)}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function Field({ label, children, wide, htmlFor }) {
  return (
    <div className={wide ? 'space-y-2 sm:col-span-2' : 'space-y-2'}>
      <Label htmlFor={htmlFor}>{label}</Label>
      {children}
    </div>
  )
}

function PlatformIcon({ platform, className = 'size-7' }) {
  if (platform?.icon) {
    return (
      <img
        src={platform.icon}
        alt=''
        className={`${className} shrink-0 object-contain p-0.5`}
      />
    )
  }
  return (
    <span
      aria-hidden='true'
      className={`${className} bg-primary/10 text-primary flex shrink-0 items-center justify-center border text-xs font-bold`}
    >
      {String(platform?.name || '?')
        .trim()
        .slice(0, 1)
        .toUpperCase()}
    </span>
  )
}

function SortablePlatformRow({
  platform,
  sorting,
  onEdit,
  onDelete,
  deleting,
}) {
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id: String(platform.id), disabled: !sorting })
  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.55 : 1,
  }
  return (
    <tr ref={setNodeRef} style={style} className='border-b last:border-0'>
      {sorting && (
        <td className='px-2 py-2'>
          <button
            type='button'
            className='text-muted-foreground hover:bg-accent flex size-8 cursor-grab touch-none items-center justify-center active:cursor-grabbing'
            title={`拖动${platform.name}`}
            aria-label={`拖动${platform.name}调整顺序`}
            {...attributes}
            {...listeners}
          >
            <GripVertical className='size-5' />
          </button>
        </td>
      )}
      <td className='px-3 py-2'>
        <span className='flex items-center gap-2 font-medium'>
          <PlatformIcon platform={platform} />
          {platform.name}
        </span>
      </td>
      <td className='px-3 py-2 tabular-nums'>{platform.space_count || 0}</td>
      <td className='px-3 py-2 tabular-nums'>{platform.total_slots || 0}</td>
      <td className='px-3 py-2 tabular-nums'>{platform.default_slots || 1}</td>
      <td className='px-3 py-2 tabular-nums'>
        {platform.password_length || 8}
      </td>
      <td className='px-3 py-2'>
        <Badge variant={platform.status === 'active' ? 'default' : 'secondary'}>
          {platform.status === 'active' ? '启用' : '暂停'}
        </Badge>
      </td>
      <td className='max-w-48 truncate px-3 py-2'>{platform.note || '-'}</td>
      <td className='px-3 py-2'>
        <div className='flex justify-end gap-1.5'>
          <Button
            size='icon'
            variant='outline'
            title='编辑平台'
            aria-label={`编辑${platform.name}`}
            disabled={sorting}
            onClick={onEdit}
          >
            <Pencil className='size-3.5' />
          </Button>
          <Button
            size='icon'
            variant='outline'
            title='删除平台'
            aria-label={`删除${platform.name}`}
            disabled={sorting || platform.space_count > 0 || deleting}
            onClick={onDelete}
          >
            <Trash2 className='size-3.5' />
          </Button>
        </div>
      </td>
    </tr>
  )
}

function PlatformFilterBar({ value, onChange, platforms, counts, total }) {
  const [visibleCount, setVisibleCount] = useState(3)
  useEffect(() => {
    const update = () => {
      const width = window.innerWidth
      setVisibleCount(
        width >= 1500 ? 8 : width >= 1200 ? 5 : width >= 900 ? 3 : 1
      )
    }
    update()
    window.addEventListener('resize', update)
    return () => window.removeEventListener('resize', update)
  }, [])
  const countMap = new Map(counts)
  const shown = platforms.slice(0, visibleCount)
  const hidden = platforms.slice(visibleCount)
  return (
    <div className='flex min-w-0 flex-wrap items-center gap-1.5'>
      <Button
        type='button'
        size='sm'
        variant='ghost'
        className={
          value === 'all'
            ? 'bg-[var(--accent-brand-soft)] text-[var(--accent-brand)] hover:bg-[var(--accent-brand-soft)] hover:text-[var(--accent-brand)]'
            : undefined
        }
        onClick={() => onChange('all')}
      >
        全部 <span className='text-xs opacity-70'>{total}</span>
      </Button>
      {shown.map((platform) => (
        <Button
          type='button'
          size='sm'
          variant='ghost'
          title={platform.name}
          className={`max-w-44 ${
            value === platform.name
              ? 'bg-[var(--accent-brand-soft)] text-[var(--accent-brand)] hover:bg-[var(--accent-brand-soft)] hover:text-[var(--accent-brand)]'
              : ''
          }`}
          key={platform.id}
          onClick={() => onChange(platform.name)}
        >
          <PlatformIcon platform={platform} className='size-5' />
          <span className='truncate'>{platform.name}</span>
          <span className='text-xs opacity-70'>
            {countMap.get(platform.name) || 0}
          </span>
        </Button>
      ))}
      {hidden.length > 0 && (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              size='icon'
              variant='outline'
              aria-label='更多平台'
              title='更多平台'
            >
              <MoreHorizontal className='size-4' />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align='start'>
            {hidden.map((platform) => (
              <DropdownMenuItem
                key={platform.id}
                onClick={() => onChange(platform.name)}
              >
                <PlatformIcon platform={platform} className='size-5' />
                {platform.name}
                <span className='ml-auto text-xs opacity-70'>
                  {countMap.get(platform.name) || 0}
                </span>
              </DropdownMenuItem>
            ))}
          </DropdownMenuContent>
        </DropdownMenu>
      )}
    </div>
  )
}

function FormGrid({ children }) {
  return <div className='grid gap-3 sm:grid-cols-2'>{children}</div>
}

function ContactDisplay({ member }) {
  const contact = member.contact || member.telegram || ''
  const type = member.contact_type || (member.telegram ? 'telegram' : 'wechat')
  if (!contact) return <span className='text-muted-foreground'>-</span>

  const icon =
    type === 'telegram' ? (
      <Send className='size-4 text-[#229ed9]' />
    ) : type === 'nodeseek' ? (
      <span className='flex size-4 items-center justify-center rounded-full bg-black text-[10px] font-bold text-white dark:bg-white dark:text-black'>
        N
      </span>
    ) : (
      <MessageCircle className='size-4 text-[#07c160]' />
    )

  return (
    <div className='flex min-w-0 items-center gap-2'>
      {icon}
      <span className='truncate'>{contact}</span>
    </div>
  )
}

function ContactTypeButtons({ value, onChange }) {
  const items = [
    {
      value: 'wechat',
      label: '微信',
      icon: <MessageCircle className='size-4' />,
      color: 'text-[#07c160]',
    },
    {
      value: 'telegram',
      label: 'Telegram',
      icon: <Send className='size-4' />,
      color: 'text-[#229ed9]',
    },
    {
      value: 'nodeseek',
      label: 'N 论坛',
      icon: (
        <span className='flex size-4 items-center justify-center rounded-full bg-black text-[10px] font-bold text-white dark:bg-white dark:text-black'>
          N
        </span>
      ),
      color: '',
    },
  ]
  return (
    <div className='border-border flex shrink-0 border-r'>
      {items.map((item) => (
        <button
          key={item.value}
          type='button'
          title={item.label}
          aria-label={`联系方式：${item.label}`}
          aria-pressed={value === item.value}
          onClick={() => onChange(item.value)}
          className={`hover:bg-muted flex size-9 items-center justify-center border-r last:border-r-0 ${item.color} ${value === item.value ? 'bg-primary/10 ring-primary/40 ring-1 ring-inset' : ''}`}
        >
          {item.icon}
        </button>
      ))}
    </div>
  )
}

function PeriodButtons({ value, onChange }) {
  return (
    <div className='bg-muted/35 grid grid-cols-4 gap-1 border p-1'>
      {[
        [1, '1 月'],
        [3, '季付'],
        [6, '半年付'],
        [12, '年付'],
      ].map(([months, label]) => (
        <Button
          key={months}
          type='button'
          size='sm'
          variant={Number(value) === months ? 'default' : 'ghost'}
          onClick={() => onChange(months)}
        >
          {label}
        </Button>
      ))}
    </div>
  )
}

function inputDate(value) {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  date.setMinutes(date.getMinutes() - date.getTimezoneOffset())
  return date.toISOString().slice(0, 10)
}

function exportRenewals(rows) {
  const headers = [
    '续费时间',
    '车友',
    '车位',
    '原到期',
    '新到期',
    '月数',
    '金额',
    '支付方式',
    '操作人',
    '备注',
  ]
  const body = rows.map((record) => [
    formatDate(record.created_at),
    record.member_name || '',
    record.space_name || '',
    formatDate(record.old_expire_date),
    formatDate(record.new_expire_date),
    record.months || '',
    Number(record.amount || 0).toFixed(2),
    record.payment || '',
    record.operator || '',
    record.note || '',
  ])
  const csv = [headers, ...body]
    .map((row) => row.map(csvCell).join(','))
    .join('\n')
  const blob = new Blob([`\uFEFF${csv}`], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `续费记录-${todayValue()}.csv`
  document.body.appendChild(link)
  link.click()
  link.remove()
  URL.revokeObjectURL(url)
}

function csvCell(value) {
  const text = String(value ?? '')
  return `"${text.replace(/"/g, '""')}"`
}

function formatDate(value) {
  if (!value) return '-'
  return new Date(value).toLocaleDateString('zh-CN')
}

function dayDiff(value) {
  if (!value) return 9999
  const end = new Date(value)
  const start = new Date()
  start.setHours(0, 0, 0, 0)
  end.setHours(0, 0, 0, 0)
  return Math.ceil((end.getTime() - start.getTime()) / 86400000)
}

function ExpiryBadge({ date }) {
  const days = dayDiff(date)
  if (days < 0)
    return <Badge variant='destructive'>已过期 {Math.abs(days)} 天</Badge>
  if (days === 0) return <Badge variant='destructive'>今日到期</Badge>
  if (days <= 7) return <Badge>剩 {days} 天</Badge>
  if (days <= 30) return <Badge variant='secondary'>剩 {days} 天</Badge>
  return <Badge variant='outline'>{formatDate(date)}</Badge>
}
