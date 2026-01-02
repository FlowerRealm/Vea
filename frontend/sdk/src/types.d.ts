/**
 * Vea SDK TypeScript 类型定义
 * 基于 OpenAPI 规范生成
 */

// ============================================================================
// 数据模型
// ============================================================================

export type NodeProtocol = 'vless' | 'trojan' | 'shadowsocks' | 'vmess' | 'hysteria2' | 'tuic'

export interface NodeSecurity {
  uuid?: string
  password?: string
  method?: string
  flow?: string
  encryption?: string
  alterId?: number
  plugin?: string
  pluginOpts?: string
  alpn?: string[]
}

export interface NodeTransport {
  type?: string
  host?: string
  path?: string
  serviceName?: string
  headers?: Record<string, string>
}

export interface NodeTLS {
  enabled?: boolean
  type?: string
  serverName?: string
  insecure?: boolean
  fingerprint?: string
  realityPublicKey?: string
  realityShortId?: string
  alpn?: string[]
}

export interface Node {
  id: string
  name: string
  address: string
  port: number
  protocol: NodeProtocol
  tags?: string[]
  security?: NodeSecurity
  transport?: NodeTransport
  tls?: NodeTLS
  sourceConfigId?: string
  lastLatencyMs: number
  lastLatencyAt: string
  lastLatencyError?: string
  lastSpeedMbps: number
  lastSpeedAt: string
  lastSpeedError?: string
  createdAt: string
  updatedAt: string
}

export interface ChainProxySettings {
  edges: any[]
  positions?: Record<string, { x: number; y: number }>
  slots?: any[]
  updatedAt?: string
}

export interface FRouterGraphNodeInfo {
  id: string
  name: string
  protocol: string
  address: string
  port: number
}

export interface FRouterGraphRequest {
  edges: any[]
  positions?: Record<string, { x: number; y: number }>
  slots?: any[]
}

export interface FRouterGraphResponse {
  edges: any[]
  positions?: Record<string, { x: number; y: number }>
  slots?: any[]
  nodes: FRouterGraphNodeInfo[]
  updatedAt?: string
}

export interface ValidateGraphResponse {
  valid: boolean
  errors: string[]
  warnings: string[]
}

export interface FRouter {
  id: string
  name: string
  chainProxy: ChainProxySettings
  tags?: string[]
  lastLatencyMs: number
  lastLatencyAt: string
  lastLatencyError?: string
  lastSpeedMbps: number
  lastSpeedAt: string
  lastSpeedError?: string
  createdAt: string
  updatedAt: string
}

export type ConfigFormat = 'xray-json'

export interface Config {
  id: string
  name: string
  format: ConfigFormat
  payload: string
  sourceUrl?: string
  checksum?: string
  lastSyncError?: string
  autoUpdateInterval: number
  lastSyncedAt: string
  expireAt?: string | null
  createdAt: string
  updatedAt: string
}

export type GeoResourceType = 'geoip' | 'geosite'

export interface GeoResource {
  id: string
  name: string
  type: GeoResourceType
  sourceUrl: string
  checksum: string
  version: string
  artifactPath?: string
  fileSizeBytes?: number
  lastSyncError?: string
  lastSynced: string
  createdAt: string
  updatedAt: string
}

export type CoreComponentKind = 'xray' | 'singbox' | 'geo' | 'generic'

export interface CoreComponent {
  id: string
  name: string
  kind: CoreComponentKind
  sourceUrl: string
  archiveType: string
  lastInstalledAt: string
  installDir: string
  lastVersion: string
  checksum: string
  lastSyncError: string
  meta?: Record<string, string>
  accessories?: string[]
  installStatus?: string
  installProgress?: number
  installMessage?: string
  createdAt: string
  updatedAt: string
}

export interface SystemProxySettings {
  enabled: boolean
  ignoreHosts: string[]
  updatedAt: string
}

export type InboundMode = 'socks' | 'http' | 'mixed' | 'tun'

export type CoreEngineKind = 'xray' | 'singbox' | 'auto'

export interface CoreEngineInfo {
  kind: CoreEngineKind
  binaryPath: string
  version: string
  capabilities: string[]
  installed: boolean
}

