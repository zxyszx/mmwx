// 代理节点类型定义。
// 节点 URI / Surge 行解析已统一交后端共享模块 proxyparser 处理,
// 这里只保留前端仍需引用的结构类型。

export interface ProxyNode {
  name: string
  type: string
  server: string
  port: number
  password?: string
  uuid?: string
  method?: string
  cipher?: string
  [key: string]: unknown
  'spider-x'?: string
}

// Clash 节点格式
export interface ClashProxy {
  name: string
  type: string
  server: string
  port: number
  [key: string]: unknown
}
