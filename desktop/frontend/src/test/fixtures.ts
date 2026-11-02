// Shared test data mirroring the --seed-demo investigations (desktop/seed.go).
// Plans 02 to 04 add their own fixture files instead of editing this one.
import type {
  Activity,
  Actor,
  BackupInfo,
  Claim,
  Counts,
  Evidence,
  Hypothesis,
  Investigation,
  InvestigationDetail,
  InvestigationSummary,
  Question,
  RestoreResult,
  StartupInfo,
} from '../api/types'

export const T0 = '2026-09-28T13:00:00.000Z'
export const T_RECENT = '2026-10-02T09:00:00.000Z'

export const human: Actor = { kind: 'human', client: 'casebook-ui' }
export const claude: Actor = { kind: 'ai', client: 'claude-code' }
export const codex: Actor = { kind: 'ai', client: 'codex' }

export const DB_PATH = 'C:\\Users\\Ada\\AppData\\Local\\Casebook\\casebook.db'

export const startupOk: StartupInfo = { ok: true, kind: '', message: '', db_path: DB_PATH }

export const investigation: Investigation = {
  id: 1,
  ref: 'INV-0001',
  title: 'September cloud bill up 38%',
  problem_statement: 'Why did the cloud bill rise 38% in September when traffic was flat?',
  status: 'investigating',
  conclusion: 'Debug logging requested for payments only reached every service and drove most of the increase.',
  conclusion_confidence: 'high',
  conclusion_rationale:
    'The change, the pricing page and the billing export agree; the remaining 29% is still open.',
  conclusion_verification: 'ai_generated',
  conclusion_supporting_claims: ['C-001'],
  conclusion_contradicting_claims: [],
  conclusion_reviewed_at: '',
  conclusion_review_basis: '',
  conclusion_needs_review: true,
  conclusion_review_token: 'fixture-conclusion-review',
  created_at: T0,
  updated_at: T_RECENT,
  archived_at: '',
  archived: false,
  is_active: true,
  created_by: human,
}

export const summaries: InvestigationSummary[] = [
  {
    id: 1,
    ref: 'INV-0001',
    title: 'September cloud bill up 38%',
    status: 'investigating',
    conclusion_confidence: 'high',
    created_at: T0,
    updated_at: T_RECENT,
    archived_at: '',
    archived: false,
    is_active: true,
    counts: { evidence: 4, claims: 4, hypotheses: 3, questions: 3, open_questions: 1 },
  },
  {
    id: 2,
    ref: 'INV-0002',
    title: 'Checkout p99 doubled after the region move',
    status: 'blocked',
    conclusion_confidence: 'unknown',
    created_at: '2026-09-29T10:00:00.000Z',
    updated_at: '2026-10-01T16:00:00.000Z',
    archived_at: '',
    archived: false,
    is_active: false,
    counts: { evidence: 2, claims: 1, hypotheses: 0, questions: 1, open_questions: 1 },
  },
  {
    id: 3,
    ref: 'INV-0003',
    title: '31 former employees still have active SSO accounts',
    status: 'archived',
    conclusion_confidence: 'high',
    created_at: '2026-09-28T15:00:00.000Z',
    updated_at: '2026-09-30T11:00:00.000Z',
    archived_at: '2026-09-30T11:00:00.000Z',
    archived: true,
    is_active: false,
    counts: { evidence: 1, claims: 1, hypotheses: 0, questions: 0, open_questions: 0 },
  },
]

function ev(n: number, partial: Partial<Evidence>): Evidence {
  return {
    id: n,
    ref: `E-00${n}`,
    investigation_ref: 'INV-0001',
    title: '',
    summary: '',
    has_raw_detail: false,
    source_type: 'manual_observation',
    source_ref: '',
    source_label: '',
    captured_at: T0,
    created_at: T0,
    updated_at: T0,
    created_by: claude,
    verification: 'ai_generated',
    metadata: '',
    claim_links: [],
    hypothesis_links: [],
    ...partial,
  }
}

