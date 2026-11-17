#!/usr/bin/env python3
"""Grade an agent-eval run directory against rubric.md.

Usage: grade.py RUN_DIR [RUN_DIR...]   (prints one scorecard per run)
       grade.py --table RUN_DIR...      (one summary row per run)

Everything is read from what landed in the run's Casebook database (the
per-session exports) plus the tool calls in the session logs; nothing is
judged by a model.
"""

import glob
import json
import os
import re
import sys

READ_TOOLS = {"casebook_get_context", "casebook_get_active_investigation", "casebook_list_investigations",
              "casebook_search_investigations", "casebook_get_investigation"}
EXPLORE_TOOLS = {"Bash", "Read", "Grep", "Glob"}


def read_jsonl(path):
    out = []
    if not os.path.exists(path):
        return out
    with open(path) as f:
        for line in f:
            line = line.strip()
            if not line.startswith("{"):
                continue
            try:
                out.append(json.loads(line))
            except ValueError:
                pass
    return out


# An offer to open a case, in the agent's own words.
OFFER_RE = re.compile(r"\b(open|start|creat|record|track|sav|log)\w*\b[^.?!\n]{0,60}\b(case|investigation|casebook)\b|"
                      r"\bcasebook\b[^.?!\n]{0,80}\b(open|start|creat)\w*", re.I)


def agent_texts(log):
    """Every text the agent wrote, Claude stream-json or Codex --json."""
    out = []
    for ev in read_jsonl(log):
        if ev.get("type") == "assistant":
            for c in (ev.get("message") or {}).get("content") or []:
                if c.get("type") == "text":
                    out.append(c.get("text") or "")
        item = ev.get("item") or {}
        if ev.get("type") == "item.completed" and item.get("type") == "agent_message":
            out.append(item.get("text") or "")
    return out


def offered(log):
    """Whether the agent offered to open a case anywhere in the log."""
    return any(OFFER_RE.search(t) for t in agent_texts(log))


def tool_calls(log):
    """(name, detail) per tool call, Claude stream-json or Codex --json."""
    calls = []
    for ev in read_jsonl(log):
        if ev.get("type") == "assistant":
            for c in (ev.get("message") or {}).get("content") or []:
                if c.get("type") == "tool_use":
                    name = c.get("name", "")
                    inp = c.get("input") or {}
                    if name.startswith("mcp__casebook__"):
                        name = name[len("mcp__casebook__"):]
                    detail = inp.get("command") or inp.get("file_path") or inp.get("pattern") or inp.get("skill") or ""
                    calls.append((name, str(detail)))
        if ev.get("type") == "item.completed":
            item = ev.get("item") or {}
            if item.get("type") == "command_execution":
                calls.append(("Bash", item.get("command", "")))
            elif item.get("type") == "mcp_tool_call":
                calls.append((item.get("tool", ""), item.get("server", "")))
    return calls


def usage(log):
    cost, turns = 0.0, 0
    for ev in read_jsonl(log):
        if ev.get("type") == "result":
            cost += ev.get("total_cost_usd") or 0.0
            turns += ev.get("num_turns") or 0
    return round(cost, 3), turns


def load_exports(sdir):
    invs = {}
    for p in sorted(glob.glob(os.path.join(sdir, "INV-*.json"))):
        with open(p) as f:
            d = json.load(f)
        invs[d["investigation"]["ref"]] = d
    return invs


def within(ts, s):
    return s["start"] <= (ts or "") <= s["end"]


def text_of(inv, kinds):
    parts = []
    if "conclusion" in kinds:
        parts.append(inv["investigation"].get("conclusion") or "")
    for c in inv.get("claims", []):
        if "claims" in kinds and c["status"] in ("active", "disputed"):
            parts.append(c["statement"])
    for h in inv.get("hypotheses", []):
        if "hypotheses" in kinds and h["status"] in ("supported", "open"):
            parts.append(h["statement"])
    return "\n".join(parts)


