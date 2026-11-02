// Connect AI clients (INT-10, D-09): where the casebook command-line tool
// is, whether Claude Code and Codex already run Casebook, and the one
// command (or the configuration to merge by hand) that connects each. The
// panel only reads: client settings are written by `casebook setup claude`
// and `casebook setup codex`, which back the file up first and change only
// Casebook's entry, so the panel copies that command for the user to run.
import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react'
import { CircleCheck, CircleDashed, Copy, RefreshCw, TriangleAlert } from 'lucide-react'
import { api } from '../../api/api'
import { CLIPBOARD_COPY, errorCopy, toApiError } from '../../api/errors'
import type { ClientConnection, ClientConnections } from '../../api/types'
import { BreakablePath } from '../ui/BreakablePath'
import { Button } from '../ui/Button'
import { Dialog } from '../ui/Dialog'
import { IconButton } from '../ui/IconButton'
import { SegmentedControl } from '../ui/SegmentedControl'
import { useToast } from '../ui/Toaster'
import './ConnectClientsDialog.css'

export const CLI_MISSING_COPY =
  'Casebook could not find its command-line tool. Install a Casebook release to use the setup commands below.'
export const CLI_ON_PATH_COPY =
  'This development build uses the command-line tool found on PATH.'

export const CLAUDE_AFTER_COPY =
  "Then restart Claude Code (or run /mcp). Use /casebook to work with your cases."
export const CODEX_AFTER_COPY = 'Then start a new Codex session and ask it to use Casebook.'
export const OTHER_MCP_COPY =
  'casebook setup mcp prints the server command and a configuration block to paste into any other MCP client.'

export const FIRST_FINDING_PROMPT = 'Help me understand why our margin changed. First ask me which period, region and source data to use. Save one finding in Casebook with its supporting evidence and source details, and note what we still need to check.'

type ClientChoice = 'claude' | 'codex' | 'other'

type Status = { tone: 'ok' | 'warn' | 'off'; label: string }

/** The one-line status of a client. */
export function clientStatus(c: ClientConnection, cliKnown: boolean): Status {
  if (c.config_problem) return { tone: 'warn', label: "Settings file can't be read" }
  if (!c.configured) return { tone: 'off', label: 'Not connected' }
  if (!c.command_exists) return { tone: 'warn', label: 'Configured command-line tool is missing' }
  if (cliKnown && !c.matches_cli) return { tone: 'warn', label: 'Uses a different command-line tool' }
  return { tone: 'ok', label: 'Connected' }
}

function StatusIcon({ tone }: { tone: Status['tone'] }) {
  const props = { size: 16, strokeWidth: 1.75, 'aria-hidden': true as const }
  if (tone === 'ok') return <CircleCheck className="connect-status-icon is-ok" {...props} />
  if (tone === 'warn') return <TriangleAlert className="connect-status-icon is-warn" {...props} />
  return <CircleDashed className="connect-status-icon is-off" {...props} />
}

function CodeLine({ text, copyLabel, onCopy }: { text: string; copyLabel: string; onCopy: (t: string) => void }) {
  return (
    <div className="connect-code-line">
      <code className="connect-code">
        <BreakablePath text={text} />
      </code>
      <IconButton icon={Copy} label={copyLabel} onClick={() => onCopy(text)} />
    </div>
  )
}

/** "Run in PowerShell" on Windows, "Run in a terminal" elsewhere (AR-12). */
export function runLabel(shell: string | undefined): string {
  return shell && shell !== 'terminal' ? `Run in ${shell}` : 'Run in a terminal'
}

interface ClientSectionProps {
  name: string
  shell?: string
  client: ClientConnection
  cliKnown: boolean
  after: string
  onCopy: (text: string) => void
}

