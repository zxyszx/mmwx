import { ExternalLink, Check, Activity, Network, Users, Server, Lock, Package, Radar, Zap, LayoutTemplate, FileCode, Shield, Globe, Sparkles } from 'lucide-react'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Button } from '@/components/ui/button'

const TABLE_ROWS = [
  { icon: Activity, feature: '流量管理', mmw: '探针读取流量，无法精确统计', mmwx: 'Agent 精确统计，到期自动失效' },
  { icon: Network, feature: '节点管理', mmw: '手动导入 / 外部订阅同步', mmwx: '页面创建 + 偷自己 / 路由出站 / Tunnel' },
  { icon: Users, feature: '用户管理', mmw: '基础用户管理，到期后本地配置仍可用', mmwx: '套餐绑定，到期自动删除节点权限' },
  { icon: Server, feature: '服务器管理', mmw: '不支持', mmwx: 'Master-Agent 远程管理' },
  { icon: Lock, feature: '证书管理', mmw: '不支持', mmwx: 'ACME 自动申请 / 续期 / 部署' },
  { icon: Package, feature: '套餐管理', mmw: '不支持', mmwx: '流量配额 / 有效期 / 限速' },
  { icon: Radar, feature: '监控探针', mmw: '探针流量采集，节点级监控', mmwx: '精确流量统计 + 实时在线用户追踪' },
  { icon: Zap, feature: '订阅生成', mmw: '12+ 客户端格式', mmwx: '12+ 客户端格式（继承）' },
  { icon: LayoutTemplate, feature: '模板系统', mmw: 'V3 模板引擎', mmwx: 'V3 模板引擎（继承）' },
  { icon: FileCode, feature: '自定义规则', mmw: 'DNS / 分流 / 规则集', mmwx: 'DNS / 分流 / 规则集（继承）' },
  { icon: Shield, feature: '安全功能', mmw: '静默模式 / TOTP 双因素', mmwx: '静默模式 / TOTP 双因素（继承）' },
  { icon: Globe, feature: '部署方式', mmw: 'Docker / 二进制', mmwx: 'Docker / 二进制（Master + Agent）' },
] as const

const EXCLUSIVE_FEATURES = [
  { title: '内嵌 Xray (PRO)', desc: '内置 Xray 核心，支持限速、设备数限制、自动限流、在线追踪等高级功能。' },
  { title: '路由出站', desc: '支持整节点级别和单用户级别的出站路由配置。' },
  { title: '共享服务器', desc: 'Owner-Consumer 模型，加密通信，Consumer 拥有受限权限。' },
  { title: '证书管理', desc: 'ACME 自动申请和续期 TLS 证书，支持多 DNS 提供商。' },
  { title: '套餐管理', desc: '创建流量套餐，配置流量配额、有效期、可用节点、限速策略。' },
  { title: 'Nginx 管理', desc: '远程安装/卸载 Nginx，管理配置文件，SSL 证书部署。' },
]

const SHARED_FEATURES = [
  '支持 12+ 客户端格式的订阅生成',
  'V3 模板引擎，灵活的订阅配置模板',
  '自定义 DNS、分流规则、规则集管理',
  '外部订阅同步（定时自动更新）',
  '链式代理（多层中转加速）',
  '静默模式和 TOTP 双因素认证',
  'Telegram 通知推送和每日报告',
  '备份与恢复功能',
  '覆写脚本（JavaScript Hooks）',
  'Docker 和二进制部署方式',
]

