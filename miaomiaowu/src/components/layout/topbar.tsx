import { useEffect, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import {
  Bell,
  ChevronLeft,
  ChevronRight,
  CreditCard,
  LayoutDashboard,
  Layers3,
  Menu,
  MoreHorizontal,
  PanelLeft,
  PanelTop,
  ScrollText,
  Server,
  Users,
  type LucideIcon,
} from 'lucide-react'
import { useAuthStore } from '@/stores/auth-store'
import { getCookie, setCookie } from '@/lib/cookies'
import { profileQueryFn } from '@/lib/profile'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { NavIcon } from '@/components/layout/nav-icon'
import { ThemeSwitch } from '@/components/theme-switch'
import { UserMenu } from './user-menu'

const NAV_LAYOUT_COOKIE = 'mmw-nav-layout'
const NAV_COLLAPSED_COOKIE = 'mmw-sidebar-collapsed'
const COOKIE_YEAR = 60 * 60 * 24 * 365

type NavLayout = 'top' | 'sidebar'

type NavItem = {
  id: string
  title: string
  to: string
  search?: { tab: string }
  icon: LucideIcon
  admin?: boolean
}

const navItems: NavItem[] = [
  { id: 'overview', title: '总览', to: '/', icon: LayoutDashboard },
  {
    id: 'platforms',
    title: '平台',
    to: '/parking',
    search: { tab: 'platforms' },
    icon: Server,
  },
  {
    id: 'spaces',
    title: '合租',
    to: '/parking',
    search: { tab: 'spaces' },
    icon: Layers3,
  },
  {
    id: 'members',
    title: '成员',
    to: '/parking',
    search: { tab: 'members' },
    icon: Users,
  },
  {
    id: 'reminders',
    title: '订阅',
    to: '/parking',
    search: { tab: 'reminders' },
    icon: Bell,
    admin: true,
  },
  {
    id: 'renewals',
    title: '账单',
    to: '/parking',
    search: { tab: 'renewals' },
    icon: CreditCard,
    admin: true,
  },
  { id: 'logs', title: '日志', to: '/logs', icon: ScrollText, admin: true },
]

function readNavLayout(): NavLayout {
  return getCookie(NAV_LAYOUT_COOKIE) === 'sidebar' ? 'sidebar' : 'top'
}

export function Topbar() {
  const { auth } = useAuthStore()
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false)
  const [layout, setLayoutState] = useState<NavLayout>(() => readNavLayout())
  const [sidebarCollapsed, setSidebarCollapsed] = useState(
    () => getCookie(NAV_COLLAPSED_COOKIE) === 'true'
  )
  const [topVisibleCount, setTopVisibleCount] = useState(3)

  const { data: profile } = useQuery({
    queryKey: ['profile'],
    queryFn: profileQueryFn,
    enabled: Boolean(auth.accessToken),
    staleTime: 5 * 60 * 1000,
  })

  const isAdmin = Boolean(profile?.is_admin)
  const allowedItems = useMemo(
    () => navItems.filter((item) => !item.admin || isAdmin),
    [isAdmin]
  )

  useEffect(() => {
    document.documentElement.dataset.navLayout = layout
    document.documentElement.dataset.sidebarCollapsed = sidebarCollapsed
      ? 'true'
      : 'false'
    return () => {
      delete document.documentElement.dataset.navLayout
      delete document.documentElement.dataset.sidebarCollapsed
    }
  }, [layout, sidebarCollapsed])

  const visibleItems = allowedItems
  const topItems = visibleItems.slice(0, topVisibleCount)
  const overflowItems = visibleItems.slice(topVisibleCount)

  useEffect(() => {
    const updateVisibleCount = () => {
      const width = window.innerWidth
      if (width >= 1100) setTopVisibleCount(visibleItems.length)
      else if (width >= 900) setTopVisibleCount(visibleItems.length)
      else setTopVisibleCount(2)
    }
    updateVisibleCount()
    window.addEventListener('resize', updateVisibleCount)
    return () => window.removeEventListener('resize', updateVisibleCount)
  }, [visibleItems.length])

  const setLayout = (next: NavLayout) => {
    setLayoutState(next)
    setCookie(NAV_LAYOUT_COOKIE, next, COOKIE_YEAR)
  }

  const toggleSidebarLayout = () => {
    const next = layout === 'top' ? 'sidebar' : 'top'
    if (next === 'sidebar' && sidebarCollapsed) {
      setSidebarCollapsed(false)
      setCookie(NAV_COLLAPSED_COOKIE, 'false', COOKIE_YEAR)
    }
    setLayout(next)
  }

  const toggleCollapsed = () => {
    const next = !sidebarCollapsed
    setSidebarCollapsed(next)
    setCookie(NAV_COLLAPSED_COOKIE, String(next), COOKIE_YEAR)
  }

  return (
    <>
      <header className='app-topbar fixed top-0 right-0 left-0 z-50 h-[58px] border-b border-[var(--divider)] bg-[var(--bg-header)] backdrop-blur-[10px]'>
        <div className='app-topbar-inner grid h-full w-full grid-cols-[minmax(0,1fr)_auto] items-center overflow-hidden px-4 sm:px-6 md:grid-cols-[190px_minmax(0,1fr)_190px]'>
          <div className='flex min-w-0 items-center gap-3'>
            <BrandMark />

            {layout === 'sidebar' && (
              <Button
                variant='outline'
                size='icon'
                aria-label={sidebarCollapsed ? '展开侧边栏' : '收纳侧边栏'}
                aria-controls='app-sidebar-navigation'
                aria-expanded={!sidebarCollapsed}
                title={sidebarCollapsed ? '展开侧边栏' : '收纳侧边栏'}
                className='app-header-control hidden h-8 w-8 shrink-0 md:inline-flex'
                onClick={toggleCollapsed}
              >
                {sidebarCollapsed ? (
                  <ChevronRight className='size-[18px]' />
                ) : (
                  <ChevronLeft className='size-[18px]' />
                )}
              </Button>
            )}

            {visibleItems.length > 0 && (
              <MobileNavMenu
                open={mobileMenuOpen}
                onOpenChange={setMobileMenuOpen}
                items={visibleItems}
              />
            )}
          </div>

          <div className='hidden min-w-0 justify-center md:flex'>
            {layout === 'top' && (
              <nav className='flex min-w-0 items-center justify-center gap-1 lg:gap-2'>
                {topItems.map((item) => (
                  <NavLink key={item.id} item={item} />
                ))}
                {overflowItems.length > 0 && (
                  <OverflowNavMenu items={overflowItems} />
                )}
              </nav>
            )}
          </div>

          <div className='flex items-center justify-end gap-1.5 pl-2 sm:pl-0'>
            <Button
              variant='outline'
              size='icon'
              aria-label={
                layout === 'top' ? '切换到侧边导航栏' : '切换到顶部导航栏'
              }
              title={layout === 'top' ? '切换到侧边导航栏' : '切换到顶部导航栏'}
              className='app-header-control hidden h-8 w-8 md:inline-flex'
              onClick={toggleSidebarLayout}
            >
              {layout === 'top' ? (
                <PanelLeft className='size-[18px]' />
              ) : (
                <PanelTop className='size-[18px]' />
              )}
            </Button>
            <ThemeSwitch />
            <UserMenu />
          </div>
        </div>
      </header>

      {layout === 'sidebar' && (
        <aside
          id='app-sidebar-navigation'
          aria-label='主导航'
          className={`app-sidebar fixed top-[58px] bottom-0 left-0 z-40 hidden overflow-y-auto border-r border-[var(--divider)] bg-[var(--bg-header)] backdrop-blur-[10px] transition-[width] duration-200 ease-out md:flex ${
            sidebarCollapsed ? 'w-[4.25rem]' : 'w-56'
          }`}
        >
          <nav className='flex w-full flex-col gap-2 p-3 pb-6'>
            {visibleItems.map((item) => (
              <NavLink
                key={item.id}
                item={item}
                iconOnly={sidebarCollapsed}
                sidebar
              />
            ))}
          </nav>
        </aside>
      )}
    </>
  )
}

