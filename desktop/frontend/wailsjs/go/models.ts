export namespace app {

	export class ReviewEvidence {
	    relation: string;
	    evidence: domain.Evidence;

	    static createFrom(source: any = {}) {
	        return new ReviewEvidence(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.relation = source["relation"];
	        this.evidence = this.convertValues(source["evidence"], domain.Evidence);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ReviewFinding {
	    kind: string;
	    ref: string;
	    statement: string;
	    confidence: string;
	    rationale: string;
	    status: string;
	    verification: string;
	    evidence: ReviewEvidence[];
	    filters: string[];
	    warnings: string[];
	    basis: string;

	    static createFrom(source: any = {}) {
	        return new ReviewFinding(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.ref = source["ref"];
	        this.statement = source["statement"];
	        this.confidence = source["confidence"];
	        this.rationale = source["rationale"];
	        this.status = source["status"];
	        this.verification = source["verification"];
	        this.evidence = this.convertValues(source["evidence"], ReviewEvidence);
	        this.filters = source["filters"];
	        this.warnings = source["warnings"];
	        this.basis = source["basis"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ReviewStatementInput {
	    kind: string;
	    ref: string;
	    basis: string;
	    verification: string;
	    settle_dispute: boolean;

	    static createFrom(source: any = {}) {
	        return new ReviewStatementInput(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.ref = source["ref"];
	        this.basis = source["basis"];
	        this.verification = source["verification"];
	        this.settle_dispute = source["settle_dispute"];
	    }
	}
	export class ShareCandidate {
	    ref: string;
	    text: string;
	    group: string;
	    label: string;
	    required: boolean;
	    selected: boolean;

	    static createFrom(source: any = {}) {
	        return new ShareCandidate(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ref = source["ref"];
	        this.text = source["text"];
	        this.group = source["group"];
	        this.label = source["label"];
	        this.required = source["required"];
	        this.selected = source["selected"];
	    }
	}
	export class ShareOptions {
	    mode: string;
	    claim_refs: string[];
	    hypothesis_refs: string[];
	    question_refs: string[];
	    ruled_out_refs: string[];

	    static createFrom(source: any = {}) {
	        return new ShareOptions(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.claim_refs = source["claim_refs"];
	        this.hypothesis_refs = source["hypothesis_refs"];
	        this.question_refs = source["question_refs"];
	        this.ruled_out_refs = source["ruled_out_refs"];
	    }
	}
	export class SharePreview {
	    markdown: string;
	    html: string;
	    snapshot_id: string;
	    investigation_ref: string;
	    mode: string;
	    candidates: ShareCandidate[];
	    omitted: contextpack.BriefOmitted;

	    static createFrom(source: any = {}) {
	        return new SharePreview(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.markdown = source["markdown"];
	        this.html = source["html"];
	        this.snapshot_id = source["snapshot_id"];
	        this.investigation_ref = source["investigation_ref"];
	        this.mode = source["mode"];
	        this.candidates = this.convertValues(source["candidates"], ShareCandidate);
	        this.omitted = this.convertValues(source["omitted"], contextpack.BriefOmitted);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace contextpack {

	export class BriefOmitted {
	    claims: number;
	    hypotheses: number;
	    questions: number;
	    evidence: number;
	    claim_refs: string[];
	    hypothesis_refs: string[];
	    question_refs: string[];
	    evidence_refs: string[];

	    static createFrom(source: any = {}) {
	        return new BriefOmitted(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.claims = source["claims"];
	        this.hypotheses = source["hypotheses"];
	        this.questions = source["questions"];
	        this.evidence = source["evidence"];
	        this.claim_refs = source["claim_refs"];
	        this.hypothesis_refs = source["hypothesis_refs"];
	        this.question_refs = source["question_refs"];
	        this.evidence_refs = source["evidence_refs"];
	    }
	}

}

export namespace domain {

	export class Actor {
	    kind: string;
	    client: string;

	    static createFrom(source: any = {}) {
	        return new Actor(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.client = source["client"];
	    }
	}
	export class Activity {
	    id: number;
	    investigation_ref: string;
	    actor: Actor;
	    action: string;
	    target_ref: string;
	    summary: string;
	    note: string;
	    created_at: string;
	    imported: boolean;

	    static createFrom(source: any = {}) {
	        return new Activity(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.investigation_ref = source["investigation_ref"];
	        this.actor = this.convertValues(source["actor"], Actor);
	        this.action = source["action"];
	        this.target_ref = source["target_ref"];
	        this.summary = source["summary"];
	        this.note = source["note"];
	        this.created_at = source["created_at"];
	        this.imported = source["imported"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

	export class Counts {
	    investigations: number;
	    evidence: number;
	    claims: number;
	    hypotheses: number;
	    questions: number;
	    claim_links: number;
	    hypothesis_links: number;
	    activity: number;

	    static createFrom(source: any = {}) {
	        return new Counts(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.investigations = source["investigations"];
	        this.evidence = source["evidence"];
	        this.claims = source["claims"];
	        this.hypotheses = source["hypotheses"];
	        this.questions = source["questions"];
	        this.claim_links = source["claim_links"];
	        this.hypothesis_links = source["hypothesis_links"];
	        this.activity = source["activity"];
	    }
	}
	export class BackupInfo {
	    path: string;
	    size_bytes: number;
	    schema_version: number;
	    created_at: string;
	    counts: Counts;

	    static createFrom(source: any = {}) {
	        return new BackupInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.size_bytes = source["size_bytes"];
	        this.schema_version = source["schema_version"];
	        this.created_at = source["created_at"];
	        this.counts = this.convertValues(source["counts"], Counts);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class EvidenceLink {
	    evidence_ref: string;
	    relation: string;
	    evidence_title: string;
	    created_at: string;
	    created_by: Actor;

	    static createFrom(source: any = {}) {
	        return new EvidenceLink(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.evidence_ref = source["evidence_ref"];
	        this.relation = source["relation"];
	        this.evidence_title = source["evidence_title"];
	        this.created_at = source["created_at"];
	        this.created_by = this.convertValues(source["created_by"], Actor);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Claim {
	    id: number;
	    ref: string;
	    investigation_ref: string;
	    statement: string;
	    confidence: string;
	    confidence_rationale: string;
	    status: string;
	    verification: string;
	    superseded_by: string;
	    created_at: string;
	    updated_at: string;
	    created_by: Actor;
	    imported: boolean;
	    links: EvidenceLink[];

	    static createFrom(source: any = {}) {
	        return new Claim(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.ref = source["ref"];
	        this.investigation_ref = source["investigation_ref"];
	        this.statement = source["statement"];
	        this.confidence = source["confidence"];
	        this.confidence_rationale = source["confidence_rationale"];
	        this.status = source["status"];
	        this.verification = source["verification"];
	        this.superseded_by = source["superseded_by"];
	        this.created_at = source["created_at"];
	        this.updated_at = source["updated_at"];
	        this.created_by = this.convertValues(source["created_by"], Actor);
	        this.imported = source["imported"];
	        this.links = this.convertValues(source["links"], EvidenceLink);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class EvidenceLinkInput {
	    evidence_ref: string;
	    relation: string;

	    static createFrom(source: any = {}) {
	        return new EvidenceLinkInput(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.evidence_ref = source["evidence_ref"];
	        this.relation = source["relation"];
	    }
	}
	export class ClaimInput {
	    statement: string;
	    confidence: string;
	    confidence_rationale: string;
	    status: string;
	    verification: string;
	    links: EvidenceLinkInput[];

	    static createFrom(source: any = {}) {
	        return new ClaimInput(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.statement = source["statement"];
	        this.confidence = source["confidence"];
	        this.confidence_rationale = source["confidence_rationale"];
	        this.status = source["status"];
	        this.verification = source["verification"];
	        this.links = this.convertValues(source["links"], EvidenceLinkInput);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ClaimPatch {
	    statement?: string;
	    confidence_rationale?: string;
	    confidence?: string;
	    status?: string;
	    verification?: string;
	    superseded_by?: string;

	    static createFrom(source: any = {}) {
	        return new ClaimPatch(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.statement = source["statement"];
	        this.confidence_rationale = source["confidence_rationale"];
	        this.confidence = source["confidence"];
	        this.status = source["status"];
	        this.verification = source["verification"];
	        this.superseded_by = source["superseded_by"];
	    }
	}
	export class ConclusionInput {
	    conclusion: string;
	    confidence: string;
	    rationale: string;
	    verification?: string;
	    supporting_claims?: string[];
	    contradicting_claims?: string[];
	    review?: boolean;
	    expected_review_token?: string;
	    review_basis?: string;

	    static createFrom(source: any = {}) {
	        return new ConclusionInput(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.conclusion = source["conclusion"];
	        this.confidence = source["confidence"];
	        this.rationale = source["rationale"];
	        this.verification = source["verification"];
	        this.supporting_claims = source["supporting_claims"];
	        this.contradicting_claims = source["contradicting_claims"];
	        this.review = source["review"];
	        this.expected_review_token = source["expected_review_token"];
	        this.review_basis = source["review_basis"];
	    }
	}

	export class CreateInvestigationInput {
	    title: string;
	    problem_statement: string;
	    status: string;
	    make_active: boolean;

	    static createFrom(source: any = {}) {
	        return new CreateInvestigationInput(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.problem_statement = source["problem_statement"];
	        this.status = source["status"];
	        this.make_active = source["make_active"];
	    }
	}
	export class DetailOptions {
	    activity_limit: number;
	    include_raw_detail: boolean;

	    static createFrom(source: any = {}) {
	        return new DetailOptions(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.activity_limit = source["activity_limit"];
	        this.include_raw_detail = source["include_raw_detail"];
	    }
	}
	export class RecordLink {
	    ref: string;
	    relation: string;

	    static createFrom(source: any = {}) {
	        return new RecordLink(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ref = source["ref"];
	        this.relation = source["relation"];
	    }
	}
	export class Evidence {
	    id: number;
	    ref: string;
	    investigation_ref: string;
	    title: string;
	    summary: string;
	    raw_detail?: string;
	    has_raw_detail: boolean;
	    source_type: string;
	    source_ref: string;
	    source_label: string;
	    captured_at: string;
	    created_at: string;
	    updated_at: string;
	    created_by: Actor;
	    imported: boolean;
	    verification: string;
	    metadata: string;
	    claim_links: RecordLink[];
	    hypothesis_links: RecordLink[];

	    static createFrom(source: any = {}) {
	        return new Evidence(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.ref = source["ref"];
	        this.investigation_ref = source["investigation_ref"];
	        this.title = source["title"];
	        this.summary = source["summary"];
	        this.raw_detail = source["raw_detail"];
	        this.has_raw_detail = source["has_raw_detail"];
	        this.source_type = source["source_type"];
	        this.source_ref = source["source_ref"];
	        this.source_label = source["source_label"];
	        this.captured_at = source["captured_at"];
	        this.created_at = source["created_at"];
	        this.updated_at = source["updated_at"];
	        this.created_by = this.convertValues(source["created_by"], Actor);
	        this.imported = source["imported"];
	        this.verification = source["verification"];
	        this.metadata = source["metadata"];
	        this.claim_links = this.convertValues(source["claim_links"], RecordLink);
	        this.hypothesis_links = this.convertValues(source["hypothesis_links"], RecordLink);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class EvidenceInput {
	    title: string;
	    summary: string;
	    raw_detail: string;
	    source_type: string;
	    source_ref: string;
	    source_label: string;
	    captured_at: string;
	    verification: string;
	    metadata: string;

	    static createFrom(source: any = {}) {
	        return new EvidenceInput(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.summary = source["summary"];
	        this.raw_detail = source["raw_detail"];
	        this.source_type = source["source_type"];
	        this.source_ref = source["source_ref"];
	        this.source_label = source["source_label"];
	        this.captured_at = source["captured_at"];
	        this.verification = source["verification"];
	        this.metadata = source["metadata"];
	    }
	}


	export class EvidencePatch {
	    title?: string;
	    summary?: string;
	    raw_detail?: string;
	    source_type?: string;
	    source_ref?: string;
	    source_label?: string;
	    captured_at?: string;
	    verification?: string;
	    metadata?: string;

	    static createFrom(source: any = {}) {
	        return new EvidencePatch(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.summary = source["summary"];
	        this.raw_detail = source["raw_detail"];
	        this.source_type = source["source_type"];
	        this.source_ref = source["source_ref"];
	        this.source_label = source["source_label"];
	        this.captured_at = source["captured_at"];
	        this.verification = source["verification"];
	        this.metadata = source["metadata"];
	    }
	}
	export class GetEvidenceOptions {
	    include_raw_detail: boolean;

	    static createFrom(source: any = {}) {
	        return new GetEvidenceOptions(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.include_raw_detail = source["include_raw_detail"];
	    }
	}
	export class Hypothesis {
	    id: number;
	    ref: string;
	    investigation_ref: string;
	    statement: string;
	    confidence: string;
	    rationale: string;
	    status: string;
	    verification: string;
	    superseded_by: string;
	    created_at: string;
	    updated_at: string;
	    created_by: Actor;
	    imported: boolean;
	    links: EvidenceLink[];

	    static createFrom(source: any = {}) {
	        return new Hypothesis(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.ref = source["ref"];
	        this.investigation_ref = source["investigation_ref"];
	        this.statement = source["statement"];
	        this.confidence = source["confidence"];
	        this.rationale = source["rationale"];
	        this.status = source["status"];
	        this.verification = source["verification"];
	        this.superseded_by = source["superseded_by"];
	        this.created_at = source["created_at"];
	        this.updated_at = source["updated_at"];
	        this.created_by = this.convertValues(source["created_by"], Actor);
	        this.imported = source["imported"];
	        this.links = this.convertValues(source["links"], EvidenceLink);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class HypothesisInput {
	    statement: string;
	    confidence: string;
	    rationale: string;
	    status: string;
	    verification: string;
	    links: EvidenceLinkInput[];

	    static createFrom(source: any = {}) {
	        return new HypothesisInput(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.statement = source["statement"];
	        this.confidence = source["confidence"];
	        this.rationale = source["rationale"];
	        this.status = source["status"];
	        this.verification = source["verification"];
	        this.links = this.convertValues(source["links"], EvidenceLinkInput);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class HypothesisPatch {
	    statement?: string;
	    rationale?: string;
	    confidence?: string;
	    status?: string;
	    verification?: string;
	    superseded_by?: string;

	    static createFrom(source: any = {}) {
	        return new HypothesisPatch(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.statement = source["statement"];
	        this.rationale = source["rationale"];
	        this.confidence = source["confidence"];
	        this.status = source["status"];
	        this.verification = source["verification"];
	        this.superseded_by = source["superseded_by"];
	    }
	}
	export class ImportResult {
	    investigation_ref: string;
	    title: string;
	    original_ref: string;
	    ref_changed: boolean;
	    counts: Counts;
	    same_case_refs?: string[];

	    static createFrom(source: any = {}) {
	        return new ImportResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.investigation_ref = source["investigation_ref"];
	        this.title = source["title"];
	        this.original_ref = source["original_ref"];
	        this.ref_changed = source["ref_changed"];
	        this.counts = this.convertValues(source["counts"], Counts);
	        this.same_case_refs = source["same_case_refs"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Investigation {
	    id: number;
	    ref: string;
	    title: string;
	    problem_statement: string;
	    status: string;
	    conclusion: string;
	    conclusion_confidence: string;
	    conclusion_rationale: string;
	    conclusion_verification: string;
	    conclusion_supporting_claims: string[];
	    conclusion_contradicting_claims: string[];
	    conclusion_reviewed_at: string;
	    conclusion_review_basis: string;
	    conclusion_review_token: string;
	    conclusion_needs_review: boolean;
	    created_at: string;
	    updated_at: string;
	    archived_at: string;
	    archived: boolean;
	    is_active: boolean;
	    created_by: Actor;

	    static createFrom(source: any = {}) {
	        return new Investigation(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.ref = source["ref"];
	        this.title = source["title"];
	        this.problem_statement = source["problem_statement"];
	        this.status = source["status"];
	        this.conclusion = source["conclusion"];
	        this.conclusion_confidence = source["conclusion_confidence"];
	        this.conclusion_rationale = source["conclusion_rationale"];
	        this.conclusion_verification = source["conclusion_verification"];
	        this.conclusion_supporting_claims = source["conclusion_supporting_claims"];
	        this.conclusion_contradicting_claims = source["conclusion_contradicting_claims"];
	        this.conclusion_reviewed_at = source["conclusion_reviewed_at"];
	        this.conclusion_review_basis = source["conclusion_review_basis"];
	        this.conclusion_review_token = source["conclusion_review_token"];
	        this.conclusion_needs_review = source["conclusion_needs_review"];
	        this.created_at = source["created_at"];
	        this.updated_at = source["updated_at"];
	        this.archived_at = source["archived_at"];
	        this.archived = source["archived"];
	        this.is_active = source["is_active"];
	        this.created_by = this.convertValues(source["created_by"], Actor);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class RecordReviewBasis {
	    ref: string;
	    basis: string;

	    static createFrom(source: any = {}) {
	        return new RecordReviewBasis(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ref = source["ref"];
	        this.basis = source["basis"];
	    }
	}
	export class Question {
	    id: number;
	    ref: string;
	    investigation_ref: string;
	    question: string;
	    status: string;
	    resolution: string;
	    defer_note: string;
	    resolved_at: string;
	    created_at: string;
	    updated_at: string;
	    created_by: Actor;
	    imported: boolean;

	    static createFrom(source: any = {}) {
	        return new Question(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.ref = source["ref"];
	        this.investigation_ref = source["investigation_ref"];
	        this.question = source["question"];
	        this.status = source["status"];
	        this.resolution = source["resolution"];
	        this.defer_note = source["defer_note"];
	        this.resolved_at = source["resolved_at"];
	        this.created_at = source["created_at"];
	        this.updated_at = source["updated_at"];
	        this.created_by = this.convertValues(source["created_by"], Actor);
	        this.imported = source["imported"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class InvestigationDetail {
	    investigation: Investigation;
	    evidence: Evidence[];
	    claims: Claim[];
	    hypotheses: Hypothesis[];
	    questions: Question[];
	    recent_activity: Activity[];
	    review_bases: RecordReviewBasis[];

	    static createFrom(source: any = {}) {
	        return new InvestigationDetail(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.investigation = this.convertValues(source["investigation"], Investigation);
	        this.evidence = this.convertValues(source["evidence"], Evidence);
	        this.claims = this.convertValues(source["claims"], Claim);
	        this.hypotheses = this.convertValues(source["hypotheses"], Hypothesis);
	        this.questions = this.convertValues(source["questions"], Question);
	        this.recent_activity = this.convertValues(source["recent_activity"], Activity);
	        this.review_bases = this.convertValues(source["review_bases"], RecordReviewBasis);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class InvestigationPatch {
	    title?: string;
	    problem_statement?: string;
	    status?: string;

	    static createFrom(source: any = {}) {
	        return new InvestigationPatch(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.problem_statement = source["problem_statement"];
	        this.status = source["status"];
	    }
	}
	export class RecordCounts {
	    evidence: number;
	    claims: number;
	    hypotheses: number;
	    questions: number;
	    open_questions: number;

	    static createFrom(source: any = {}) {
	        return new RecordCounts(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.evidence = source["evidence"];
	        this.claims = source["claims"];
	        this.hypotheses = source["hypotheses"];
	        this.questions = source["questions"];
	        this.open_questions = source["open_questions"];
	    }
	}
	export class InvestigationSummary {
	    id: number;
	    ref: string;
	    title: string;
	    status: string;
	    conclusion_confidence: string;
	    created_at: string;
	    updated_at: string;
	    archived_at: string;
	    archived: boolean;
	    is_active: boolean;
	    counts: RecordCounts;

	    static createFrom(source: any = {}) {
	        return new InvestigationSummary(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.ref = source["ref"];
	        this.title = source["title"];
	        this.status = source["status"];
	        this.conclusion_confidence = source["conclusion_confidence"];
	        this.created_at = source["created_at"];
	        this.updated_at = source["updated_at"];
	        this.archived_at = source["archived_at"];
	        this.archived = source["archived"];
	        this.is_active = source["is_active"];
	        this.counts = this.convertValues(source["counts"], RecordCounts);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ListOptions {
	    include_archived: boolean;
	    status: string;
	    limit: number;

	    static createFrom(source: any = {}) {
	        return new ListOptions(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.include_archived = source["include_archived"];
	        this.status = source["status"];
	        this.limit = source["limit"];
	    }
	}

	export class QuestionInput {
	    question: string;

	    static createFrom(source: any = {}) {
	        return new QuestionInput(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.question = source["question"];
	    }
	}



	export class RestoreResult {
	    source_path: string;
	    safety_copy_path: string;
	    source_schema_version: number;
	    counts: Counts;

	    static createFrom(source: any = {}) {
	        return new RestoreResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source_path = source["source_path"];
	        this.safety_copy_path = source["safety_copy_path"];
	        this.source_schema_version = source["source_schema_version"];
	        this.counts = this.convertValues(source["counts"], Counts);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SnippetSegment {
	    text: string;
	    match: boolean;

	    static createFrom(source: any = {}) {
	        return new SnippetSegment(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.text = source["text"];
	        this.match = source["match"];
	    }
	}
	export class SearchHit {
	    investigation_ref: string;
	    investigation_title: string;
	    record_type: string;
	    record_ref: string;
	    snippet: string;
	    segments: SnippetSegment[];
	    rank: number;

	    static createFrom(source: any = {}) {
	        return new SearchHit(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.investigation_ref = source["investigation_ref"];
	        this.investigation_title = source["investigation_title"];
	        this.record_type = source["record_type"];
	        this.record_ref = source["record_ref"];
	        this.snippet = source["snippet"];
	        this.segments = this.convertValues(source["segments"], SnippetSegment);
	        this.rank = source["rank"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SearchOptions {
	    investigation_ref: string;
	    include_archived: boolean;
	    limit: number;

	    static createFrom(source: any = {}) {
	        return new SearchOptions(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.investigation_ref = source["investigation_ref"];
	        this.include_archived = source["include_archived"];
	        this.limit = source["limit"];
	    }
	}

}

export namespace main {

	export class BackupOutcome {
	    cancelled: boolean;
	    info: domain.BackupInfo;

	    static createFrom(source: any = {}) {
	        return new BackupOutcome(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cancelled = source["cancelled"];
	        this.info = this.convertValues(source["info"], domain.BackupInfo);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ClientConnection {
	    configured: boolean;
	    matches_cli: boolean;
	    command: string;
	    command_exists: boolean;
	    config_path: string;
	    config_problem: boolean;
	    hooks: string[];
	    setup_command: string;
	    setup_command_cmd: string;
	    setup_command_wsl: string;
	    snippet: string;

	    static createFrom(source: any = {}) {
	        return new ClientConnection(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.configured = source["configured"];
	        this.matches_cli = source["matches_cli"];
	        this.command = source["command"];
	        this.command_exists = source["command_exists"];
	        this.config_path = source["config_path"];
	        this.config_problem = source["config_problem"];
	        this.hooks = source["hooks"];
	        this.setup_command = source["setup_command"];
	        this.setup_command_cmd = source["setup_command_cmd"];
	        this.setup_command_wsl = source["setup_command_wsl"];
	        this.snippet = source["snippet"];
	    }
	}
	export class ClientConnections {
	    cli_path: string;
	    cli_source: string;
	    cli_name: string;
	    db_path: string;
	    shell: string;
	    claude: ClientConnection;
	    codex: ClientConnection;

	    static createFrom(source: any = {}) {
	        return new ClientConnections(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cli_path = source["cli_path"];
	        this.cli_source = source["cli_source"];
	        this.cli_name = source["cli_name"];
	        this.db_path = source["db_path"];
	        this.shell = source["shell"];
	        this.claude = this.convertValues(source["claude"], ClientConnection);
	        this.codex = this.convertValues(source["codex"], ClientConnection);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class FileResult {
	    cancelled: boolean;
	    path: string;
	    file_name: string;

	    static createFrom(source: any = {}) {
	        return new FileResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cancelled = source["cancelled"];
	        this.path = source["path"];
	        this.file_name = source["file_name"];
	    }
	}
	export class ImportOutcome {
	    cancelled: boolean;
	    file_name: string;
	    title: string;
	    result: domain.ImportResult;

	    static createFrom(source: any = {}) {
	        return new ImportOutcome(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cancelled = source["cancelled"];
	        this.file_name = source["file_name"];
	        this.title = source["title"];
	        this.result = this.convertValues(source["result"], domain.ImportResult);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ReportPreview {
	    markdown: string;
	    html: string;

	    static createFrom(source: any = {}) {
	        return new ReportPreview(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.markdown = source["markdown"];
	        this.html = source["html"];
	    }
	}
	export class RestorePick {
	    cancelled: boolean;
	    path: string;
	    file_name: string;
	    info: domain.BackupInfo;

	    static createFrom(source: any = {}) {
	        return new RestorePick(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cancelled = source["cancelled"];
	        this.path = source["path"];
	        this.file_name = source["file_name"];
	        this.info = this.convertValues(source["info"], domain.BackupInfo);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class StartupInfo {
	    ok: boolean;
	    kind: string;
	    message: string;
	    db_path: string;
	    recoverable: boolean;

	    static createFrom(source: any = {}) {
	        return new StartupInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.kind = source["kind"];
	        this.message = source["message"];
	        this.db_path = source["db_path"];
	        this.recoverable = source["recoverable"];
	    }
	}

}

export namespace update {

	export class Asset {
	    name: string;
	    url: string;
	    size_bytes: number;

	    static createFrom(source: any = {}) {
	        return new Asset(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.url = source["url"];
	        this.size_bytes = source["size_bytes"];
	    }
	}
	export class Result {
	    current: string;
	    latest: string;
	    state: string;
	    release_url: string;
	    // Go type: time
	    published_at: any;
	    asset?: Asset;
	    // Go type: time
	    checked_at: any;

	    static createFrom(source: any = {}) {
	        return new Result(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.current = source["current"];
	        this.latest = source["latest"];
	        this.state = source["state"];
	        this.release_url = source["release_url"];
	        this.published_at = this.convertValues(source["published_at"], null);
	        this.asset = this.convertValues(source["asset"], Asset);
	        this.checked_at = this.convertValues(source["checked_at"], null);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

