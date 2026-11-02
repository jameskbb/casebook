# Casebook build brief

This is the brief the first build was made from, kept as written. It addresses the AI that built the repository, and "GSD" is the planning tool used for that build.

The original product brief for Casebook, saved as the source of truth for the first build (2026-10-01). Its worked example was later replaced with a fictional case (a September cloud bill investigation) so the repository carries no real-world example; the requirements are unchanged.

---

Build Casebook - Local-First Evidence-Based Investigation System

I want you to take ownership of designing and implementing a new open-source project called Casebook.

This should be treated as a serious product build, not a throwaway prototype.

I want you to initialize this repository as a GSD project, establish the product requirements and architecture, create a realistic implementation roadmap, and then immediately execute against that roadmap as far as possible in this session.

Do not stop after planning, scaffolding, or producing documentation.

Make reasonable product and technical decisions without asking me questions unless you are genuinely blocked. Record important assumptions and architectural decisions in the GSD project documentation.

Use subagents where they materially improve quality, especially for architecture review, MCP integration, desktop UX, testing, and a final adversarial review. However, do not build any multi-agent orchestration framework into Casebook itself.

⸻

Product Vision

Casebook is a local-first evidence and investigation layer for AI-assisted problem solving.

The core problem is that difficult investigations increasingly span:

* multiple AI sessions
* multiple AI clients
* multiple days
* multiple source systems
* multiple hypotheses
* multiple people

Today, the useful state of an investigation tends to live inside chat history.

That is a poor representation of knowledge.

A long conversation may contain:

* incorrect hypotheses
* dead ends
* outdated assumptions
* large tool outputs
* corrected mistakes
* intermediate reasoning
* a small number of actual durable findings

Casebook should externalize the durable investigation state.

The investigation itself becomes the persistent object.

AI clients become interchangeable interfaces into that investigation.

A user should be able to begin investigating a problem in Claude Code, continue it later through Codex, inspect or edit the findings in the Casebook desktop application, and have every client operate against the same underlying evidence and investigation state.

The fundamental idea is:

Do not preserve the agent's entire reasoning path. Preserve the evidence, claims, hypotheses, unanswered questions, conclusions, provenance, and uncertainty that matter.

Casebook is not intended to capture or expose hidden chain-of-thought.

It should store concise, inspectable reasoning artifacts.

⸻

Primary Use Case

A user is investigating a difficult business or technical question.

Example:

Why did our cloud bill rise 38% in September when traffic was flat?

During investigation, an AI might discover:

* a shared platform default switched every service to debug logging
* log ingestion, billed per GB, accounts for most of the increase
* 214 services are currently affected
* a new tracing agent may also be shipping some logs twice
* additional validation is still required

Instead of this information living only inside a Claude conversation, Casebook should persist it as structured investigation state.

For example:

Current conclusion

Debug logging requested for payments only reached every service and drove most of the increase.

Confidence: High

Established claims

Claim C-001:

A shared platform default moved every service to debug logging on August 28; only payments asked for it.

Confidence: High

Supported by:

* Evidence E-001
* Evidence E-004

Claim C-002:

Log ingestion, billed per GB, is $96,400 of the $135,800 increase; compute and network are flat.

Confidence: High

Supported by:

* Evidence E-002
* Evidence E-003

Hypothesis

H-001:

The new tracing agent may also be shipping some services' logs twice.

Confidence: Medium

Status: Unresolved

Open question

Q-001:

Can every service except payments go back to info level today?

Then tomorrow another AI client can load this state and continue from there without needing the entire original conversation.

⸻

Product Principles

Casebook should follow these principles aggressively.

1. Investigation state belongs to the user

It does not belong to:

* Claude
* Codex
* a particular chat
* a particular model
* a particular vendor

The Casebook database is the durable source of truth.

2. Local-first

V1 should require:

* no hosted service
* no cloud database
* no account
* no SaaS backend
* no authentication server
* no telemetry

Everything should function locally.

The application should work offline except when an external AI client or external data source naturally requires network access.

3. One local knowledge engine