function ClientSection({ name, shell, client, cliKnown, after, onCopy }: ClientSectionProps) {
  const status = clientStatus(client, cliKnown)
  return (
    <section className="connect-section" aria-label={name}>
      <div className="connect-section-head">
        <h3 className="connect-section-title">Connect {name}</h3>
        <span className={`connect-status is-${status.tone}`}>
          <StatusIcon tone={status.tone} />
          {status.label}
        </span>
      </div>
      <p className="connect-label">{runLabel(shell)}</p>
      <CodeLine text={client.setup_command} copyLabel={`Copy the ${name} setup command`} onCopy={onCopy} />
      {status.tone === 'warn' && !client.config_problem && <p className="connect-muted">Run this setup command to update its location.</p>}
      <p className="connect-muted">{after}</p>
      <details className="connect-disclosure">
        <summary>Other shells and manual configuration</summary>
        <div className="connect-details-body">
          {client.setup_command_cmd && <>
            <p className="connect-label">Or in Command Prompt</p>
            <CodeLine text={client.setup_command_cmd} copyLabel={`Copy the ${name} setup command for Command Prompt`} onCopy={onCopy} />
          </>}
          {client.setup_command_wsl && <>
            <p className="connect-label">Or in WSL (bash), for {name} installed inside WSL</p>
            <CodeLine text={client.setup_command_wsl} copyLabel={`Copy the ${name} setup command for WSL`} onCopy={onCopy} />
            {name === 'Claude Code' && <p className="connect-muted">The WSL command adds the server only. Add session hooks by hand as the Claude Code guide shows.</p>}
          </>}
          <p className="connect-label">Or merge the configuration by hand</p>
          <pre className="connect-code connect-pre">{client.snippet}</pre>
          <Button variant="secondary" icon={Copy} onClick={() => onCopy(client.snippet)}>Copy configuration</Button>
        </div>
      </details>
      <details className="connect-disclosure">
        <summary>{name} settings and diagnostics</summary>
        <div className="connect-details-body">
          <p className="connect-muted connect-wrap">Settings: <BreakablePath text={client.config_path} /></p>
          <p className="connect-muted">This checks local settings, not a running AI session. Choose Check again after running setup.</p>
          {client.hooks.length > 0 && <p className="connect-muted">Session hooks: {client.hooks.join(', ')}</p>}
          {name === 'Claude Code' && <p className="connect-muted">--hooks loads your AI's current case when a session starts and reminds Claude to save findings. --skill adds the /casebook skill.</p>}
          {name === 'Codex' && <p className="connect-muted">--skill adds the /casebook skill.</p>}
        </div>
      </details>
    </section>
  )
}

export interface ConnectClientsDialogProps {
  onClose: () => void
}

export function ConnectClientsDialog({ onClose }: ConnectClientsDialogProps) {
  const toast = useToast()
  const doneRef = useRef<HTMLButtonElement>(null)
  const [client, setClient] = useState<ClientChoice>('claude')
  const [data, setData] = useState<ClientConnections | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      setData(await api.clientConnections())
      setError(null)
    } catch (e) {
      setError(errorCopy(toApiError(e)).message)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  // Keep the client choice at the top when local status first arrives.
  const introRef = useRef<HTMLParagraphElement>(null)
  const loaded = data !== null
  useLayoutEffect(() => {
    if (!loaded) return
    const body = introRef.current?.closest('.dialog-body')
    if (body) body.scrollTop = 0
  }, [loaded])

  const copy = async (text: string) => {
    try {
      await api.copyText(text)
      toast.success('Copied to the clipboard.')
    } catch {
      toast.error(CLIPBOARD_COPY)
    }
  }

  const cliKnown = Boolean(data?.cli_path)
  return (
    <Dialog
      title="Connect AI clients"
      onClose={onClose}
      initialFocus={doneRef}
      className="connect-dialog"
      footer={
        <>
          <Button variant="secondary" icon={RefreshCw} busy={loading} busyLabel="Checking..." onClick={() => void load()}>
            Check again
          </Button>
          <Button variant="primary" ref={doneRef} onClick={onClose}>
            Done
          </Button>
        </>
      }
    >
      <p className="dialog-text" ref={introRef}>
        Choose the AI client you use. Copy its command and run it yourself: setup backs up the client's settings first and changes only Casebook's entry.
      </p>
      <SegmentedControl<ClientChoice> label="Your AI client" value={client} onChange={setClient} options={[
        { value: 'claude', label: 'Claude Code' },
        { value: 'codex', label: 'Codex' },
        { value: 'other', label: 'Other MCP client' },
      ]} />
      {error && <p className="connect-error" role="alert">{error}</p>}
      {loading && !data && <p className="connect-muted" role="status">Checking local client settings...</p>}
      {data && <>
        {!cliKnown && <p className="connect-note">
          <TriangleAlert className="connect-status-icon is-warn" size={16} strokeWidth={1.75} aria-hidden="true" />
          <span>{CLI_MISSING_COPY}</span>
        </p>}
        {client !== 'other' ? <ClientSection
          key={client}
          name={client === 'claude' ? 'Claude Code' : 'Codex'}
          shell={data.shell}
          client={data[client]}
          cliKnown={cliKnown}
          after={client === 'claude' ? CLAUDE_AFTER_COPY : CODEX_AFTER_COPY}
          onCopy={(t) => void copy(t)}
        /> : <section className="connect-section" aria-label="Other MCP clients">
          <h3 className="connect-section-title">Connect another MCP client</h3>
          <p className="connect-muted">{OTHER_MCP_COPY}</p>
          <p className="connect-muted">Use the tool shown under Command-line tool and data file to run casebook setup mcp. Add the printed configuration to your client's settings, then restart the client.</p>
        </section>}
        <section className="connect-section" aria-label="Your first finding">
          <h3 className="connect-section-title">Ask for your first finding</h3>
          <p className="connect-muted">In {client === 'claude' ? 'Claude Code' : client === 'codex' ? 'Codex' : 'your connected AI client'}, start with this prompt and adapt the question to your business.</p>
          <p className="connect-prompt">{FIRST_FINDING_PROMPT}</p>
          <Button variant="secondary" icon={Copy} onClick={() => void copy(FIRST_FINDING_PROMPT)}>Copy first finding prompt</Button>
          <p className="connect-muted">Then open the case here, select the claim and read its linked evidence. Choose Verify myself only after you have checked it. Write a conclusion and use Share summary to review what you send to your team.</p>
        </section>
        <details className="connect-disclosure">
          <summary>Command-line tool and data file</summary>
          <section className="connect-details-body" aria-label="Command-line tool">
            {cliKnown && <CodeLine text={data.cli_path} copyLabel="Copy the command-line tool path" onCopy={(t) => void copy(t)} />}
            {data.cli_source === 'path' && <p className="connect-muted">{CLI_ON_PATH_COPY}</p>}
            {!cliKnown && <p className="connect-muted">Building from source? Run scripts/build-cli.sh to build the command-line tool. Casebook looks in the bin folder next to this app, then on PATH. Until it finds the tool, the setup commands use the name casebook.</p>}
            {data.db_path && <p className="connect-muted connect-wrap">
              This window uses <BreakablePath text={data.db_path} />, so the commands pass it with --db and clients open the same data.
            </p>}
            <p className="connect-muted">Casebook only reads settings in this panel. It never runs the commands or edits client settings for you.</p>
          </section>
        </details>
      </>}

    </Dialog>
  )
}
