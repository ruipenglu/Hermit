// 前后端 API 适配层：Wails 内使用生成的绑定，浏览器（开发自测）使用 mock。
import { Clipboard, System } from '@wailsio/runtime'
import * as AppService from '../../bindings/github.com/lurp/hermit/internal/app/service.js'
import type { PublicMapping, PublicState, TestRecord } from '../../bindings/github.com/lurp/hermit/internal/core/models'
import type { MappingDraft, NodeInfo, SaveResult, SaveTestResult } from '../../bindings/github.com/lurp/hermit/internal/app/models'
import { mockApi } from './mock'

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

// runtime 在浏览器中也会创建 _wails，使用原生注入的系统标记区分预览。
export const inWails = typeof window !== 'undefined' && System.IsMac()

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

export const api: Api = inWails ? realApi : mockApi
