// 前后端 API 适配层：Wails 内使用生成的绑定，浏览器（开发自测）使用 mock。
import { Clipboard, System } from '@wailsio/runtime'
import * as AppService from '../../bindings/github.com/ruipenglu/Hermit/internal/app/service.js'
import type { PublicMapping, PublicState, TestRecord } from '../../bindings/github.com/ruipenglu/Hermit/internal/core/models'
import type { MappingDraft, NodeInfo, SaveResult, SaveTestResult } from '../../bindings/github.com/ruipenglu/Hermit/internal/app/models'

export type { PublicMapping, PublicState, TestRecord, MappingDraft, NodeInfo, SaveResult, SaveTestResult }

export interface Api {
  version(): Promise<string>
  status(): Promise<PublicState>
  nodes(): Promise<NodeInfo[]>
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  doctor(): Promise<Record<string, any> | null>
  saveMapping(draft: MappingDraft): Promise<SaveResult>
  saveAndTest(draft: MappingDraft): Promise<SaveTestResult>
  deleteMapping(id: string): Promise<PublicState>
  setEnabled(enabled: boolean): Promise<PublicState>
  testMapping(id: string): Promise<TestRecord>
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  recover(): Promise<Record<string, any> | null>
  copyText(text: string): Promise<void>
}

// 打包窗口使用 wails:；系统标记可能晚于模块初始化，不能仅依赖 IsMac。
export const inWails = typeof window !== 'undefined' && (window.location.protocol === 'wails:' || System.IsMac())

const realApi: Api = {
  version: () => AppService.Version(),
  status: () => AppService.Status(),
  nodes: () => AppService.Nodes().then((nodes) => nodes ?? []),
  doctor: () => AppService.Doctor(),
  saveMapping: (draft) => AppService.SaveMapping(draft),
  saveAndTest: (draft) => AppService.SaveAndTest(draft),
  deleteMapping: (id) => AppService.DeleteMapping(id),
  setEnabled: (enabled) => AppService.SetEnabled(enabled),
  testMapping: (id) => AppService.TestMapping(id),
  recover: () => AppService.Recover(),
  copyText: (text) => Clipboard.SetText(text),
}

export const api: Api = import.meta.env.DEV && !inWails ? (await import('./mock')).mockApi : realApi
