// The single module that talks to the Go Desktop bindings (D-25). Components
// import `api` from here and never touch wailsjs or window.go, so tests mock
// this one module. Every function rejects with an ApiError.
import * as Desktop from '../../wailsjs/go/main/Desktop'
import { toApiError } from './errors'
import type {
  BackupOutcome,
  ClientConnections,
  Claim,
  ClaimInput,
  ClaimPatch,
  ConclusionInput,
  CreateInvestigationInput,
  DetailOptions,
  Evidence,
  EvidenceInput,
  EvidencePatch,
  FileResult,
  GetEvidenceOptions,
  Hypothesis,
  HypothesisInput,
  HypothesisPatch,
  ImportOutcome,
  Investigation,
  InvestigationDetail,
  InvestigationPatch,
  InvestigationSummary,
  ListOptions,
  Question,
  QuestionInput,
  Relation,
  ReportPreview,
  ShareOptions,
  SharePreview,
  ReviewFinding,
  ReviewStatementInput,
  RestorePick,
  RestoreResult,
  SearchHit,
  SearchOptions,
  StartupInfo,
  UpdateCheck,
  Verification,
  WindowTheme,
} from './types'

// The generated bindings type their arguments and results as the generated
// model classes; the JSON on the wire is exactly the plain shapes in
// types.ts, so the casts below only bridge the two type systems.
type Loose = never

async function call<T>(fn: () => Promise<unknown>): Promise<T> {
  try {
    return (await fn()) as T
  } catch (e) {
    throw toApiError(e)
  }
}

