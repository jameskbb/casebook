// mockApi(overrides) returns an Api whose every function is a vi.fn stub
// resolving to fixtures.ts data. Tests mock the single api module with it:
//
//   const mocks = vi.hoisted(() => ({ api: {} as Api }))
//   vi.mock('../api/api', () => ({ api: mocks.api }))
//   beforeEach(() => Object.assign(mocks.api, mockApi({ ... })))
import { vi, type Mock } from 'vitest'
import type { Api } from '../api/api'
import type { Hypothesis } from '../api/types'
import {
  backupInfo,
  claims,
  DB_PATH,
  detail,
  evidence,
  hypotheses,
  investigation,
  questions,
  restoreResult,
  startupOk,
  summaries,
} from './fixtures'
import { clientConnections } from './fixtures.clients'
import { upToDate } from './fixtures.update'

export type MockedApi = { [K in keyof Api]: Api[K] & Mock<Api[K]> }

const hyp = (ref: string): Hypothesis => hypotheses.find((h) => h.ref === ref) ?? hypotheses[0]

export function mockApi(overrides: Partial<Api> = {}): MockedApi {
  const defaults: Api = {
    startupState: async () => startupOk,
    dataLocation: async () => DB_PATH,
    appVersion: async () => 'v0.1.0',
    quit: async () => undefined,
    changeToken: async () => '1.1',
    checkForUpdates: async () => upToDate,
    openReleasePage: async () => undefined,
    openDataFolder: async () => undefined,
    copyDataLocation: async () => undefined,
    pickBackupToRecover: async () => ({
      cancelled: false,
      path: backupInfo.path,
      file_name: 'casebook-backup-2026-11-06.db',
      info: backupInfo,
    }),
    recoverFromBackup: async () => ({
      ...restoreResult,
      safety_copy_path: 'C:\\Users\\Ada\\AppData\\Local\\Casebook\\backups\\casebook-unopened-20261109T090000Z.db',
    }),
    listInvestigations: async () => summaries,
    getInvestigationDetail: async () => detail,
    createInvestigation: async (input) => ({ ...investigation, title: input.title }),
    updateInvestigation: async (_ref, patch) => ({ ...investigation, ...patch }),
    archiveInvestigation: async () => ({ ...investigation, status: 'archived', archived: true }),
    unarchiveInvestigation: async () => investigation,
    updateConclusion: async (_ref, input) => ({
      ...investigation,
      conclusion: input.conclusion,
      conclusion_confidence: input.confidence || 'unknown',
      conclusion_rationale: input.rationale ?? '',
      conclusion_verification: input.verification ?? investigation.conclusion_verification,
      conclusion_supporting_claims: input.supporting_claims ?? investigation.conclusion_supporting_claims,
      conclusion_contradicting_claims: input.contradicting_claims ?? investigation.conclusion_contradicting_claims,
      conclusion_reviewed_at: input.review ? '2026-11-08T12:00:00.000Z' : '',
      conclusion_review_basis: input.review_basis ?? '',
      conclusion_needs_review: !input.review,
    }),
    setActiveInvestigation: async () => ({ ...investigation, is_active: true }),
    clearActiveInvestigation: async () => true,
    search: async () => [],
    reviewStatement: async () => undefined,
    addEvidence: async () => evidence[0],
    getEvidence: async (_inv, ref) => evidence.find((e) => e.ref === ref) ?? evidence[0],
    updateEvidence: async (_inv, ref) => evidence.find((e) => e.ref === ref) ?? evidence[0],
    setEvidenceVerification: async (_inv, ref, v) => ({
      ...(evidence.find((e) => e.ref === ref) ?? evidence[0]),
      verification: v,
    }),
    addClaim: async () => claims[0],
    getClaim: async (_inv, ref) => claims.find((c) => c.ref === ref) ?? claims[0],
    updateClaim: async (_inv, ref) => claims.find((c) => c.ref === ref) ?? claims[0],
    setClaimVerification: async (_inv, ref, v) => ({ ...(claims.find((c) => c.ref === ref) ?? claims[0]), verification: v }),
    supersedeClaim: async (_inv, ref) => claims.find((c) => c.ref === ref) ?? claims[0],
    linkEvidenceToClaim: async (_inv, ref) => claims.find((c) => c.ref === ref) ?? claims[0],
    unlinkEvidenceFromClaim: async (_inv, ref) => claims.find((c) => c.ref === ref) ?? claims[0],
    addHypothesis: async () => hypotheses[0],
    getHypothesis: async (_inv, ref) => hyp(ref),
    updateHypothesis: async (_inv, ref) => hyp(ref),
    setHypothesisVerification: async (_inv, ref, v) => ({ ...hyp(ref), verification: v }),
    supersedeHypothesis: async (_inv, ref) => hyp(ref),
    linkEvidenceToHypothesis: async (_inv, ref) => hyp(ref),
    unlinkEvidenceFromHypothesis: async (_inv, ref) => hyp(ref),
    addQuestion: async () => questions[0],
    updateQuestion: async (_inv, ref, text) => ({ ...(questions.find((q) => q.ref === ref) ?? questions[0]), question: text }),
    resolveQuestion: async (_inv, ref, resolution) => ({
      ...(questions.find((q) => q.ref === ref) ?? questions[0]),
      status: 'resolved',
      resolution,
    }),
    deferQuestion: async (_inv, ref, note) => ({
      ...(questions.find((q) => q.ref === ref) ?? questions[0]),
      status: 'deferred',
      defer_note: note,
    }),
    reopenQuestion: async (_inv, ref) => ({ ...(questions.find((q) => q.ref === ref) ?? questions[0]), status: 'open' }),
    deleteInvestigation: async () => undefined,
    deleteEvidence: async () => undefined,
    deleteClaim: async () => undefined,
    deleteHypothesis: async () => undefined,
    deleteQuestion: async () => undefined,
    copyContext: async () => undefined,
    copyContextPlain: async () => undefined,
    copyReport: async () => undefined,
    sharePreview: async (ref, options) => ({ markdown: `# ${ref}: Why did calls go up in October?\n`, text: `${ref}: Why did calls go up in October?\n`, html: `<h1>${ref}: Why did calls go up in October?</h1>\n`, snapshot_id: 'reviewed-1', investigation_ref: ref, mode: options.mode, candidates: [], omitted: { claims: 0, hypotheses: 0, questions: 0, evidence: 0, claim_refs: [], hypothesis_refs: [], question_refs: [], evidence_refs: [] } }),
    copySharePreview: async () => undefined,
    saveSharePreview: async () => ({ cancelled: false, path: 'Why did calls go up.html', file_name: 'Why did calls go up.html' }),
    reportPreview: async () => ({ markdown: '# INV-0001: Why did calls go up in October?\n', html: '<h1>INV-0001: Why did calls go up in October?</h1>\n' }),
    saveReport: async () => ({
      cancelled: false,
      path: 'C:\\Users\\Ada\\Documents\\Why did calls go up.html',
      file_name: 'Why did calls go up.html',
    }),
    copyText: async () => undefined,
    setWindowTheme: async () => undefined,
    clientConnections: async () => clientConnections,
    connectClient: async (tool) => ({ tool, connected: true, changed: true, message: "Casebook added itself to Claude Code's settings and kept a backup of the file. Close Claude Code and open it again to start.", files: [] }),
    disconnectClient: async (tool) => ({ tool, connected: false, changed: true, message: "Casebook took itself out of Claude Code's settings and kept a backup of the file.", files: [] }),
    exportInvestigation: async () => ({
      cancelled: false,
      path: 'C:\\Users\\Ada\\Documents\\INV-0001-why-did-calls-go-up.json',
      file_name: 'INV-0001-why-did-calls-go-up.json',
    }),
    importInvestigation: async () => ({
      cancelled: false,
      file_name: 'INV-0001-why-did-calls-go-up.json',
      title: 'Why did calls go up 30% in October when sales stayed the same?',
      result: { investigation_ref: 'INV-0004', title: 'Why did calls go up 30% in October when sales stayed the same?', original_ref: 'INV-0001', ref_changed: true, counts: backupInfo.counts },
    }),
    backUp: async () => ({ cancelled: false, info: backupInfo }),
    pickBackupToRestore: async () => ({
      cancelled: false,
      path: backupInfo.path,
      file_name: 'casebook-backup-2026-11-06.db',
      info: backupInfo,
    }),
    restoreBackup: async () => restoreResult,
  }
  const out = {} as Record<string, unknown>
  for (const [name, impl] of Object.entries({ ...defaults, ...overrides })) {
    out[name] = vi.fn(impl as (...args: unknown[]) => unknown)
  }
  return out as MockedApi
}
