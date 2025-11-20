/**
 * Vea SDK TypeScript 类型定义
 * 基于 OpenAPI 规范生成
 */

// ============================================================================
// 数据模型
// ============================================================================

export type NodeProtocol = 'vless' | 'trojan' | 'shadowsocks' | 'vmess'

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
  uploadBytes: number
  downloadBytes: number
  lastLatencyMs: number
  lastLatencyAt: string
  lastSpeedMbps: number
  lastSpeedAt: string
  lastSpeedError?: string
  lastSelectedAt: string
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
  uploadBytes: number
  downloadBytes: number
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

export type CoreComponentKind = 'xray' | 'geo' | 'generic'

export interface CoreComponent {
  id: string
  name: string
  kind: CoreComponentKind
  sourceUrl: string
  archiveType: string
  autoUpdateInterval: number
  lastInstalledAt: string
  installDir: string
  lastVersion: string
  checksum: string
  lastSyncError: string
  meta?: Record<string, string>
  createdAt: string
  updatedAt: string
}

export interface DNSSetting {
  strategy: string
  servers: string[]
}

export interface TrafficRule {
  id: string
  name: string
  targets: string[]
  nodeId: string
  priority: number
  createdAt: string
  updatedAt: string
}

export interface TrafficProfile {
  defaultNodeId: string
  dns: DNSSetting
  rules: TrafficRule[]
  updatedAt: string
}

export interface SystemProxySettings {
  enabled: boolean
  ignoreHosts: string[]
  updatedAt: string
}

export interface XrayStatus {
  enabled: boolean
  running: boolean
  activeNodeId: string
  binary: string
  config: string
}

export interface ServiceState {
  nodes: Node[]
  configs: Config[]
  geoResources: GeoResource[]
  components: CoreComponent[]
  trafficProfile: TrafficProfile
  systemProxy: SystemProxySettings
  generatedAt: string
}

// ============================================================================
// 请求/响应类型
// ============================================================================

export interface NodeShareLinkRequest {
  shareLink: string
}

export interface NodeManualRequest {
  name: string
  address: string
  port: number
  protocol: NodeProtocol
  tags?: string[]
}

export interface NodeUpdateRequest {
  name: string
  address: string
  port: number
  protocol: NodeProtocol
  tags?: string[]
}

export interface TrafficRequest {
  uploadBytes: number
  downloadBytes: number
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
  autoUpdateIntervalMinutes?: number
}

export interface XrayStartOptions {
  activeNodeId?: string
}

export interface TrafficProfileRequest {
  defaultNodeId?: string
  dns?: {
    strategy?: string
    servers?: string[]
  }
}

export interface TrafficRuleRequest {
  name: string
  targets: string[]
  nodeId: string
  priority?: number
}

export interface SystemProxyRequest {
  enabled: boolean
  ignoreHosts: string[]
}

export interface NodesListResponse {
  nodes: Node[]
  activeNodeId: string
  lastSelectedNodeId: string
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

export interface NodesAPI {
  list(): Promise<NodesListResponse>
  create(data: NodeShareLinkRequest | NodeManualRequest): Promise<Node>
  update(id: string, data: NodeUpdateRequest): Promise<Node>
  delete(id: string): Promise<null>
  resetTraffic(id: string): Promise<Node>
  incrementTraffic(id: string, traffic: TrafficRequest): Promise<Node>
  ping(id: string): Promise<null>
  speedtest(id: string): Promise<null>
  select(id: string): Promise<null>
  bulkPing(ids?: string[]): Promise<null>
  resetSpeed(ids?: string[]): Promise<null>
}

export interface ConfigsAPI {
  list(): Promise<Config[]>
  import(data: ConfigImportRequest): Promise<Config>
  update(id: string, data: ConfigUpdateRequest): Promise<Config>
  delete(id: string): Promise<null>
  refresh(id: string): Promise<Config>
  pullNodes(id: string): Promise<Node[]>
  incrementTraffic(id: string, traffic: TrafficRequest): Promise<Config>
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

export interface XrayAPI {
  status(): Promise<XrayStatus>
  start(options?: XrayStartOptions): Promise<null>
  stop(): Promise<null>
}

export interface TrafficRulesAPI {
  list(): Promise<TrafficRule[]>
  create(data: TrafficRuleRequest): Promise<TrafficRule>
  update(id: string, data: TrafficRuleRequest): Promise<TrafficRule>
  delete(id: string): Promise<null>
}

export interface TrafficAPI {
  rules: TrafficRulesAPI
  getProfile(): Promise<TrafficProfile>
  updateProfile(data: TrafficProfileRequest): Promise<TrafficProfile>
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
  configs: ConfigsAPI
  geo: GeoAPI
  components: ComponentsAPI
  xray: XrayAPI
  traffic: TrafficAPI
  settings: SettingsAPI

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
// 简化 API 对象（兼容现有前端）
// ============================================================================

export interface SimpleAPI {
  request(path: string, options?: any): Promise<any>
  get(path: string, options?: any): Promise<any>
  post(path: string, body?: any, options?: any): Promise<any>
  put(path: string, body?: any, options?: any): Promise<any>
  delete(path: string, options?: any): Promise<any>
  client: VeaClient
}

export function createAPI(baseURL?: string): SimpleAPI

// ============================================================================
// 节点管理器
// ============================================================================

export interface NodeManagerUpdate {
  nodes: Node[]
  activeNodeId: string
  lastSelectedNodeId: string
}

export interface NodeManager {
  getNodes(): Node[]
  getActiveNodeId(): string
  getLastSelectedNodeId(): string
  refresh(): Promise<NodesListResponse>
  startPolling(): void
  stopPolling(): void
  onUpdate(callback: (update: NodeManagerUpdate) => void): () => void
}

export function createNodeManager(client: VeaClient, interval?: number): NodeManager

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