function BrandMark() {
  return (
    <Link
      to='/'
      className='hover:text-primary flex shrink-0 items-center gap-2.5 text-lg font-semibold tracking-tight transition outline-none focus:outline-none'
    >
      <img
        src={`${import.meta.env.BASE_URL}images/logo.webp`}
        alt='妙妙屋 Logo'
        className='h-7 w-7 shrink-0 rounded-lg object-cover ring-1 ring-[var(--border-subtle)]'
      />
      <span className='hidden text-[13px] font-semibold whitespace-nowrap text-[var(--text-primary)] md:inline'>
        妙妙屋
      </span>
    </Link>
  )
}

function NavLink({
  item,
  iconOnly,
  sidebar,
  responsive,
}: {
  item: NavItem
  iconOnly?: boolean
  sidebar?: boolean
  responsive?: boolean
}) {
  const Icon = item.icon
  return (
    <Link
      to={item.to}
      search={item.search}
      activeOptions={{ exact: true, includeSearch: true }}
      data-nav-item
      aria-label={item.title}
      title={item.title}
      className={`nav-motion relative inline-flex h-10 items-center gap-2 border-0 bg-transparent px-2.5 py-2 text-[13px] font-medium whitespace-nowrap text-[var(--text-secondary)] transition-colors duration-150 hover:text-[var(--text-primary)] ${
        iconOnly
          ? 'w-9 justify-center px-2'
          : responsive
            ? 'w-9 justify-center px-2 min-[1600px]:w-auto min-[1600px]:justify-start min-[1600px]:px-3'
            : sidebar
              ? 'w-full justify-center px-3'
              : 'justify-start px-3'
      }`}
      activeProps={{
        'data-status': 'active',
        className: 'text-[var(--text-primary)]',
      }}
    >
      {(sidebar || iconOnly) && (
        <NavIcon icon={Icon} to={item.to} className='size-[18px] shrink-0' />
      )}
      {!iconOnly && (
        <span className={responsive ? 'hidden min-[1600px]:inline' : undefined}>
          {item.title}
        </span>
      )}
    </Link>
  )
}

