// Shared test data: a call center question with findings and sources in
// every state, a help desk question with no answer yet, and an archived
// payroll question. Screenshots are taken over this, so it stays plain.
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

export const T0 = '2026-11-02T13:00:00.000Z'
export const T_RECENT = '2026-11-06T09:00:00.000Z'

export const human: Actor = { kind: 'human', client: 'casebook-ui' }
export const claude: Actor = { kind: 'ai', client: 'claude-code' }
export const codex: Actor = { kind: 'ai', client: 'codex' }

export const DB_PATH = 'C:\\Users\\Ada\\AppData\\Local\\Casebook\\casebook.db'

export const startupOk: StartupInfo = { ok: true, kind: '', message: '', db_path: DB_PATH }

export const investigation: Investigation = {
  id: 1,
  ref: 'INV-0001',
  title: 'Why did calls go up 30% in October when sales stayed the same?',
  problem_statement: 'Calls to the call center went up 30% in October, from 2,400 to 3,120. Sales stayed the same. Why?',
  status: 'investigating',
  conclusion:
    'The extra calls were customers asking about a bill. A billing mistake on October 3 charged some customers twice. About the same number of people called to buy, so sales stayed the same.',
  conclusion_confidence: 'high',
  conclusion_rationale:
    "The billing team's notice and the call log agree. Whether long waits cost any sales is still open.",
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
    title: 'Why did calls go up 30% in October when sales stayed the same?',
    status: 'investigating',
    conclusion_confidence: 'high',
    created_at: T0,
    updated_at: T_RECENT,
    archived_at: '',
    archived: false,
    is_active: true,
    counts: { evidence: 4, claims: 4, hypotheses: 3, questions: 3, open_questions: 1 },
    conclusion:
      'The extra calls were customers asking about a bill. A billing mistake on October 3 charged some customers twice. About the same number of people called to buy, so sales stayed the same.',
    sources_total: 4,
    sources_checked_by_you: 1,
    findings_disagree: 1,
    answer_checked_by_you: false,
  },
  {
    id: 2,
    ref: 'INV-0002',
    title: 'Why are help desk tickets taking longer to close this month?',
    status: 'blocked',
    conclusion_confidence: 'unknown',
    created_at: '2026-11-03T10:00:00.000Z',
    updated_at: '2026-11-05T16:00:00.000Z',
    archived_at: '',
    archived: false,
    is_active: false,
    counts: { evidence: 2, claims: 1, hypotheses: 0, questions: 1, open_questions: 1 },
    conclusion: '',
    sources_total: 2,
    sources_checked_by_you: 0,
    findings_disagree: 0,
    answer_checked_by_you: false,
  },
  {
    id: 3,
    ref: 'INV-0003',
    title: 'Why was March payroll $18,000 over budget?',
    status: 'archived',
    conclusion_confidence: 'high',
    created_at: '2026-11-02T15:00:00.000Z',
    updated_at: '2026-11-04T11:00:00.000Z',
    archived_at: '2026-11-04T11:00:00.000Z',
    archived: true,
    is_active: false,
    counts: { evidence: 1, claims: 1, hypotheses: 0, questions: 0, open_questions: 0 },
    conclusion: 'Two new hires started a month earlier than the budget assumed.',
    sources_total: 1,
    sources_checked_by_you: 1,
    findings_disagree: 0,
    answer_checked_by_you: true,
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
    title: 'Notice dated October 4: 1,900 customers were charged twice',
    summary:
      'The billing team wrote to all managers on October 4. A mistake on October 3 charged 1,900 customers twice. Refunds were promised within two weeks.',
    source_type: 'file',
    source_ref: 'Shared drive, Billing, notice-2026-10-04.pdf',
    source_label: 'Email from the billing team',
    has_raw_detail: true,
    claim_links: [{ ref: 'C-001', relation: 'supports' }],
  }),
  ev(2, {
    title: 'Sales were $510,000 in September and $512,000 in October',
    summary: 'The October sales report shows $510,000 of sales in September and $512,000 in October. The number of orders was 4,080 and 4,100.',
    source_type: 'documentation',
    source_ref: 'October sales report, page 2, Sales by month',
    source_label: 'October sales report',
    claim_links: [{ ref: 'C-002', relation: 'supports' }],
    hypothesis_links: [{ ref: 'H-003', relation: 'supports' }],
  }),
  ev(3, {
    title: 'Calls about a bill: 400 in September, 1,070 in October',
    summary:
      'In the call log, calls about a bill went from 400 in September to 1,070 in October. Calls to buy went from 2,000 to 2,050. The average wait was 40 seconds in both months.',
    source_type: 'sql_query',
    source_label: 'Call log',
    has_raw_detail: true,
    metadata: '{"calls_counted":5520}',
    claim_links: [
      { ref: 'C-002', relation: 'supports' },
      { ref: 'C-003', relation: 'contradicts' },
      { ref: 'C-004', relation: 'contradicts' },
    ],
    hypothesis_links: [{ ref: 'H-002', relation: 'contradicts' }],
  }),
  ev(4, {
    title: 'Head of billing: the mistake was on October 3 only',
    summary: 'The head of billing said the bills sent on October 3 went out twice because a file was loaded two times. Bills sent on other days were not affected.',
    source_type: 'user_statement',
    source_label: 'Head of billing, in person',
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
    statement: 'A billing mistake on October 3 charged some customers twice.',
    confidence: 'high',
    confidence_rationale: 'The notice and the head of billing agree.',
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
    statement: 'The extra calls were about a bill. About the same number of people called to buy.',
    confidence: 'high',
    confidence_rationale: 'The call log and the sales report agree.',
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
    statement: 'The extra calls were people calling to buy.',
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
    statement: 'Callers waited too long and hung up before buying.',
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
    statement: 'Customers who were charged twice are less likely to buy again.',
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
    statement: 'A new ad brought in more callers.',
    confidence: 'low',
    rationale: 'Calls to buy barely moved: 2,000 in September, 2,050 in October.',
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
    statement: 'People who called to buy bought about as often as in September.',
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
    question: 'Have the customers who were charged twice been paid back?',
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
    question: 'Do calls about a bill go to the same phone line as calls to buy?',
    status: 'resolved',
    resolution: 'Yes. Both go to the sales line, which is why the total went up (see E-003).',
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
    question: 'Should we call the customers who were charged twice to say sorry?',
    status: 'deferred',
    resolution: '',
    defer_note: 'Waiting for the refund list on November 10.',
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
    summary: 'Updated conclusion of INV-0001 (confidence high)',
    note: '',
    created_at: '2026-11-06T08:30:00.000Z',
  },
  {
    id: 20,
    investigation_ref: 'INV-0001',
    actor: claude,
    action: 'evidence.add',
    target_ref: 'E-003',
    summary: 'Calls about a bill: 400 in September, 1,070 in October',
    note: '',
    created_at: '2026-11-04T12:00:00.000Z',
  },
  {
    id: 1,
    investigation_ref: 'INV-0001',
    actor: human,
    action: 'investigation.create',
    target_ref: 'INV-0001',
    summary: 'Why did calls go up 30% in October when sales stayed the same?',
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

/** A brand-new question with nothing in it yet. */
export function emptyDetail(inv: Partial<Investigation> = {}): InvestigationDetail {
  return {
    investigation: {
      ...investigation,
      id: 9,
      ref: 'INV-0009',
      title: 'Why did returns double in May?',
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
    conclusion: inv.conclusion,
    sources_total: 0,
    sources_checked_by_you: 0,
    findings_disagree: 0,
    answer_checked_by_you: false,
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
  path: 'C:\\Users\\Ada\\Documents\\casebook-backup-2026-11-06.db',
  size_bytes: 245760,
  schema_version: 1,
  created_at: T_RECENT,
  counts,
}

export const restoreResult: RestoreResult = {
  source_path: backupInfo.path,
  safety_copy_path: 'C:\\Users\\Ada\\AppData\\Local\\Casebook\\backups\\casebook-pre-restore-20261106T090000Z.db',
  source_schema_version: 1,
  counts: { ...counts, activity: 49 },
}