Do not create separate implementations for the desktop application, MCP, CLI, and hooks.

They must all call the same Go domain/application layer and operate against the same database.

4. Evidence over transcript

Do not store gigantic AI transcripts as the primary knowledge model.

Casebook should emphasize:

Evidence -> Claims -> Conclusion

while also preserving:

* hypotheses
* contradictions
* open questions
* disproven ideas
* provenance
* confidence

5. Confidence belongs to individual claims

Do not only provide one overall confidence score.

Claims and hypotheses should each be capable of carrying:

* confidence
* confidence rationale
* status
* supporting evidence
* contradicting evidence

Use an understandable V1 confidence model:

* Low
* Medium
* High
* Unknown / Not assessed

Do not manufacture fake mathematical precision.

6. Human verification must be possible

AI-created knowledge should not automatically become unquestioned truth.

Records should support states such as:

* AI generated
* Human verified
* Source verified
* Disputed
* Superseded

Keep the initial implementation simple, but ensure the schema can represent this cleanly.

7. No over-engineering

This project is specifically intended to avoid mistakes made in more orchestration-heavy agent tooling.

Do NOT build:

* DAG execution
* workflow engines
* agent orchestration
* schedulers
* prompt pipelines
* approval graphs
* general-purpose task runners
* event-sourcing frameworks
* distributed systems
* microservices
* vector databases unless there is a demonstrated need

Casebook should be boring, reliable infrastructure around AI.

⸻

Technology Direction

Use:

* Go for the local application core
* SQLite for persistence
* Wails for the desktop application
* React + TypeScript for the Wails frontend
* MCP over stdio for local AI clients
* Go CLI commands for local tooling and hooks

Use current stable releases and verify current official documentation where necessary.

Prefer a SQLite implementation that minimizes cross-platform packaging pain. Avoid unnecessary CGO dependencies if a mature pure-Go option meets the requirements.

The target operating systems are:

* Windows x64
* Windows ARM64 where practical
* macOS Apple Silicon
* macOS Intel where practical

End users must NOT need Go installed.

The product should ultimately be distributed as a normal application:

Windows:

Casebook installer / executable

macOS:

Casebook.app / DMG

The development toolchain is irrelevant to the end-user experience.

⸻

Important Packaging Principle

Casebook should feel like an application, not a development environment.

A senior business leader should never need to:

* install Go
* install Python
* install Node
* create a virtual environment
* run a package manager
* open a terminal
* start a development server
* select a port
* manage a database
* understand MCP configuration manually

The eventual experience should be approximately:

1. Install Casebook.
2. Open Casebook.
3. Use it.

Advanced integrations may expose additional setup, but the base desktop application must behave like a normal installed application.

⸻

Architecture

Build Casebook as a modular monolith.

Conceptually:

                       CASEBOOK
                    Desktop App
                        |
                +-------+-------+
                |               |
             React UI         Go Core
                                |
                   +------------+------------+
                   |            |            |
                SQLite         MCP          CLI
                                |            |
                         Claude Code      Hooks
                         Codex            Utilities
                         Other MCP
                         clients

The UI, MCP server, CLI commands, and hooks must all call shared application/domain services.

Do not duplicate persistence or business logic.

It is acceptable for one installed Casebook product to contain a small companion CLI/MCP executable if platform GUI constraints make a single binary awkward.

If you do that:

* both executables must use the same Go packages
* both must use the same database
* business logic must not be duplicated
* installation should still feel like one product

Choose the cleanest implementation.

Document the decision.

⸻

Core Domain Model

Design this carefully before implementation.

At minimum Casebook needs the following concepts.

Investigation

Represents the durable case being investigated.

Suggested fields:

* ID
* Title
* Problem statement / primary question
* Status
* Current conclusion
* Conclusion confidence
* Conclusion confidence rationale
* Created timestamp
* Updated timestamp
* Archived timestamp if applicable
* Active state
* Created by / source where useful

Suggested statuses:

* New
* Investigating
* Blocked
* Resolved
* Archived

Evidence

An observation or source artifact.