export function MmwxDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (v: boolean) => void }) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-3xl max-h-[85vh] flex flex-col p-0'>
        <DialogHeader className='px-6 pt-6 pb-0 shrink-0'>
          <DialogTitle className='text-xl'>与妙妙屋的区别</DialogTitle>
          <div className='flex gap-2 pt-3'>
            <Button size='sm' variant='outline' asChild>
              <a href='https://miaomiaowu.net/x' target='_blank' rel='noopener noreferrer' className='flex items-center gap-1.5'>
                <Sparkles className='size-3.5' /> 妙妙屋X介绍 <ExternalLink className='size-3' />
              </a>
            </Button>
            <Button size='sm' asChild>
              <a href='https://miaomiaowu.net/x/docs/upgrade-from-mmw' target='_blank' rel='noopener noreferrer' className='flex items-center gap-1.5'>
                体验迁移流程 <ExternalLink className='size-3' />
              </a>
            </Button>
          </div>
        </DialogHeader>

        <ScrollArea className='flex-1 px-6 pb-6'>
          <div className='space-y-6 pt-4'>
            {/* 概述 */}
            <div className='rounded-md border bg-muted/30 p-4 text-sm text-muted-foreground space-y-2'>
              <p>妙妙屋X 是妙妙屋的增强版本，继承了妙妙屋的全部核心功能（订阅生成、模板系统、自定义规则等），并在此基础上新增了远程服务器管理、Xray 完整管理、证书管理、套餐管理等企业级功能。</p>
              <p>简单来说：妙妙屋是订阅管理平台，妙妙屋X 是完整的代理服务管理平台。</p>
            </div>

            {/* 对比表格 */}
            <div>
              <h3 className='text-base font-semibold mb-3'>功能对比总览</h3>
              <div className='overflow-x-auto'>
                <table className='w-full text-sm'>
                  <thead>
                    <tr className='border-b bg-muted/30'>
                      <th className='text-left py-2 px-3 font-semibold w-1/4'>功能维度</th>
                      <th className='text-center py-2 px-3 font-semibold w-[37.5%]'>妙妙屋</th>
                      <th className='text-center py-2 px-3 font-semibold w-[37.5%]'>妙妙屋X</th>
                    </tr>
                  </thead>
                  <tbody>
                    {TABLE_ROWS.map((row, i) => (
                      <tr key={i} className={i < TABLE_ROWS.length - 1 ? 'border-b' : ''}>
                        <td className='py-2 px-3 font-medium'>
                          <div className='flex items-center gap-1.5'>
                            <row.icon className='size-3.5 text-primary shrink-0' />
                            {row.feature}
                          </div>
                        </td>
                        <td className='text-center py-2 px-3 text-muted-foreground text-xs'>{row.mmw}</td>
                        <td className='text-center py-2 px-3 text-xs'>{row.mmwx}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>

            {/* 独有功能 */}
            <div>
              <h3 className='text-base font-semibold mb-3 flex items-center gap-2'>
                <Sparkles className='size-4 text-primary' /> 妙妙屋X 独有功能
              </h3>
              <div className='grid gap-3 sm:grid-cols-2'>
                {EXCLUSIVE_FEATURES.map((f, i) => (
                  <div key={i} className='rounded-md border p-3'>
                    <div className='font-medium text-sm mb-1'>{f.title}</div>
                    <p className='text-xs text-muted-foreground'>{f.desc}</p>
                  </div>
                ))}
              </div>
            </div>

            {/* 共同功能 */}
            <div>
              <h3 className='text-base font-semibold mb-3'>共同功能</h3>
              <div className='rounded-md border p-4'>
                <p className='text-xs text-muted-foreground mb-3'>妙妙屋X 继承了妙妙屋的全部核心功能，以下功能在两个版本中完全一致：</p>
                <div className='grid gap-1.5 sm:grid-cols-2'>
                  {SHARED_FEATURES.map((f, i) => (
                    <div key={i} className='flex items-center gap-1.5 text-xs'>
                      <Check className='size-3.5 text-green-500 shrink-0' />
                      <span>{f}</span>
                    </div>
                  ))}
                </div>
              </div>
            </div>

            {/* 总结 */}
            <div className='rounded-md border bg-muted/30 p-4 text-xs text-muted-foreground space-y-2'>
              <p>如果你只需要管理订阅和节点导入，妙妙屋已经足够满足需求。它简单轻量，适合个人使用。</p>
              <p>如果你需要管理多台远程服务器、精确控制用户流量和权限、自动化证书管理，或者需要通过页面配置 Xray 而无需手动登录服务器，那么妙妙屋X 是更好的选择。</p>
            </div>
          </div>
        </ScrollArea>
      </DialogContent>
    </Dialog>
  )
}
