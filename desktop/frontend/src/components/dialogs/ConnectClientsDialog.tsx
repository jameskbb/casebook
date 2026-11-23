// Connect your AI: the one line that connects Claude Code or Codex, whether
// each is connected already, and a first message to start with. The panel
// only reads: an AI tool's settings are written by the line the person runs,
// which saves a copy of the settings first and changes only Casebook's entry.
// Everything a person does not need to act (other ways to connect, where the
// settings are, the program an AI uses to reach Casebook) sits under
// Technical details.
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
  'Casebook could not find the part of it your AI talks to, so the line below will not work yet. Install Casebook again to fix this.'
export const CLI_ON_PATH_COPY =
  'This test version uses the helper program already installed on this computer.'

export const CLAUDE_AFTER_COPY =
  'Then restart Claude Code. Type /casebook there to work with your questions.'
export const CODEX_AFTER_COPY = 'Then start a new chat in Codex and ask it to use Casebook.'
export const OTHER_MCP_COPY =
  'Send this to whoever set up your AI tool: run casebook setup mcp, paste what it prints into the tool\'s settings, then restart the tool. The tool has to support MCP, the usual way AI tools connect to other apps.'

export const FIRST_FINDING_PROMPT = 'Help me find out why calls to our call center went up when sales stayed the same. First ask me which months and which reports to use. Save one finding in Casebook with the source it came from, and note what we still need to check.'

type ClientChoice = 'claude' | 'codex' | 'other'

type Status = { tone: 'ok' | 'warn' | 'off'; label: string }

/** The one-line state of an AI tool: Connected, Not connected, or Needs fixing. */
export function clientStatus(c: ClientConnection, cliKnown: boolean): Status {
  if (c.config_problem) return { tone: 'warn', label: NEEDS_FIXING }
  if (!c.configured) return { tone: 'off', label: 'Not connected' }
  if (!c.command_exists || (cliKnown && !c.matches_cli)) return { tone: 'warn', label: NEEDS_FIXING }
  return { tone: 'ok', label: 'Connected' }
}

export const NEEDS_FIXING = 'Needs fixing: run the line again'

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

/** Where the line is pasted: "PowerShell" on Windows, "the Terminal app" elsewhere. */
export function shellName(shell: string | undefined): string {
  return shell && shell !== 'terminal' ? shell : 'the Terminal app'
}

/** The hand-off sentence: what to do with the line, and what to do if that means nothing to you. */
export function runLabel(shell: string | undefined): string {
  const where = shellName(shell)
  return `Copy this line, paste it into ${where} and press Enter. Never used ${where}? Send this line to whoever set up your AI tool.`
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
      <CodeLine text={client.setup_command} copyLabel={`Copy the line for ${name}`} onCopy={onCopy} />
      <p className="connect-muted">{after}</p>
    </section>
  )
}