Evidence may originate from:

* SQL query result
* code
* file
* Jira ticket
* documentation
* API response
* user statement
* AI tool output
* manual observation
* external URL
* another Casebook artifact

Suggested fields:

* ID
* Investigation ID
* Title
* Concise content / summary
* Optional raw detail
* Source type
* Source reference / URI / path
* Source label
* Captured timestamp
* Created timestamp
* Created by
* Verification state
* Metadata JSON where useful

Evidence should support provenance.

Avoid blindly storing enormous raw outputs.

Claim

A statement believed to be supported by evidence.

Suggested fields:

* ID
* Investigation ID
* Statement
* Confidence
* Confidence rationale
* Status
* Verification state
* Created timestamp
* Updated timestamp
* Created by

Suggested status:

* Active
* Disputed
* Superseded
* Rejected

Claim <-> Evidence relationship

This is important.

Evidence should be able to:

* support a claim
* contradict a claim

Do not flatten this relationship into an opaque text field.

Create an explicit relationship model.

Hypothesis

A possible explanation that has not been established.

Suggested fields:

* ID
* Investigation ID
* Statement
* Confidence
* Rationale
* Status
* Created timestamp
* Updated timestamp

Suggested statuses:

* Open
* Supported
* Disproven
* Superseded

Hypotheses should also be linkable to supporting or contradicting evidence.

Question

Something that remains unresolved.

Suggested fields:

* ID
* Investigation ID
* Question
* Status
* Resolution
* Created timestamp
* Resolved timestamp

Suggested statuses:

* Open
* Resolved
* Deferred

Provenance / Activity

Preserve useful attribution such as:

* Claude Code
* Codex
* Casebook UI
* human user
* imported source
* MCP client identifier where available

Do not build a giant audit/event-sourcing system.

A simple useful activity history is enough.

⸻

Context Packs

One of Casebook's most important functions is transforming a potentially large investigation into a concise reusable context package for another AI.

Implement a deterministic context generator.

It should be available through:

* desktop UI
* CLI
* MCP

Example output:

Investigation: INV-0042
Title: September cloud bill up 38%
Primary question:
Why did the cloud bill rise 38% in September when traffic was flat?
Current conclusion:
Debug logging requested for payments only reached every service and drove most of the increase.
Conclusion confidence:
High
Established claims:
C-001 [High]
A shared platform default moved every service to debug logging on
August 28; only payments asked for it.
Evidence: E-001, E-004
C-002 [High]
Log ingestion, billed per GB, is $96,400 of the $135,800 increase;
compute and network are flat.
Evidence: E-002, E-003
Hypotheses:
H-001 [Medium, Open]
The new tracing agent may also be shipping some services' logs twice.
Open questions:
Q-001
Can every service except payments go back to info level today?
Known disproven hypotheses:
H-002
...
Recent material changes:
...

The context pack should prioritize current durable state, not historical chatter.

Where practical, support both:

* human-readable Markdown
* structured JSON

⸻

Desktop Application

Build a real desktop UI using Wails + React + TypeScript.

The first version should be visually polished enough that I could hand it to a business leader without apologizing for it.

Do not over-design it, but do not make it look like an internal admin panel.

Main screen

Show:

* investigations
* status
* current confidence
* last updated
* search
* create investigation
* archive state

Investigation detail

A selected investigation should clearly surface:

1. Primary question
2. Current conclusion
3. Current conclusion confidence
4. Claims
5. Hypotheses
6. Evidence
7. Open questions
8. Activity / provenance

The hierarchy should make it immediately obvious:

What do we believe?
Why do we believe it?
How sure are we?
What still needs to be figured out?

Editing

Users must be able to manually:

* create an investigation
* edit the problem statement
* add evidence
* add claims
* link evidence to claims
* mark evidence as supporting or contradicting
* create hypotheses
* resolve/disprove hypotheses
* add questions
* resolve questions
* update conclusion
* update confidence
* verify/dispute/supersede records

Active investigation

Support the concept of an active investigation.

This is important for CLI, MCP, and hooks.