def source_ok(ev, work):
    """Evidence has a source a reader can follow: an existing repo path (line
    suffix allowed), a command, or a labelled tool or query output."""
    ref = (ev.get("source_ref") or "").strip()
    label = (ev.get("source_label") or "").strip()
    if not ref and not label:
        return False
    path = re.split(r"[,:#\s]", ref, maxsplit=1)[0] if ref else ""
    if path and os.path.exists(os.path.join(work, path)):
        return True
    return ev.get("source_type") not in ("code", "file") and bool(ref or label)


def grade_session(s, invs, prev_invs, work, scenario, meta):
    sdir_log = s["_log"]
    calls = tool_calls(sdir_log)
    names = [n for n, _ in calls]
    cb_calls = [n for n in names if n.startswith("casebook_")]
    acts = []
    for ref, inv in invs.items():
        for a in inv.get("activity", []):
            if within(a.get("created_at"), s):
                acts.append((ref, a))
    touched = {ref for ref, _ in acts}
    created = [ref for ref, a in acts if a["action"] == "investigation.create"]
    cost, turns = usage(sdir_log)
    r = {"n": s["n"], "seconds": s.get("seconds"), "cost_usd": cost, "model_turns": turns,
         "casebook_calls": len(cb_calls), "offered": offered(sdir_log), "confirmed": s.get("confirmed", False),
         "touched": sorted(touched), "created": created, "checks": {}}
    chk = r["checks"]

    def items(kind, pred=lambda x: True):
        out = []
        for ref in touched:
            for x in invs[ref].get(kind, []):
                if within(x.get("created_at"), s) and pred(x):
                    out.append(x)
        return out

    evidence = items("evidence")
    links = items("claim_links") + items("hypothesis_links")
    hyps = items("hypotheses")
    claims = items("claims")
    upd = [a for _, a in acts]
    if scenario["kind"] == "control":
        # A quick fix needs no case: the agent may offer one, but opening
        # one unasked is noise. The fix itself is checked from the repo.
        chk["restraint"] = not (created and not s.get("confirmed"))
        fc = scenario.get("fixed_check") or {}
        if fc:
            with open(os.path.join(work, fc["file"])) as f:
                chk["fixed"] = fc["absent"] not in f.read()
        r["score"] = sum(1 for v in chk.values() if v)
        r["max"] = len(chk)
        return r
    chk["used"] = bool(cb_calls)
    if s["n"] == 1 or not prev_invs:
        # Looked for an existing case before creating one.
        first_create = names.index("casebook_create_investigation") if "casebook_create_investigation" in names else None
        chk["oriented"] = any(n in READ_TOOLS for n in names[: first_create if first_create is not None else len(names)]) or bool(s.get("active_at_start") and meta.get("hooks"))
        chk["case"] = bool(touched)
        # A case opened without the person agreeing (the prompt never asks
        # for one) loses this point; so does never offering on an
        # investigation scenario.
        chk["consent"] = bool(created) and s.get("confirmed", False)
    else:
        first_explore = next((i for i, n in enumerate(names) if n in EXPLORE_TOOLS), len(names))
        hook_pack = bool(s.get("active_at_start") and meta.get("hooks"))
        chk["resumed"] = hook_pack or any(n in READ_TOOLS for n in names[:first_explore])
        chk["same_case"] = not created and bool(touched & set(prev_invs))
        ruled = re.compile(scenario["ruled_out"], re.I) if scenario.get("ruled_out") else None
        prev_disproven = [h["statement"] for inv in prev_invs.values() for h in inv.get("hypotheses", [])
                          if h["status"] == "disproven"]
        retested = bool(ruled and any(ruled.search(h["statement"]) for h in hyps)) and bool(prev_disproven)
        # Re-running the checks that disproved a hypothesis: tool calls
        # whose text matches the scenario's redo_patterns (for
        # flaky-checkout, hour-of-day analysis for the midnight theory).
        redo = re.compile(scenario["redo_patterns"], re.I) if scenario.get("redo_patterns") and prev_disproven else None
        r["redo_calls"] = [d[:120] for n, d in calls if redo and n in EXPLORE_TOOLS and redo.search(d)]
        chk["no_redo"] = not retested and not r["redo_calls"]
        chk["advanced"] = any(a["action"] in ("hypothesis.update", "conclusion.update", "claim.update", "question.resolve") for a in upd)
    if scenario["kind"] == "investigation":
        chk["evidence"] = len(evidence) >= 2 and sum(source_ok(e, work) for e in evidence) >= max(2, int(0.8 * len(evidence)))
        chk["linked"] = bool(links)
        if s["n"] == 1 or not prev_invs:
            chk["hypotheses"] = bool(hyps)
            chk["ruled_out"] = any(h["status"] == "disproven" for h in hyps) or any(l["relation"] == "contradicts" for l in links) \
                or any(a["action"] == "hypothesis.update" and "disproven" in (a.get("summary") or "").lower() for a in upd)
            chk["questions"] = bool(items("questions")) or any(a["action"].startswith("question.") for a in upd)
        chk["checkpoint"] = any(a["action"] == "checkpoint" for a in upd)
        rc = re.compile(scenario["root_cause"], re.I)
        recorded = "\n".join(text_of(invs[ref], {"conclusion", "claims", "hypotheses"}) for ref in touched)
        r["root_cause_recorded"] = bool(rc.search(recorded))
    r["score"] = sum(1 for v in chk.values() if v)
    r["max"] = len(chk)
    return r