export const api = {
  // App and startup.
  startupState: () => call<StartupInfo>(() => Desktop.StartupState()),
  dataLocation: () => call<string>(() => Desktop.DataLocation()),
  appVersion: () => call<string>(() => Desktop.AppVersion()),
  quit: () => call<void>(() => Desktop.Quit()),
  changeToken: () => call<string>(() => Desktop.ChangeToken()),
  /** Asks GitHub for the latest release (ADR 0022); only when the person chooses to. */
  checkForUpdates: () => call<UpdateCheck>(() => Desktop.CheckForUpdates()),
  /** Opens the release page the last successful check found. */
  openReleasePage: () => call<void>(() => Desktop.OpenReleasePage()),

  // Startup recovery (AR-11): these work when the data failed to open.
  openDataFolder: () => call<void>(() => Desktop.OpenDataFolder()),
  copyDataLocation: () => call<void>(() => Desktop.CopyDataLocation()),
  pickBackupToRecover: () => call<RestorePick>(() => Desktop.PickBackupToRecover()),
  recoverFromBackup: (path: string) => call<RestoreResult>(() => Desktop.RecoverFromBackup(path)),

  // Investigations.
  listInvestigations: (o: ListOptions = {}) =>
    call<InvestigationSummary[]>(() => Desktop.ListInvestigations(o as Loose)),
  getInvestigationDetail: (invRef: string, o: DetailOptions = {}) =>
    call<InvestigationDetail>(() => Desktop.GetInvestigationDetail(invRef, o as Loose)),
  createInvestigation: (input: CreateInvestigationInput) =>
    call<Investigation>(() => Desktop.CreateInvestigation(input as Loose)),
  updateInvestigation: (invRef: string, patch: InvestigationPatch) =>
    call<Investigation>(() => Desktop.UpdateInvestigation(invRef, patch as Loose)),
  archiveInvestigation: (invRef: string) => call<Investigation>(() => Desktop.ArchiveInvestigation(invRef)),
  unarchiveInvestigation: (invRef: string) => call<Investigation>(() => Desktop.UnarchiveInvestigation(invRef)),
  updateConclusion: (invRef: string, input: ConclusionInput) =>
    call<Investigation>(() => Desktop.UpdateConclusion(invRef, input as Loose)),
  setActiveInvestigation: (invRef: string) => call<Investigation>(() => Desktop.SetActiveInvestigation(invRef)),
  /** Clears the active investigation only while it is still invRef; true when it cleared. */
  clearActiveInvestigation: (invRef: string) => call<boolean>(() => Desktop.ClearActiveInvestigation(invRef)),
  search: (query: string, o: SearchOptions = {}) => call<SearchHit[]>(() => Desktop.Search(query, o as Loose)),

  // Focused human review.
  getReviewFindings: (invRef: string) => call<ReviewFinding[]>(() => Desktop.GetReviewFindings(invRef)),
  reviewStatement: (invRef: string, input: ReviewStatementInput) =>
    call<ReviewFinding>(() => Desktop.ReviewStatement(invRef, input as Loose)),

  // Evidence.
  addEvidence: (invRef: string, input: EvidenceInput) =>
    call<Evidence>(() => Desktop.AddEvidence(invRef, input as Loose)),
  getEvidence: (invRef: string, evidenceRef: string, o: GetEvidenceOptions = {}) =>
    call<Evidence>(() => Desktop.GetEvidence(invRef, evidenceRef, o as Loose)),
  updateEvidence: (invRef: string, evidenceRef: string, patch: EvidencePatch) =>
    call<Evidence>(() => Desktop.UpdateEvidence(invRef, evidenceRef, patch as Loose)),
  setEvidenceVerification: (invRef: string, evidenceRef: string, v: Verification) =>
    call<Evidence>(() => Desktop.SetEvidenceVerification(invRef, evidenceRef, v)),

  // Claims.
  addClaim: (invRef: string, input: ClaimInput) => call<Claim>(() => Desktop.AddClaim(invRef, input as Loose)),
  getClaim: (invRef: string, claimRef: string) => call<Claim>(() => Desktop.GetClaim(invRef, claimRef)),
  updateClaim: (invRef: string, claimRef: string, patch: ClaimPatch) =>
    call<Claim>(() => Desktop.UpdateClaim(invRef, claimRef, patch as Loose)),
  setClaimVerification: (invRef: string, claimRef: string, v: Verification) =>
    call<Claim>(() => Desktop.SetClaimVerification(invRef, claimRef, v)),
  supersedeClaim: (invRef: string, claimRef: string, replacementRef: string) =>
    call<Claim>(() => Desktop.SupersedeClaim(invRef, claimRef, replacementRef)),
  linkEvidenceToClaim: (invRef: string, claimRef: string, evidenceRef: string, rel: Relation) =>
    call<Claim>(() => Desktop.LinkEvidenceToClaim(invRef, claimRef, evidenceRef, rel)),
  unlinkEvidenceFromClaim: (invRef: string, claimRef: string, evidenceRef: string) =>
    call<Claim>(() => Desktop.UnlinkEvidenceFromClaim(invRef, claimRef, evidenceRef)),

  // Hypotheses.
  addHypothesis: (invRef: string, input: HypothesisInput) =>
    call<Hypothesis>(() => Desktop.AddHypothesis(invRef, input as Loose)),
  getHypothesis: (invRef: string, hypRef: string) => call<Hypothesis>(() => Desktop.GetHypothesis(invRef, hypRef)),
  updateHypothesis: (invRef: string, hypRef: string, patch: HypothesisPatch) =>
    call<Hypothesis>(() => Desktop.UpdateHypothesis(invRef, hypRef, patch as Loose)),
  setHypothesisVerification: (invRef: string, hypRef: string, v: Verification) =>
    call<Hypothesis>(() => Desktop.SetHypothesisVerification(invRef, hypRef, v)),
  supersedeHypothesis: (invRef: string, hypRef: string, replacementRef: string) =>
    call<Hypothesis>(() => Desktop.SupersedeHypothesis(invRef, hypRef, replacementRef)),
  linkEvidenceToHypothesis: (invRef: string, hypRef: string, evidenceRef: string, rel: Relation) =>
    call<Hypothesis>(() => Desktop.LinkEvidenceToHypothesis(invRef, hypRef, evidenceRef, rel)),
  unlinkEvidenceFromHypothesis: (invRef: string, hypRef: string, evidenceRef: string) =>
    call<Hypothesis>(() => Desktop.UnlinkEvidenceFromHypothesis(invRef, hypRef, evidenceRef)),

  // Questions.
  addQuestion: (invRef: string, input: QuestionInput) =>
    call<Question>(() => Desktop.AddQuestion(invRef, input as Loose)),
  updateQuestion: (invRef: string, qRef: string, text: string) =>
    call<Question>(() => Desktop.UpdateQuestion(invRef, qRef, text)),
  resolveQuestion: (invRef: string, qRef: string, resolution: string) =>
    call<Question>(() => Desktop.ResolveQuestion(invRef, qRef, resolution)),
  deferQuestion: (invRef: string, qRef: string, note: string) =>
    call<Question>(() => Desktop.DeferQuestion(invRef, qRef, note)),
  reopenQuestion: (invRef: string, qRef: string) => call<Question>(() => Desktop.ReopenQuestion(invRef, qRef)),

  // Deleting (AR-10): permanent; the page confirms first. An investigation
  // must be archived; a record something still depends on is a conflict.
  deleteInvestigation: (invRef: string) => call<void>(() => Desktop.DeleteInvestigation(invRef)),
  deleteEvidence: (invRef: string, ref: string) => call<void>(() => Desktop.DeleteEvidence(invRef, ref)),
  deleteClaim: (invRef: string, ref: string) => call<void>(() => Desktop.DeleteClaim(invRef, ref)),
  deleteHypothesis: (invRef: string, ref: string) => call<void>(() => Desktop.DeleteHypothesis(invRef, ref)),
  deleteQuestion: (invRef: string, ref: string) => call<void>(() => Desktop.DeleteQuestion(invRef, ref)),

  // Clipboard and window.
  copyContext: (invRef: string) => call<void>(() => Desktop.CopyContext(invRef)),
  copyContextPlain: (invRef: string) => call<void>(() => Desktop.CopyContextPlain(invRef)),
  copyReport: (invRef: string) => call<void>(() => Desktop.CopyReport(invRef)),
  /** The report as Markdown and HTML for the Share summary dialog. */
  sharePreview: (invRef: string, options: ShareOptions) => call<SharePreview>(() => Desktop.SharePreview(invRef, options as Loose)),
  copySharePreview: (snapshotID: string) => call<void>(() => Desktop.CopySharePreview(snapshotID)),
  saveSharePreview: (snapshotID: string) => call<FileResult>(() => Desktop.SaveSharePreview(snapshotID)),
  reportPreview: (invRef: string) => call<ReportPreview>(() => Desktop.ReportPreview(invRef)),
  /** Saves the report as a Markdown file through the native save dialog. */
  saveReport: (invRef: string) => call<FileResult>(() => Desktop.SaveReport(invRef)),
  copyText: (text: string) => call<void>(() => Desktop.CopyText(text)),
  setWindowTheme: (theme: WindowTheme) => call<void>(() => Desktop.SetWindowTheme(theme)),

  // AI clients (read-only status and setup instructions).
  clientConnections: () => call<ClientConnections>(() => Desktop.ClientConnections()),

  // Files: native dialogs run in Go; cancelled dialogs resolve with
  // cancelled: true.
  exportInvestigation: (invRef: string) => call<FileResult>(() => Desktop.ExportInvestigation(invRef)),
  importInvestigation: () => call<ImportOutcome>(() => Desktop.ImportInvestigation()),
  backUp: () => call<BackupOutcome>(() => Desktop.BackUp()),
  pickBackupToRestore: () => call<RestorePick>(() => Desktop.PickBackupToRestore()),
  restoreBackup: (path: string) => call<RestoreResult>(() => Desktop.RestoreBackup(path)),
}

export type Api = typeof api
