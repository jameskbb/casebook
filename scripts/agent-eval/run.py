#!/usr/bin/env python3
"""Run one Casebook agent-eval scenario in an isolated sandbox.

Every run gets its own directory (--out) holding a throwaway copy of the
scenario repo, its own Casebook database, its own Claude Code or Codex
configuration written by `casebook setup`, the session logs and the graded
scorecard. The user's Casebook data directory, ~/.claude, ~/.claude.json and
~/.codex/config.toml are never read or written (the one exception: with
--client codex the run links ~/.codex/auth.json into its private CODEX_HOME so
Codex can sign in; the link is removed when the run ends).

The prompts are ordinary user requests that never mention Casebook. Each
session is a fresh agent process with no chat history; only what landed in
Casebook carries over. When a session ends with no investigation in the
database but its last message offers to open a case, the runner answers once
with --confirm (a person saying yes), in the same session.

See README.md next to this file.
"""

import argparse
import datetime
import json
import os
import re
import shutil
import subprocess
import sys
import time

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.dirname(os.path.dirname(HERE))
sys.path.insert(0, HERE)
sys.dont_write_bytecode = True
import grade  # noqa: E402

DEFAULT_CONFIRM = "Yes, please open a case for it."
OFFER_RE = grade.OFFER_RE
ALLOWED_TOOLS = "Bash,Read,Edit,Write,Grep,Glob,Skill,TodoWrite,mcp__casebook"