The user should be able to say:

Make this the active investigation.

Then local AI integrations can retrieve that investigation without requiring the user to repeatedly provide an ID.

Context

Provide a prominent:

Copy Context

or equivalent action.

It should generate the current investigation context pack.

Import / Export

Support:

* export investigation as JSON
* import investigation JSON
* backup all Casebook data
* restore backup

Use versioned export formats.

⸻

MCP Integration

MCP is a first-class interface, not an afterthought.

Implement a local MCP server using stdio.

The goal is for Casebook to connect cleanly to local clients such as:

* Claude Code
* Codex
* future MCP-capable clients

Do not tie the MCP implementation to one AI vendor.

Suggested tools:

casebook_list_investigations
casebook_search_investigations
casebook_get_investigation
casebook_create_investigation
casebook_get_active_investigation
casebook_set_active_investigation
casebook_add_evidence
casebook_get_evidence
casebook_add_claim
casebook_update_claim
casebook_link_evidence_to_claim
casebook_add_hypothesis
casebook_update_hypothesis
casebook_add_question
casebook_resolve_question
casebook_update_conclusion
casebook_get_context
casebook_checkpoint

Names can change if there is a strong MCP convention reason.

Tool descriptions must be excellent because AI clients depend heavily on them.

Each mutating tool should return useful structured results.

Avoid giant MCP responses.

⸻

Claude Code Integration

Claude Code is one of the primary power-user environments.

Casebook should support optional Claude Code hooks.

The hooks should make investigation state feel persistent across sessions.

Session start / pre-work behavior

When Claude Code starts, Casebook should be able to provide:

* active investigation
* primary question
* current conclusion
* claims
* hypotheses
* open questions
* compact evidence references

Do not inject the entire database.

Generate a concise context pack.

Post-work / checkpoint behavior

At the end of work, Casebook should help Claude persist material discoveries.

The goal is NOT:

Save the whole transcript.

The goal is:

* persist new evidence
* create or update claims
* mark contradicted claims
* update hypotheses
* mark disproven hypotheses
* resolve questions
* add new questions
* update the conclusion when justified

Hooks should preserve provenance and encourage checkpointing.

MCP should remain the primary mechanism by which the model writes structured Casebook state.

Setup UX

Create a command or mechanism such as:

casebook setup claude

that can assist a technical user in configuring:

* Casebook MCP
* optional Casebook hooks

It must be conservative.

Do not blindly overwrite an existing Claude configuration.

If modifying configuration:

* read existing configuration
* preserve existing user settings
* merge only Casebook-owned values
* back up files before changing them
* make the operation idempotent
* provide an uninstall/remove integration command

If automatic configuration is too risky for the first version, generate the exact configuration and provide a copy/install workflow instead.

Choose the safest useful approach.

⸻

Codex Integration

Casebook should be easy to configure as an MCP server for Codex as well.

Do not build a separate Casebook implementation for Codex.

Use the same MCP server.

Provide:

* clear setup documentation
* example MCP configuration
* a way to verify that Codex can see Casebook
* a simple smoke test

If current Codex tooling supports a safe automated setup workflow, you may implement:

casebook setup codex

Otherwise provide instructions and a diagnostic command.

⸻

CLI

Provide a useful technical CLI.

Potential commands:

casebook
casebook ui
casebook mcp
casebook status
casebook doctor
casebook investigation list
casebook investigation show
casebook investigation active
casebook investigation activate
casebook context
casebook export
casebook import
casebook setup claude
casebook setup codex
casebook hook session-start
casebook hook checkpoint

Do not create commands just because they sound cool.

Implement commands that materially support the product.

⸻

Database

Use SQLite.

The database should be automatically created in the correct per-user application data directory.

Examples conceptually:

Windows:
%LOCALAPPDATA%\Casebook\casebook.db

macOS:
~/Library/Application Support/Casebook/casebook.db

Use proper OS APIs rather than hardcoding paths where possible.

Requirements:

* migrations
* foreign keys
* sensible indexes
* transaction safety
* migration tests
* no destructive automatic schema changes
* clear backup/export story