export interface ProxyConfig {
  inboundMode: InboundMode
  inboundPort: number
  inboundConfig?: Record<string, any>
  tunSettings?: Record<string, any>
  resolvedService?: Record<string, any>
  dnsConfig?: Record<string, any>
  logConfig?: Record<string, any>
  performanceConfig?: Record<string, any>
  xrayConfig?: Record<string, any>
  preferredEngine: CoreEngineKind
  frouterId: string
  updatedAt: string
}

export interface ServiceState {
  schemaVersion?: string
  nodes: Node[]
  frouters: FRouter[]
  configs: Config[]
  geoResources: GeoResource[]
  components: CoreComponent[]
  systemProxy: SystemProxySettings
  proxyConfig: ProxyConfig
  frontendSettings?: Record<string, any>
  generatedAt: string
}

// ============================================================================
// 请求/响应类型
// ============================================================================

export interface FRouterCreateRequest {
  name: string
  chainProxy?: ChainProxySettings
  tags?: string[]
}

export interface FRouterUpdateRequest {
  name: string
  chainProxy?: ChainProxySettings
  tags?: string[]
}

export interface NodeCreateRequest {
  name: string
  address: string
  port: number
  protocol: NodeProtocol
  tags?: string[]
  security?: NodeSecurity
  transport?: NodeTransport
  tls?: NodeTLS
}

export interface NodeUpdateRequest {
  name: string
  address: string
  port: number
  protocol: NodeProtocol
  tags?: string[]
  security?: NodeSecurity
  transport?: NodeTransport
  tls?: NodeTLS
}

export interface ConfigImportRequest {
  name: string
  format: ConfigFormat
  sourceUrl: string
  payload?: string
  autoUpdateIntervalMinutes?: number
  expireAt?: string | null
}

export interface ConfigUpdateRequest {
  name: string
  format: ConfigFormat
  sourceUrl: string
  payload?: string
  autoUpdateIntervalMinutes?: number
  expireAt?: string | null
}

export interface GeoResourceRequest {
  name: string
  type: GeoResourceType
  sourceUrl: string
  checksum?: string
  version?: string
}

export interface ComponentRequest {
  name?: string
  kind?: CoreComponentKind
  sourceUrl?: string
  archiveType?: string
}

export interface SystemProxyRequest {
  enabled: boolean
  ignoreHosts: string[]
}

export interface NodesListResponse {
  nodes: Node[]
}

export interface FRoutersListResponse {
  frouters: FRouter[]
}

export interface SystemProxyResponse {
  settings: SystemProxySettings
  message: string
}

export interface HealthResponse {
  status: string
  timestamp: string
}

// ============================================================================
// 错误类型
// ============================================================================

export class VeaError extends Error {
  statusCode: number
  response: any
  constructor(message: string, statusCode: number, response: any)
}

// ============================================================================
// API 接口
// ============================================================================

export interface FRoutersAPI {
  list(): Promise<FRoutersListResponse>
  create(data: FRouterCreateRequest): Promise<FRouter>
  update(id: string, data: FRouterUpdateRequest): Promise<FRouter>
  delete(id: string): Promise<null>
  ping(id: string): Promise<null>
  speedtest(id: string): Promise<null>
  measureLatency(id: string): Promise<null>
  measureSpeed(id: string): Promise<null>
  bulkPing(ids?: string[]): Promise<null>
  resetSpeed(ids?: string[]): Promise<null>
  getGraph(id: string): Promise<FRouterGraphResponse>
  saveGraph(id: string, data: FRouterGraphRequest): Promise<FRouter>
  validateGraph(id: string, data: FRouterGraphRequest): Promise<ValidateGraphResponse>
}

export interface ConfigsAPI {
  list(): Promise<Config[]>
  import(data: ConfigImportRequest): Promise<Config>
  update(id: string, data: ConfigUpdateRequest): Promise<Config>
  delete(id: string): Promise<null>
  refresh(id: string): Promise<Config>
  pullNodes(id: string): Promise<NodesListResponse>
}

export interface NodesAPI {
  list(): Promise<NodesListResponse>
  create(data: NodeCreateRequest): Promise<Node>
  update(id: string, data: NodeUpdateRequest): Promise<Node>
  ping(id: string): Promise<null>
  speedtest(id: string): Promise<null>
}

export interface GeoAPI {
  list(): Promise<GeoResource[]>
  create(data: GeoResourceRequest): Promise<GeoResource>
  update(id: string, data: GeoResourceRequest): Promise<GeoResource>
  delete(id: string): Promise<null>
  refresh(id: string): Promise<GeoResource>
}