export const evidence: Evidence[] = [
  ev(1, {
    title: 'Service chart default log level set to debug',
    summary:
      "A platform change merged on August 28 changed the shared service chart's default log level from info to debug, so every service picks up debug logging at its next deploy.",
    source_type: 'code',
    source_ref: 'platform-charts/service/values.yaml',
    source_label: 'Platform Helm chart',
    has_raw_detail: true,
    claim_links: [{ ref: 'C-001', relation: 'supports' }],
  }),
  ev(2, {
    title: 'Logs are billed per GB ingested in the month',
    summary: "The provider's pricing page bills logs per GB ingested, with no volume discount below 500 TB a month, so a jump in log volume lands on the same month's bill in full.",
    source_type: 'documentation',
    source_ref: 'Provider pricing page, Logs and monitoring section',
    source_label: 'Provider pricing page',
    claim_links: [{ ref: 'C-002', relation: 'supports' }],
    hypothesis_links: [{ ref: 'H-003', relation: 'supports' }],
  }),
  ev(3, {
    title: 'Log ingestion is 71% of the September increase',
    summary:
      'In the billing export, log ingestion rose from $12,100 to $108,500, which is $96,400 of the $135,800 month-over-month increase, spread across 214 services with none above 4% of the total; compute and network moved less than 2%.',
    source_type: 'sql_query',
    source_label: 'Billing export query',
    has_raw_detail: true,
    metadata: '{"affected_records":214}',
    claim_links: [
      { ref: 'C-002', relation: 'supports' },
      { ref: 'C-003', relation: 'contradicts' },
      { ref: 'C-004', relation: 'contradicts' },
    ],
    hypothesis_links: [{ ref: 'H-002', relation: 'contradicts' }],
  }),
  ev(4, {
    title: 'Head of platform: debug was requested for payments only',
    summary: 'The head of platform engineering said the payments team asked for debug logs for the card network audit, and the change went in as the chart default instead of in the payments values file.',
    source_type: 'user_statement',
    source_label: 'Head of platform, in person',
    created_by: human,
    verification: 'human_verified',
    claim_links: [{ ref: 'C-001', relation: 'supports' }],
  }),
]

function link(ref: string, relation: 'supports' | 'contradicts') {
  const e = evidence.find((x) => x.ref === ref)!
  return { evidence_ref: ref, relation, evidence_title: e.title, created_at: T0, created_by: e.created_by }
}

export const claims: Claim[] = [
  {
    id: 1,
    ref: 'C-001',
    investigation_ref: 'INV-0001',
    statement:
      'A shared platform default moved every service to debug logging on August 28; only payments asked for it.',
    confidence: 'high',
    confidence_rationale: 'The chart diff and the head of platform agree.',
    status: 'active',
    verification: 'human_verified',
    superseded_by: '',
    created_at: T0,
    updated_at: T0,
    created_by: human,
    links: [link('E-001', 'supports'), link('E-004', 'supports')],
  },
  {
    id: 2,
    ref: 'C-002',
    investigation_ref: 'INV-0001',
    statement: 'Log ingestion, billed per GB, is $96,400 of the $135,800 increase; compute and network are flat.',
    confidence: 'high',
    confidence_rationale: 'The pricing model and the billing export agree.',
    status: 'active',
    verification: 'ai_generated',
    superseded_by: '',
    created_at: T0,
    updated_at: T0,
    created_by: claude,
    links: [link('E-002', 'supports'), link('E-003', 'supports')],
  },
  {
    id: 3,
    ref: 'C-003',
    investigation_ref: 'INV-0001',
    statement: 'The increase comes from the August autoscaling change adding compute nodes.',
    confidence: 'low',
    confidence_rationale: '',
    status: 'superseded',
    verification: 'superseded',
    superseded_by: 'C-002',
    created_at: T0,
    updated_at: T0,
    created_by: claude,
    links: [link('E-003', 'contradicts')],
  },
  {
    id: 4,
    ref: 'C-004',
    investigation_ref: 'INV-0001',
    statement: "The payments team's own debug logs are most of the $96,400.",
    confidence: 'low',
    confidence_rationale: '',
    status: 'disputed',
    verification: 'disputed',
    superseded_by: '',
    created_at: T0,
    updated_at: T0,
    created_by: claude,
    links: [link('E-003', 'contradicts')],
  },
]

export const hypotheses: Hypothesis[] = [
  {
    id: 1,
    ref: 'H-001',
    investigation_ref: 'INV-0001',
    statement: "The new tracing agent may also be shipping some services' logs twice.",
    confidence: 'medium',
    rationale: '',
    status: 'open',
    verification: 'ai_generated',
    superseded_by: '',
    created_at: T0,
    updated_at: T0,
    created_by: claude,
    links: [],
  },
  {
    id: 2,
    ref: 'H-002',
    investigation_ref: 'INV-0001',
    statement: 'Data transfer from the region move doubled the network line.',
    confidence: 'low',
    rationale: 'Network spend fell $600 month over month.',
    status: 'disproven',
    verification: 'ai_generated',
    superseded_by: '',
    created_at: T0,
    updated_at: T0,
    created_by: claude,
    links: [link('E-003', 'contradicts')],
  },
  {
    id: 3,
    ref: 'H-003',
    investigation_ref: 'INV-0001',
    statement: 'Services that pin their own log level were not affected, which is why 214 of 260 services show the jump.',
    confidence: 'medium',
    rationale: '',
    status: 'supported',
    verification: 'ai_generated',
    superseded_by: '',
    created_at: T0,
    updated_at: T0,
    created_by: claude,
    links: [link('E-002', 'supports')],
  },
]