def now_iso():
    t = datetime.datetime.now(datetime.timezone.utc)
    return t.strftime("%Y-%m-%dT%H:%M:%S.") + "%03dZ" % (t.microsecond // 1000)


def die(msg):
    print("agent-eval: " + msg, file=sys.stderr)
    sys.exit(2)


def real(p):
    return os.path.realpath(os.path.abspath(p))


def inside(path, root):
    path, root = real(path), real(root)
    return path == root or path.startswith(root + os.sep)


def guard_isolation(out):
    """Refuse an --out that overlaps any real configuration or data."""
    home = os.path.expanduser("~")
    forbidden = [
        os.path.join(home, ".claude"),
        os.path.join(home, ".codex"),
        os.path.join(home, ".local", "share", "casebook"),
        os.path.join(home, "Library", "Application Support", "Casebook"),
        ROOT,
    ]
    for env in ("CASEBOOK_HOME", "XDG_DATA_HOME", "CLAUDE_CONFIG_DIR", "CODEX_HOME"):
        if os.environ.get(env):
            forbidden.append(os.environ[env])
    for f in forbidden:
        if inside(out, f) or inside(f, out):
            die("--out %s overlaps %s; pick a scratch directory" % (out, f))


def run(cmd, env, cwd=None, stdout=None, timeout=None, check=True):
    r = subprocess.run(cmd, env=env, cwd=cwd, stdout=stdout or subprocess.PIPE,
                       stderr=subprocess.PIPE, text=True, timeout=timeout)
    if check and r.returncode != 0:
        die("%s failed (%d): %s" % (" ".join(cmd[:3]), r.returncode, r.stderr.strip()[-2000:]))
    return r


def go_binary():
    """The go command: on PATH, else $GOROOT/bin/go, else ~/.local/go/bin/go."""
    for cand in (shutil.which("go"), os.path.join(os.environ.get("GOROOT", ""), "bin", "go"),
                 os.path.expanduser("~/.local/go/bin/go"), "/usr/local/go/bin/go"):
        if cand and os.path.isfile(cand):
            return cand
    die("go not found; the scenarios are Go repositories")


def casebook_env(base, db):
    env = dict(base)
    # The agent needs go on PATH to build and test the scenario repo.
    gobin = os.path.dirname(go_binary())
    env["PATH"] = gobin + os.pathsep + env.get("PATH", "")
    env["CASEBOOK_DB"] = db
    env["CASEBOOK_HOME"] = os.path.dirname(db)
    return env


def snapshot(cb, db, env, dest):
    """Save the investigation list, every export and the active context."""
    os.makedirs(dest, exist_ok=True)
    lst = run([cb, "--db", db, "--json", "investigation", "list", "--archived"], env).stdout
    with open(os.path.join(dest, "list.json"), "w") as f:
        f.write(lst)
    for inv in json.loads(lst or "[]"):
        ref = inv["ref"]
        run([cb, "--db", db, "export", ref, "-o", os.path.join(dest, ref + ".json"), "--force"], env)
    st = run([cb, "--db", db, "--json", "status"], env).stdout
    with open(os.path.join(dest, "status.json"), "w") as f:
        f.write(st)


def setup_claude(cb, db, out, work, env, args):
    cfg = os.path.join(out, "claude-config")
    os.makedirs(cfg, exist_ok=True)
    e = dict(env, CLAUDE_CONFIG_DIR=cfg)
    cmd = [cb, "--db", db, "setup", "claude"]
    if not args.no_hooks:
        cmd.append("--hooks")
        if args.no_hint:
            # Variant: no one-line hint when no case is active (the hint is
            # the default; casebook setup claude --hooks --quiet-when-inactive).
            cmd.append("--quiet-when-inactive")
    if args.extras:
        cmd += args.extras.split()
    run(cmd, e)
    with open(os.path.join(cfg, ".claude.json")) as f:
        server = json.load(f)["mcpServers"]["casebook"]
    mcp = os.path.join(out, "mcp.json")
    with open(mcp, "w") as f:
        json.dump({"mcpServers": {"casebook": server}}, f, indent=2)
    settings = os.path.join(cfg, "settings.json")
    if not os.path.exists(settings):
        with open(settings, "w") as f:
            f.write("{}\n")
    # Skills and commands setup installed for the user go to the sandbox
    # project instead, since the run does not load user settings.
    for sub in ("skills", "commands"):
        src = os.path.join(cfg, sub)
        if os.path.isdir(src):
            shutil.copytree(src, os.path.join(work, ".claude", sub), dirs_exist_ok=True)
    return mcp, settings


def setup_codex(cb, db, out, env, args):
    home = os.path.join(out, "codex-home")
    os.makedirs(home, exist_ok=True)
    e = dict(env, CODEX_HOME=home)
    cmd = [cb, "--db", db, "setup", "codex"]
    if args.extras:
        cmd += args.extras.split()
    run(cmd, e)
    auth = os.path.expanduser("~/.codex/auth.json")
    if os.path.exists(auth):
        os.symlink(auth, os.path.join(home, "auth.json"))
    return home


def add_snippet(cb, db, env, work, client):
    """Opt-in: append what `casebook setup instructions` prints to the
    project instructions file, as a person following the docs would."""
    r = run([cb, "--db", db, "setup", "instructions"], env, check=False)
    if r.returncode != 0:
        die("--snippet needs a casebook with `setup instructions`")
    name = "CLAUDE.md" if client == "claude" else "AGENTS.md"
    with open(os.path.join(work, name), "a") as f:
        f.write(r.stdout)


def claude_turn(prompt, work, env, args, mcp, settings, log, resume=None):
    cmd = ["claude", "-p", prompt, "--output-format", "stream-json", "--verbose",
           "--mcp-config", mcp, "--strict-mcp-config", "--settings", settings,
           "--setting-sources", "project,local",
           "--permission-mode", "acceptEdits", "--allowedTools", ALLOWED_TOOLS,
           "--max-turns", str(args.max_turns), "--max-budget-usd", str(args.budget)]
    if args.model:
        cmd += ["--model", args.model]
    if resume:
        cmd += ["--resume", resume]
    with open(log, "a") as f, open(log + ".stderr", "a") as ef:
        subprocess.run(cmd, cwd=work, env=env, stdout=f, stderr=ef,
                       stdin=subprocess.DEVNULL, timeout=args.timeout)
    sid, text = None, ""
    for ev in grade.read_jsonl(log):
        if ev.get("type") == "system" and ev.get("subtype") == "init":
            sid = ev.get("session_id")
        if ev.get("type") == "result":
            text = ev.get("result") or ""
    return sid, text


def codex_turn(prompt, work, env, args, log, resume=None):
    # codex exec resume takes no --sandbox or -C; it reuses the session's
    # directory, and the sandbox is set through -c.
    cmd = ["codex", "exec"] + (["resume"] if resume else [])
    # Nobody is there to answer an approval prompt, so Casebook tool calls
    # are approved up front, as a person clicking approve would.
    cmd += ["--json", "--skip-git-repo-check", "-c", 'sandbox_mode="workspace-write"',
            "-c", 'mcp_servers.casebook.default_tools_approval_mode="approve"']
    if args.model:
        cmd += ["-m", args.model]
    cmd += [resume] if resume else ["-C", work]
    cmd.append(prompt)
    with open(log, "a") as f, open(log + ".stderr", "a") as ef:
        subprocess.run(cmd, cwd=work, env=env, stdout=f, stderr=ef,
                       stdin=subprocess.DEVNULL, timeout=args.timeout)
    sid, text = resume, ""
    for ev in grade.read_jsonl(log):
        if ev.get("type") == "thread.started":
            sid = ev.get("thread_id")
        item = ev.get("item") or {}
        if ev.get("type") == "item.completed" and item.get("type") == "agent_message":
            text = item.get("text") or ""
    return sid, text


def main():
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("scenario", help="scenario name under scenarios/")
    p.add_argument("--client", choices=["claude", "codex"], default="claude")
    p.add_argument("--model", default="", help="model for every session (client default when empty)")
    p.add_argument("--casebook", default="", help="casebook binary (default: build this checkout)")
    p.add_argument("--out", default="", help="run directory (must not exist)")
    p.add_argument("--label", default="", help="free-form label stored with the score")
    p.add_argument("--sessions", type=int, default=0, help="run only the first N sessions")
    p.add_argument("--budget", type=float, default=1.5, help="USD cap per claude turn")
    p.add_argument("--max-turns", type=int, default=40)
    p.add_argument("--timeout", type=int, default=1200, help="seconds per turn")
    p.add_argument("--no-hooks", action="store_true", help="claude: register MCP only, no hooks")
    p.add_argument("--no-hint", action="store_true", help="claude: install the hooks with --quiet-when-inactive (no hint when no case is active)")
    p.add_argument("--extras", default="", help="extra flags for casebook setup claude/codex")
    p.add_argument("--snippet", action="store_true", help="append `casebook setup instructions` to CLAUDE.md or AGENTS.md")
    p.add_argument("--confirm", default=DEFAULT_CONFIRM, help="reply sent when the agent offers to open a case")
    args = p.parse_args()

    sc_dir = os.path.join(HERE, "scenarios", args.scenario)
    with open(os.path.join(sc_dir, "scenario.json")) as f:
        scenario = json.load(f)
    stamp = datetime.datetime.now().strftime("%Y%m%dT%H%M%S")
    base_tmp = os.environ.get("TMPDIR") or "/tmp"
    out = real(args.out or os.path.join(base_tmp, "casebook-agent-eval", "%s-%s-%s" % (args.scenario, args.client, stamp)))
    guard_isolation(out)
    if os.path.exists(out):
        die("--out %s already exists" % out)
    os.makedirs(out)

    cb = args.casebook
    if not cb:
        cb = os.path.join(out, "bin", "casebook")
        r = subprocess.run([go_binary(), "build", "-o", cb, "./cmd/casebook"], cwd=ROOT,
                           stderr=subprocess.PIPE, text=True)
        if r.returncode != 0:
            die("go build failed: " + r.stderr)
    cb = real(cb)

    db = os.path.join(out, "casebook-home", "casebook.db")
    os.makedirs(os.path.dirname(db), mode=0o700)
    env = casebook_env(os.environ, db)
    st = json.loads(run([cb, "--db", db, "--json", "status"], env).stdout)
    if not inside(st["db_path"], out):
        die("casebook resolved its database to %s, outside the run directory" % st["db_path"])

    work = os.path.join(out, "work")
    shutil.copytree(os.path.join(sc_dir, "repo"), work)
    git = ["git", "-c", "user.name=agent-eval", "-c", "user.email=agent-eval@example.invalid"]
    run(git + ["init", "-q"], env, cwd=work)
    run(git + ["add", "-A"], env, cwd=work)
    run(git + ["commit", "-q", "-m", "initial"], env, cwd=work)

    if args.client == "claude":
        mcp, settings = setup_claude(cb, db, out, work, env, args)
    else:
        env["CODEX_HOME"] = setup_codex(cb, db, out, env, args)
    if args.snippet:
        add_snippet(cb, db, env, work, args.client)

    meta = {"scenario": args.scenario, "client": args.client, "model": args.model or "default",
            "label": args.label, "hooks": args.client == "claude" and not args.no_hooks,
            "snippet": args.snippet, "hint": args.client == "claude" and not args.no_hooks and not args.no_hint,
            "extras": args.extras, "sessions": []}
    prompts = scenario["sessions"][: args.sessions or None]
    try:
        for i, prompt in enumerate(prompts, 1):
            sdir = os.path.join(out, "session-%d" % i)
            os.makedirs(sdir)
            log = os.path.join(sdir, "log.jsonl")
            before = json.loads(run([cb, "--db", db, "--json", "status"], env).stdout)
            s = {"n": i, "prompt": prompt, "start": now_iso(), "active_at_start": before["active_investigation_ref"],
                 "confirmed": False, "turns": 1}
            t0 = time.time()
            if args.client == "claude":
                sid, text = claude_turn(prompt, work, env, args, mcp, settings, log)
            else:
                sid, text = codex_turn(prompt, work, env, args, log)
            invs = json.loads(run([cb, "--db", db, "--json", "investigation", "list", "--archived"], env).stdout or "[]")
            # An offer anywhere in the turn counts: Codex in particular asks
            # mid-turn and keeps working.
            s["offered"] = grade.offered(log)
            if not invs and s["offered"] and sid:
                s["confirmed"], s["turns"] = True, 2
                s["confirm_at"] = now_iso()
                if args.client == "claude":
                    claude_turn(args.confirm, work, env, args, mcp, settings, log, resume=sid)
                else:
                    codex_turn(args.confirm, work, env, args, log, resume=sid)
            s["end"] = now_iso()
            s["seconds"] = round(time.time() - t0)
            s["session_id"] = sid
            snapshot(cb, db, env, sdir)
            meta["sessions"].append(s)
            with open(os.path.join(out, "meta.json"), "w") as f:
                json.dump(meta, f, indent=2)
    finally:
        link = os.path.join(out, "codex-home", "auth.json")
        if os.path.islink(link):
            os.unlink(link)

    card = grade.grade_run(out, scenario)
    with open(os.path.join(out, "score.json"), "w") as f:
        json.dump(card, f, indent=2)
    print(grade.render(card))
    print("run directory: " + out)


if __name__ == "__main__":
    main()