export interface ComponentsAPI {
  list(): Promise<CoreComponent[]>
  create(data: ComponentRequest): Promise<CoreComponent>
  update(id: string, data: ComponentRequest): Promise<CoreComponent>
  delete(id: string): Promise<null>
  install(id: string): Promise<CoreComponent>
}

export interface ProxyAPI {
  getConfig(): Promise<any>
  updateConfig(data: any): Promise<any>
  status(): Promise<any>
  start(data?: any): Promise<any>
  stop(): Promise<any>
}

export interface SettingsAPI {
  getSystemProxy(): Promise<SystemProxyResponse>
  updateSystemProxy(data: SystemProxyRequest): Promise<SystemProxyResponse>
}

// ============================================================================
// 客户端配置
// ============================================================================

export interface VeaClientOptions {
  baseURL?: string
  timeout?: number
  headers?: Record<string, string>
}

export interface RequestOptions {
  method?: string
  path: string
  body?: any
  headers?: Record<string, string>
  timeout?: number
}

// ============================================================================
// 主客户端类
// ============================================================================

export class VeaClient {
  baseURL: string
  timeout: number
  headers: Record<string, string>
  isBrowser: boolean
  isNode: boolean

  nodes: NodesAPI
  frouters: FRoutersAPI
  configs: ConfigsAPI
  geo: GeoAPI
  components: ComponentsAPI
  settings: SettingsAPI
  proxy: ProxyAPI

  constructor(options?: VeaClientOptions)

  request(options: RequestOptions): Promise<any>
  get(path: string, options?: Partial<RequestOptions>): Promise<any>
  post(path: string, body?: any, options?: Partial<RequestOptions>): Promise<any>
  put(path: string, body?: any, options?: Partial<RequestOptions>): Promise<any>
  delete(path: string, options?: Partial<RequestOptions>): Promise<any>

  health(): Promise<HealthResponse>
  snapshot(): Promise<ServiceState>
}

// ============================================================================
// 工具函数类型
// ============================================================================

export function formatTime(value: string | Date): string
export function formatBytes(bytes: number): string
export function formatInterval(duration: number | string): string
export function formatLatency(ms: number): string
export function formatSpeed(mbps: number): string
export function escapeHtml(value: any): string
export function sleep(ms: number): Promise<void>
export function parseList(input: string): string[]
export function parseNumber(value: any): number
export function debounce<T extends (...args: any[]) => any>(fn: T, delay: number): T
export function throttle<T extends (...args: any[]) => any>(fn: T, delay: number): T

export interface Poller {
  start(): void
  stop(): void
  isRunning(): boolean
}

export function createPoller(fn: () => void | Promise<void>, interval: number): Poller

export interface RetryOptions {
  maxRetries?: number
  delay?: number
  shouldRetry?: (error: any) => boolean
}

export function retry<T>(fn: () => Promise<T>, options?: RetryOptions): Promise<T>

// ============================================================================
// 简化 API 对象（给前端用）
// ============================================================================

export interface SimpleAPI {
  request(path: string, options?: any): Promise<any>
  get(path: string, options?: any): Promise<any>
  post(path: string, body?: any, options?: any): Promise<any>
  put(path: string, body?: any, options?: any): Promise<any>
  delete(path: string, options?: any): Promise<any>

  nodes: NodesAPI
  frouters: FRoutersAPI
  configs: ConfigsAPI
  geo: GeoAPI
  components: ComponentsAPI
  settings: SettingsAPI
  proxy: ProxyAPI

  client: VeaClient
}

export function createAPI(baseURL?: string): SimpleAPI

// ============================================================================
// 默认导出
// ============================================================================

export { VeaClient as default }

// ============================================================================
// 工具函数默认导出
// ============================================================================

declare const utils: {
  formatTime: typeof formatTime
  formatBytes: typeof formatBytes
  formatInterval: typeof formatInterval
  formatLatency: typeof formatLatency
  formatSpeed: typeof formatSpeed
  escapeHtml: typeof escapeHtml
  sleep: typeof sleep
  parseList: typeof parseList
  parseNumber: typeof parseNumber
  debounce: typeof debounce
  throttle: typeof throttle
  createPoller: typeof createPoller
  retry: typeof retry
}

export { utils }