export const questions: Question[] = [
  {
    id: 1,
    ref: 'Q-001',
    investigation_ref: 'INV-0001',
    question: 'Can every service except payments go back to info level today?',
    status: 'open',
    resolution: '',
    defer_note: '',
    resolved_at: '',
    created_at: T0,
    updated_at: T0,
    created_by: human,
  },
  {
    id: 2,
    ref: 'Q-002',
    investigation_ref: 'INV-0001',
    question: 'Which cost center does log ingestion bill to in the finance export?',
    status: 'resolved',
    resolution: 'Platform shared services (CC-4100), so no product team saw the jump in its own showback (see E-003).',
    defer_note: '',
    resolved_at: T0,
    created_at: T0,
    updated_at: T0,
    created_by: claude,
  },
  {
    id: 3,
    ref: 'Q-003',
    investigation_ref: 'INV-0001',
    question: 'Should finance restate September showback so the debug logs are charged to the teams that produced them?',
    status: 'deferred',
    resolution: '',
    defer_note: 'Waiting for the quarter close on October 10.',
    resolved_at: '',
    created_at: T0,
    updated_at: T0,
    created_by: human,
  },
]

export const activity: Activity[] = [
  {
    id: 30,
    investigation_ref: 'INV-0001',
    actor: human,
    action: 'active.set',
    target_ref: 'INV-0001',
    summary: '',
    note: '',
    created_at: T_RECENT,
  },
  {
    id: 29,
    investigation_ref: 'INV-0001',
    actor: human,
    action: 'conclusion.update',
    target_ref: 'INV-0001',
    summary: 'conclusion, confidence, rationale',
    note: '',
    created_at: '2026-10-02T08:30:00.000Z',
  },
  {
    id: 20,
    investigation_ref: 'INV-0001',
    actor: claude,
    action: 'evidence.add',
    target_ref: 'E-003',
    summary: 'Log ingestion is 71% of the September increase',
    note: '',
    created_at: '2026-09-30T12:00:00.000Z',
  },
  {
    id: 1,
    investigation_ref: 'INV-0001',
    actor: human,
    action: 'investigation.create',
    target_ref: 'INV-0001',
    summary: 'September cloud bill up 38%',
    note: '',
    created_at: T0,
  },
]

export const detail: InvestigationDetail = {
  investigation,
  evidence,
  claims,
  hypotheses,
  questions,
  recent_activity: activity,
  review_bases: [...claims, ...hypotheses].map((record) => ({ ref: record.ref, basis: `fixture-${record.ref}` })),
}

/** A brand-new investigation with nothing in it yet. */
export function emptyDetail(inv: Partial<Investigation> = {}): InvestigationDetail {
  return {
    investigation: {
      ...investigation,
      id: 9,
      ref: 'INV-0009',
      title: 'New investigation',
      problem_statement: '',
      status: 'new',
      conclusion: '',
      conclusion_confidence: 'unknown',
      conclusion_rationale: '',
      conclusion_verification: 'ai_generated',
      conclusion_supporting_claims: [],
      conclusion_contradicting_claims: [],
      conclusion_reviewed_at: '',
      conclusion_review_basis: '',
      conclusion_needs_review: false,
      conclusion_review_token: '',
      is_active: false,
      ...inv,
    },
    evidence: [],
    claims: [],
    hypotheses: [],
    questions: [],
    recent_activity: [],
    review_bases: [],
  }
}

/** Builds the summary the list would return for an investigation. */
export function summaryOf(inv: Investigation): InvestigationSummary {
  return {
    id: inv.id,
    ref: inv.ref,
    title: inv.title,
    status: inv.status,
    conclusion_confidence: inv.conclusion_confidence,
    created_at: inv.created_at,
    updated_at: inv.updated_at,
    archived_at: inv.archived_at,
    archived: inv.archived,
    is_active: inv.is_active,
    counts: { evidence: 0, claims: 0, hypotheses: 0, questions: 0, open_questions: 0 },
  }
}

export const counts: Counts = {
  investigations: 3,
  evidence: 7,
  claims: 6,
  hypotheses: 3,
  questions: 5,
  claim_links: 8,
  hypothesis_links: 2,
  activity: 48,
}

export const backupInfo: BackupInfo = {
  path: 'C:\\Users\\Ada\\Documents\\casebook-backup-2026-10-02.db',
  size_bytes: 245760,
  schema_version: 1,
  created_at: T_RECENT,
  counts,
}

export const restoreResult: RestoreResult = {
  source_path: backupInfo.path,
  safety_copy_path: 'C:\\Users\\Ada\\AppData\\Local\\Casebook\\backups\\casebook-pre-restore-20261002T090000Z.db',
  source_schema_version: 1,
  counts: { ...counts, activity: 49 },
}