Avoid ORM complexity unless it genuinely improves the implementation.

Straightforward SQL is fine.

⸻

Concurrency

The GUI and MCP process may access the same SQLite database.

Design for that intentionally.

Use SQLite WAL mode where appropriate.

Handle database locking gracefully.

Do not invent a separate synchronization service.

Test simultaneous reads and realistic write contention.

⸻

Search

V1 needs useful investigation search.

Search across at least:

* investigation title
* problem statement
* claims
* hypotheses
* evidence summaries
* questions

Start with SQLite capabilities.

Do not introduce a vector database for V1.

If SQLite FTS5 materially improves this with modest complexity, use it.

⸻

Security and Privacy

V1 is local-first.

Requirements:

* no telemetry
* no cloud sync
* no automatic data upload
* no background network access unless explicitly required by a configured external integration
* no secrets written into logs
* sane file permissions
* sanitize imported data
* validate MCP inputs

Document the local data model clearly.

⸻

Testing Expectations

This cannot be a "looks like it works" project.

Create tests for the durable parts of the system.

At minimum:

Domain tests

* create investigation
* add evidence
* add claim
* link supporting evidence
* link contradicting evidence
* hypothesis lifecycle
* question lifecycle
* confidence updates
* conclusion updates
* verification state
* superseding records

Persistence tests

* migrations
* round-trip persistence
* relationships
* deletion/archive behavior
* transactions
* concurrent access where relevant

Context tests

Given a known investigation state, generated context should:

* contain the right active claims
* contain open questions
* contain current conclusion
* include evidence references
* exclude superseded noise
* include disproven hypotheses only where useful

Use snapshot/golden testing where appropriate.

MCP tests

Test every MCP tool.

At minimum prove:

1. Create investigation through MCP.
2. Add evidence.
3. Add claim.
4. Link evidence.
5. Update conclusion.
6. Retrieve investigation.
7. Generate context.

Cross-surface integration test

This is especially important.

Prove this workflow:

1. Create investigation through the desktop/core layer.
2. Read it through MCP.
3. Add evidence through MCP.
4. Read the new evidence through the desktop/core layer.
5. Generate context.
6. Confirm all interfaces see the same state.

That is one of the product's central promises.

⸻

Diagnostics

Create:

casebook doctor

It should check useful things such as:

* Casebook data directory
* database health
* migrations
* write access
* MCP startup
* config location
* Claude configuration status if detectable
* Codex configuration status if detectable

Do not expose sensitive data.

This will be extremely valuable once other people begin installing Casebook.

⸻

Packaging and Releases

Set the project up so GitHub Actions can eventually build release artifacts for:

* Windows x64
* Windows ARM64 if practical
* macOS Apple Silicon
* macOS Intel if practical

Do not require end users to install Go or Node.

Node may be required during build for the frontend, but it must not be an end-user dependency.

Produce clear build documentation.

Prepare the architecture for future:

* Windows code signing
* macOS Developer ID signing
* macOS notarization
* DMG creation

Do not block the initial MVP on certificates or credentials that are not available.

Structure CI so signing can be added through secrets later.

⸻

Open Source Quality

Assume this may become a public repository.

Create:

* strong README
* architecture overview
* contribution guidance
* development setup
* build instructions
* MCP setup guide
* Claude Code integration guide
* Codex integration guide
* local data/privacy explanation
* screenshots section placeholder if necessary
* license recommendation, but do not invent ownership/legal claims

The README should explain the product simply:

Casebook gives AI investigations durable memory. Evidence, claims, hypotheses, questions, and conclusions live locally and can be shared across AI clients without relying on chat history.

Avoid jargon-heavy descriptions.

⸻

GSD Project Setup

Before implementing, initialize this as a proper GSD project using the GSD tooling available in this environment.

If a GSD command/skill is installed, use the official workflow rather than inventing a parallel project-management structure.

Capture at least:

* product vision
* requirements
* architectural constraints
* phased roadmap
* current state
* implementation decisions
* deferred ideas
* acceptance criteria