function MobileNavMenu({
  open,
  onOpenChange,
  items,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  items: NavItem[]
}) {
  return (
    <DropdownMenu open={open} onOpenChange={onOpenChange}>
      <DropdownMenuTrigger asChild>
        <Button
          variant='outline'
          size='icon'
          className='app-header-control h-9 w-9 md:hidden'
        >
          <Menu className='h-5 w-5' />
          <span className='sr-only'>打开菜单</span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align='start' className='w-48 rounded-xl p-1'>
        {items.map((item) => {
          const Icon = item.icon
          return (
            <DropdownMenuItem key={item.id} asChild>
              <Link
                to={item.to}
                search={item.search}
                activeOptions={{ exact: true, includeSearch: true }}
                className='hover:bg-accent/35 focus:bg-accent/35 flex cursor-pointer items-center gap-3 px-3 py-2'
                onClick={() => onOpenChange(false)}
              >
                <NavIcon
                  icon={Icon}
                  to={item.to}
                  className='size-[18px] shrink-0'
                />
                <span>{item.title}</span>
              </Link>
            </DropdownMenuItem>
          )
        })}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

function OverflowNavMenu({ items }: { items: NavItem[] }) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant='outline'
          size='icon'
          className='app-header-control h-8 w-8 shrink-0'
          aria-label='更多功能'
          title='更多功能'
        >
          <MoreHorizontal className='size-5' />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align='start' className='w-52 rounded-xl p-1'>
        {items.map((item) => {
          const Icon = item.icon
          return (
            <DropdownMenuItem key={item.id} asChild>
              <Link
                to={item.to}
                search={item.search}
                activeOptions={{ exact: true, includeSearch: true }}
                className='hover:bg-accent/35 focus:bg-accent/35 flex cursor-pointer items-center gap-3 px-3 py-2'
              >
                <NavIcon
                  icon={Icon}
                  to={item.to}
                  className='size-[18px] shrink-0'
                />
                <span>{item.title}</span>
              </Link>
            </DropdownMenuItem>
          )
        })}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