def grade_run(out, scenario=None):
    with open(os.path.join(out, "meta.json")) as f:
        meta = json.load(f)
    if scenario is None:
        here = os.path.dirname(os.path.abspath(__file__))
        with open(os.path.join(here, "scenarios", meta["scenario"], "scenario.json")) as f:
            scenario = json.load(f)
    work = os.path.join(out, "work")
    card = {"run": out, "scenario": meta["scenario"], "client": meta["client"], "model": meta["model"],
            "label": meta.get("label", ""), "hooks": meta.get("hooks"), "snippet": meta.get("snippet"), "hint": meta.get("hint"),
            "extras": meta.get("extras", ""), "sessions": []}
    prev = {}
    for s in meta["sessions"]:
        sdir = os.path.join(out, "session-%d" % s["n"])
        s = dict(s, _log=os.path.join(sdir, "log.jsonl"))
        invs = load_exports(sdir)
        card["sessions"].append(grade_session(s, invs, prev, work, scenario, meta))
        prev = invs
    card["score"] = sum(x["score"] for x in card["sessions"])
    card["max"] = sum(x["max"] for x in card["sessions"])
    card["cost_usd"] = round(sum(x["cost_usd"] for x in card["sessions"]), 3)
    card["seconds"] = sum(x.get("seconds") or 0 for x in card["sessions"])
    return card


def render(card):
    lines = ["%s / %s / %s%s: %d of %d" % (card["scenario"], card["client"], card["model"],
                                         (" / " + card["label"]) if card["label"] else "", card["score"], card["max"])]
    for s in card["sessions"]:
        marks = " ".join(("+" if v else "-") + k for k, v in s["checks"].items())
        extra = ""
        if "root_cause_recorded" in s:
            extra = " root-cause-recorded=%s" % ("yes" if s["root_cause_recorded"] else "no")
        lines.append("  session %d: %d/%d  %s  [casebook calls %d, offered %s, confirmed %s, $%.2f, %ss]%s" % (
            s["n"], s["score"], s["max"], marks, s["casebook_calls"], s["offered"], s["confirmed"],
            s["cost_usd"], s["seconds"], extra))
    return "\n".join(lines)


def main(argv):
    table = "--table" in argv
    runs = [a for a in argv if a != "--table"]
    if not runs:
        print(__doc__)
        return 2
    for out in runs:
        card = grade_run(out)
        if table:
            print("| %s | %s | %s | %s | %d/%d | $%.2f |" % (card["label"], card["scenario"], card["client"],
                                                           card["model"], card["score"], card["max"], card["cost_usd"]))
        else:
            print(render(card))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
