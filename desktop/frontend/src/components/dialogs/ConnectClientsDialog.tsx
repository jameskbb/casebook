// Connect AI clients: where the casebook command-line tool
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
  'Casebook could not find the helper program your AI uses to reach it, so the commands below will not work yet. Install Casebook again to get it.'
export const CLI_ON_PATH_COPY =
  'This test version uses the helper program already installed on this computer.'

export const CLAUDE_AFTER_COPY =
  'Then restart Claude Code. Type /casebook there to work with your questions.'
export const CODEX_AFTER_COPY = 'Then start a new chat in Codex and ask it to use Casebook.'
export const OTHER_MCP_COPY =
  'Run casebook setup mcp. It prints the settings to paste into your AI tool. The tool has to support MCP, the usual way AI tools connect to other apps.'

export const FIRST_FINDING_PROMPT = 'Help me find out why calls to our call center went up when sales stayed the same. First ask me which months and which reports to use. Save one finding in Casebook with the source it came from, and note what we still need to check.'

type ClientChoice = 'claude' | 'codex' | 'other'

type Status = { tone: 'ok' | 'warn' | 'off'; label: string }

/** The one-line status of an AI tool. */
export function clientStatus(c: ClientConnection, cliKnown: boolean): Status {
  if (c.config_problem) return { tone: 'warn', label: "Settings file can't be read" }
  if (!c.configured) return { tone: 'off', label: 'Not connected' }
  if (!c.command_exists) return { tone: 'warn', label: 'Connected to a copy of Casebook that is gone' }
  if (cliKnown && !c.matches_cli) return { tone: 'warn', label: 'Connected to a different copy of Casebook' }
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

/** "Run in PowerShell" on Windows, "Run in a terminal" elsewhere. */
export function runLabel(shell: string | undefined): string {
  return shell && shell !== 'terminal' ? `Run this in ${shell}` : 'Run this in a terminal window'
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
      {status.tone === 'warn' && !client.config_problem && <p className="connect-muted">Run the command again to fix this.</p>}
      <p className="connect-muted">{after}</p>
      <details className="connect-disclosure">
        <summary>Other ways to connect</summary>
        <div className="connect-details-body">
          {client.setup_command_cmd && <>
            <p className="connect-label">Or in Command Prompt</p>
            <CodeLine text={client.setup_command_cmd} copyLabel={`Copy the ${name} setup command for Command Prompt`} onCopy={onCopy} />
          </>}
          {client.setup_command_wsl && <>
            <p className="connect-label">Or in WSL (bash), for {name} installed inside WSL</p>
            <CodeLine text={client.setup_command_wsl} copyLabel={`Copy the ${name} setup command for WSL`} onCopy={onCopy} />
            {name === 'Claude Code' && <p className="connect-muted">The WSL command connects Claude Code only. It does not add the reminders to save findings. Add those by hand as the Claude Code guide shows.</p>}
          </>}
          <p className="connect-label">Or add the settings by hand</p>
          <pre className="connect-code connect-pre">{client.snippet}</pre>
          <Button variant="secondary" icon={Copy} onClick={() => onCopy(client.snippet)}>Copy settings</Button>
        </div>
      </details>
      <details className="connect-disclosure">
        <summary>{name} settings</summary>
        <div className="connect-details-body">
          <p className="connect-muted connect-wrap">Settings: <BreakablePath text={client.config_path} /></p>
          <p className="connect-muted">This reads the settings file. It cannot tell whether {name} is running. Choose Check again after you run the command.</p>
          {client.hooks.length > 0 && <p className="connect-muted">Reminders added: {client.hooks.join(', ')}</p>}
          {name === 'Claude Code' && <p className="connect-muted">In the command, --hooks opens your AI's current question when Claude Code starts and reminds it to save findings. --skill adds the /casebook command.</p>}
          {name === 'Codex' && <p className="connect-muted">In the command, --skill adds the /casebook command.</p>}
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

  // Keep the choice of AI tool at the top when the status first arrives.
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
      title="Connect your AI"
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
        Choose the AI tool you use. Copy its command and run it yourself. The command saves a copy of the tool's settings first and adds only Casebook.
      </p>
      <SegmentedControl<ClientChoice> label="Your AI tool" value={client} onChange={setClient} options={[
        { value: 'claude', label: 'Claude Code' },
        { value: 'codex', label: 'Codex' },
        { value: 'other', label: 'Another AI tool' },
      ]} />
      {error && <p className="connect-error" role="alert">{error}</p>}
      {loading && !data && <p className="connect-muted" role="status">Checking your settings...</p>}
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
        /> : <section className="connect-section" aria-label="Another AI tool">
          <h3 className="connect-section-title">Connect another AI tool</h3>
          <p className="connect-muted">{OTHER_MCP_COPY}</p>
          <p className="connect-muted">Paste what it prints into your AI tool's settings, then restart the tool. The helper program that runs the command is listed under Technical details below.</p>
        </section>}
        <section className="connect-section" aria-label="Your first finding">
          <h3 className="connect-section-title">Ask for your first finding</h3>
          <p className="connect-muted">In {client === 'claude' ? 'Claude Code' : client === 'codex' ? 'Codex' : 'your AI tool'}, start with this message. Change the question to your own.</p>
          <p className="connect-prompt">{FIRST_FINDING_PROMPT}</p>
          <Button variant="secondary" icon={Copy} onClick={() => void copy(FIRST_FINDING_PROMPT)}>Copy this message</Button>
          <p className="connect-muted">Then open the question here, open the finding and read its sources. Choose I checked this only after you have checked it. Write the answer and use Share the answer to review what you send to your team.</p>
        </section>
        <details className="connect-disclosure">
          <summary>Technical details</summary>
          <section className="connect-details-body" aria-label="Technical details">
            {cliKnown && <CodeLine text={data.cli_path} copyLabel="Copy the location of the helper program" onCopy={(t) => void copy(t)} />}
            {data.cli_source === 'path' && <p className="connect-muted">{CLI_ON_PATH_COPY}</p>}
            {!cliKnown && <p className="connect-muted">Working from the source code? Run scripts/build-cli.sh to build the helper program. Casebook looks in the bin folder next to this app, then among the programs installed on this computer. Until it finds one, the commands use the name casebook.</p>}
            {data.db_path && <p className="connect-muted connect-wrap">
              This window uses <BreakablePath text={data.db_path} />. The commands include it after --db, so your AI opens the same data.
            </p>}
            <p className="connect-muted">Casebook only reads settings here. It never runs the commands or changes your AI tool's settings for you.</p>
          </section>
        </details>
      </>}

    </Dialog>
  )
}