The roadmap should favor vertical slices rather than building every backend component before seeing the UI.

A reasonable shape would be:

Phase 1:
Domain model + SQLite + tests

Phase 2:
Usable desktop investigation UI

Phase 3:
MCP vertical slice

Phase 4:
Context generation + active investigation

Phase 5:
Claude Code integration + hooks

Phase 6:
Codex integration

Phase 7:
Packaging, diagnostics, documentation, release automation

Adjust this if your architecture review identifies a better sequence.

Then EXECUTE.

Do not merely produce the roadmap.

⸻

Scope Discipline

For V1, explicitly defer unless necessary:

* multi-user collaboration
* cloud sync
* team workspaces
* hosted accounts
* role-based access control
* organization-wide knowledge
* vector databases
* embeddings
* complex semantic search
* automatic knowledge promotion
* automatic AI-provider routing
* agent orchestration
* workflows
* DAGs
* schedulers
* transcript recording
* billing
* mobile apps
* enterprise management

Leave extension points where sensible, but do not implement speculative infrastructure.

⸻

UX Standard

Casebook should answer four questions at a glance:

1. What are we trying to figure out?
2. What do we currently believe?
3. What evidence supports or contradicts it?
4. What is still unresolved?

The UI should visually differentiate:

* evidence
* established claims
* hypotheses
* unresolved questions
* disputed information
* verified information

Confidence should be visible but not visually overwhelming.

Make evidence drill-down easy.

Do not create a dense developer dashboard.

This should be understandable to a business leader.

⸻

Important Reasoning Model

The fundamental Casebook mental model is:

QUESTION
    ↓
EVIDENCE
    ↓
CLAIMS <---- CONTRADICTING EVIDENCE
    ↓
CONCLUSION
Parallel:
HYPOTHESES
    ↓
tested against evidence
    ↓
supported / unresolved / disproven
OPEN QUESTIONS
    ↓
future investigation

Do not turn that into a general graph platform.

The relationships only need to support the investigation experience.

⸻

Definition of a Successful Initial Build

I want the first substantial implementation to demonstrate this exact scenario:

1. Launch Casebook as a desktop application.
2. Create a new investigation.
3. Add a problem statement.
4. Add evidence manually.
5. Create a claim.
6. Link the evidence to the claim.
7. Give the claim a confidence level.
8. Add a hypothesis.
9. Add an open question.
10. Set a current conclusion.
11. Mark the investigation active.
12. Close and reopen Casebook.
13. Everything persists.
14. Launch the Casebook MCP server.
15. Retrieve the same active investigation through MCP.
16. Add new evidence through MCP.
17. Add or update a claim through MCP.
18. Reopen/refresh the Casebook UI.
19. The MCP-created information is visible.
20. Generate a compact context pack.
21. Configure Claude Code to access the same MCP server.
22. Demonstrate the expected Claude Code setup.
23. Provide equivalent Codex MCP setup instructions.
24. Run the automated test suite successfully.
25. Produce buildable Windows and macOS project configurations.

If possible within the current environment, actually run the application and perform the end-to-end smoke test.

⸻

Final Architecture Review

Before considering this pass complete, run an adversarial review of what you built.

Review it from three perspectives:

Product reviewer

Could a technically capable business leader understand and use this without being taught the implementation?

Senior software architect

Did we introduce unnecessary abstractions, duplication, hidden coupling, portability problems, or packaging traps?

AI workflow reviewer

Does this actually provide durable evidence-based context between Claude, Codex, and future MCP clients, or did we accidentally create another glorified notes application?

Fix material issues discovered in that review.

Do not merely document them if they can reasonably be corrected now.

⸻

Final Deliverable

When you finish this run, give me a concise summary containing:

1. What you built.
2. The architecture you chose.
3. Important decisions and why.
4. Current GSD phase/state.
5. What works end-to-end.
6. Test results.
7. How to run the desktop application.
8. How to connect Claude Code.
9. How to connect Codex.
10. What remains unfinished.
11. The highest-value next step.

But again: do not stop at planning. Build the product.