/** The rest of what Casebook knows about one AI tool, for whoever set it up. */
function ClientTechnical({ name, client, onCopy }: { name: string; client: ClientConnection; onCopy: (text: string) => void }) {
  return (
    <>
      {client.setup_command_cmd && <>
        <p className="connect-label">The same line for Command Prompt</p>
        <CodeLine text={client.setup_command_cmd} copyLabel={`Copy the line for ${name} in Command Prompt`} onCopy={onCopy} />
      </>}
      {client.setup_command_wsl && <>
        <p className="connect-label">The same line for WSL (bash), for {name} installed inside WSL</p>
        <CodeLine text={client.setup_command_wsl} copyLabel={`Copy the line for ${name} in WSL`} onCopy={onCopy} />
        {name === 'Claude Code' && <p className="connect-muted">The WSL line connects Claude Code only. It does not add the reminders to save findings. Add those by hand as the Claude Code guide shows.</p>}
      </>}
      <p className="connect-label">Or add the settings by hand</p>
      <pre className="connect-code connect-pre">{client.snippet}</pre>
      <Button variant="secondary" icon={Copy} onClick={() => onCopy(client.snippet)}>Copy settings</Button>
      <p className="connect-muted connect-wrap">{name} settings: <BreakablePath text={client.config_path} /></p>
      {client.config_problem && <p className="connect-muted">Casebook could not read this settings file.</p>}
      <p className="connect-muted">Casebook reads the settings file. It cannot tell whether {name} is running. Choose Look again after you run the line.</p>
      {client.hooks.length > 0 && <p className="connect-muted">Reminders added: {client.hooks.join(', ')}</p>}
      {name === 'Claude Code' && <p className="connect-muted">In the line, --hooks opens the question your AI is working on when Claude Code starts and reminds it to save findings. --skill adds the /casebook command.</p>}
      {name === 'Codex' && <p className="connect-muted">In the line, --skill adds the /casebook command.</p>}
    </>
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

  // Keep the choice of AI tool at the top when the state first arrives.
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
      toast.success('Copied.')
    } catch {
      toast.error(CLIPBOARD_COPY)
    }
  }

  const cliKnown = Boolean(data?.cli_path)
  const name = client === 'claude' ? 'Claude Code' : client === 'codex' ? 'Codex' : 'your AI tool'
  return (
    <Dialog
      title="Connect your AI"
      onClose={onClose}
      initialFocus={doneRef}
      className="connect-dialog"
      footer={
        <>
          <Button variant="secondary" icon={RefreshCw} busy={loading} busyLabel="Looking..." onClick={() => void load()}>
            Look again
          </Button>
          <Button variant="primary" ref={doneRef} onClick={onClose}>
            Done
          </Button>
        </>
      }
    >
      <p className="dialog-text" ref={introRef}>
        Choose the AI tool you use. You do this once.
      </p>
      <SegmentedControl<ClientChoice> label="Your AI tool" value={client} onChange={setClient} options={[
        { value: 'claude', label: 'Claude Code' },
        { value: 'codex', label: 'Codex' },
        { value: 'other', label: 'Another AI tool' },
      ]} />
      {error && <p className="connect-error" role="alert">{error}</p>}
      {loading && !data && <p className="connect-muted" role="status">Looking at your settings...</p>}
      {data && <>
        {!cliKnown && <p className="connect-note">
          <TriangleAlert className="connect-status-icon is-warn" size={16} strokeWidth={1.75} aria-hidden="true" />
          <span>{CLI_MISSING_COPY}</span>
        </p>}
        {client !== 'other' ? <ClientSection
          key={client}
          name={name}
          shell={data.shell}
          client={data[client]}
          cliKnown={cliKnown}
          after={client === 'claude' ? CLAUDE_AFTER_COPY : CODEX_AFTER_COPY}
          onCopy={(t) => void copy(t)}
        /> : <section className="connect-section" aria-label="Another AI tool">
          <h3 className="connect-section-title">Connect another AI tool</h3>
          <p className="connect-muted">{OTHER_MCP_COPY}</p>
        </section>}
        <section className="connect-section" aria-label="Your first finding">
          <h3 className="connect-section-title">Ask for your first finding</h3>
          <p className="connect-muted">In {name}, start with this message. Change the question to your own.</p>
          <p className="connect-prompt">{FIRST_FINDING_PROMPT}</p>
          <Button variant="secondary" icon={Copy} onClick={() => void copy(FIRST_FINDING_PROMPT)}>Copy this message</Button>
          <p className="connect-muted">Then open the question here and read each finding and its sources. Choose I checked this only after you have checked it. Write the answer, then use Share the answer to send it to your team.</p>
        </section>
        <details className="connect-disclosure">
          <summary>Technical details</summary>
          <section className="connect-details-body" aria-label="Technical details">
            {client !== 'other' && <ClientTechnical name={name} client={data[client]} onCopy={(t) => void copy(t)} />}
            {cliKnown && <>
              <p className="connect-label">The program your AI uses to reach Casebook</p>
              <CodeLine text={data.cli_path} copyLabel="Copy where the program is" onCopy={(t) => void copy(t)} />
            </>}
            {data.cli_source === 'path' && <p className="connect-muted">{CLI_ON_PATH_COPY}</p>}
            {data.db_path && <p className="connect-muted connect-wrap">
              This window keeps your questions in <BreakablePath text={data.db_path} />. The lines include it after --db, so your AI opens the same questions.
            </p>}
            <p className="connect-muted">Casebook only reads settings here. It never runs the lines or changes your AI tool's settings for you.</p>
          </section>
        </details>
      </>}

    </Dialog>
  )
}
