// Connect your AI: one button per AI tool found on this computer. "Connect
// it for me" has Casebook add itself to that tool's settings (it keeps a
// backup of the file), the section then says Connected and offers
// Disconnect. Tools not found here are named under Another AI tool. Then a
// first message to start with. Everything a person does not need to act (the
// line to run by hand, where the settings are, the program an AI uses to
// reach Casebook) sits under Technical details.
import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react'
import { CircleCheck, CircleDashed, Copy, TriangleAlert } from 'lucide-react'
import { api } from '../../api/api'
import { CLIPBOARD_COPY, errorCopy, plainError, toApiError } from '../../api/errors'
import type { ClientChange, ClientConnection, ClientConnections, ClientTool } from '../../api/types'
import { CLIENT_TOOLS, isConnected, type ToolName } from '../../lib/clients'
import { useNoteClients } from '../../state/appData'
import { BreakablePath } from '../ui/BreakablePath'
import { Button } from '../ui/Button'
import { Dialog } from '../ui/Dialog'
import { IconButton } from '../ui/IconButton'
import { useToast } from '../ui/Toaster'
import './ConnectClientsDialog.css'

export const CLI_MISSING_COPY =
  'Casebook could not find the part of it your AI talks to, so it cannot connect an AI tool yet. Install Casebook again to fix this.'
export const CLI_ON_PATH_COPY =
  'This test version uses the helper program already installed on this computer.'

export const CONNECT_ACTION = 'Connect it for me'
export const DISCONNECT_ACTION = 'Disconnect'
export const CONNECTED = 'Connected'
export const NOT_CONNECTED = 'Not connected'
export const SETTINGS_UNREADABLE = 'Casebook cannot read its settings'
export const NONE_FOUND_COPY = 'Casebook did not find an AI tool on this computer that it can connect by itself.'
export const BY_HAND_COPY = 'Casebook cannot connect this one by itself. Send the steps under Technical details to the person who set up your AI tool.'
export const OTHER_TOOL_COPY = 'Using a different AI tool? Send the steps under Technical details to the person who set it up.'
export const OTHER_MCP_COPY =
  'Another AI tool: run casebook setup mcp, paste what it prints into the tool\'s settings, then restart the tool. The tool has to support MCP, the usual way AI tools connect to other apps.'

/** What the button will do, in one sentence. */
export const connectChanges = (name: string) => `Casebook adds itself to ${name}'s settings and keeps a backup of the file.`

export const FIRST_FINDING_PROMPT = 'Help me find out why calls to our call center went up when sales stayed the same. First ask me which months and which numbers to use. Save one finding in Casebook with the source it came from, and note what we still need to check.'

type Status = { tone: 'ok' | 'warn' | 'off'; label: string }

/** The one-line state of an AI tool. */
export function clientStatus(c: ClientConnection): Status {
  if (c.config_problem) return { tone: 'warn', label: SETTINGS_UNREADABLE }
  return isConnected(c) ? { tone: 'ok', label: CONNECTED } : { tone: 'off', label: NOT_CONNECTED }
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

/** Where the line is pasted: "PowerShell" on Windows, "the Terminal app" elsewhere. */
export function shellName(shell: string | undefined): string {
  return shell && shell !== 'terminal' ? shell : 'the Terminal app'
}

interface ToolSectionProps {
  tool: ToolName
  client: ClientConnection
  /** Casebook's helper program is there, so connecting can work. */
  canConnect: boolean
  /** Called after a change, with what it did, so the window reads the settings again. */
  onChanged: (change: ClientChange) => void
  /** This tool's button was the last one pressed: only then is its result shown. */
  current: boolean
  onPressed: (tool: ClientTool) => void
}

/** One AI tool found on this computer: what it is, whether it is connected, and the one button. */
function ToolSection({ tool: { tool, title, name }, client, canConnect, onChanged, current, onPressed }: ToolSectionProps) {
  const status = clientStatus(client)
  const connected = isConnected(client)
  const [busy, setBusy] = useState(false)
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')

  const change = async (call: (t: ClientTool) => Promise<ClientChange>) => {
    if (busy) return
    onPressed(tool)
    setBusy(true)
    setMessage('')
    setError('')
    try {
      const done = await call(tool)
      setMessage(done.message)
      onChanged(done)
    } catch (e) {
      const err = toApiError(e)
      // The messages for connecting are written for a person; a tool Casebook cannot write to says how to do it by hand.
      setError(err.kind === 'validation' ? BY_HAND_COPY : err.kind === 'busy' || err.kind === 'internal' || !err.message ? plainError(err) : err.message)
    } finally {
      setBusy(false)
    }
  }

  return (
    <section className="connect-section" aria-label={title}>
      <div className="connect-section-head">
        <h3 className="connect-section-title">{title}</h3>
        <span className={`connect-status is-${status.tone}`}>
          <StatusIcon tone={status.tone} />
          {status.label}
        </span>
      </div>
      {connected ? (
        <Button variant="secondary" busy={busy} busyLabel="Disconnecting..." aria-label={`${DISCONNECT_ACTION}: ${title}`} onClick={() => void change(api.disconnectClient)}>
          {DISCONNECT_ACTION}
        </Button>
      ) : (
        <>
          <p className="connect-muted">{connectChanges(name)}</p>
          <Button variant="primary" busy={busy} busyLabel="Connecting..." disabled={!canConnect} aria-label={`${CONNECT_ACTION}: ${title}`} onClick={() => void change(api.connectClient)}>
            {CONNECT_ACTION}
          </Button>
        </>
      )}
      {current && message && <p className="connect-muted" role="status">{message}</p>}
      {current && error && <p className="connect-error" role="alert">{error}</p>}
    </section>
  )
}

/** What whoever set up the AI tool needs to do it by hand. */
function ToolTechnical({ title, client, shell, onCopy }: { title: string; client: ClientConnection; shell?: string; onCopy: (text: string) => void }) {
  return (
    <>
      <p className="connect-label">{title}</p>
      {client.setup_command && <>
        <p className="connect-muted">Paste this line into {shellName(shell)} and press Enter. It does the same as the button.</p>
        <CodeLine text={client.setup_command} copyLabel={`Copy the line for ${title}`} onCopy={onCopy} />
      </>}
      {client.setup_command_cmd && <>
        <p className="connect-muted">The same line for Command Prompt</p>
        <CodeLine text={client.setup_command_cmd} copyLabel={`Copy the line for ${title} in Command Prompt`} onCopy={onCopy} />
      </>}
      {client.setup_command_wsl && <>
        <p className="connect-muted">The same line for WSL (bash), for {title} installed inside WSL</p>
        <CodeLine text={client.setup_command_wsl} copyLabel={`Copy the line for ${title} in WSL`} onCopy={onCopy} />
      </>}
      <p className="connect-muted">Or add this to the settings by hand</p>
      <pre className="connect-code connect-pre">{client.snippet}</pre>
      <Button variant="secondary" icon={Copy} aria-label={`Copy settings for ${title}`} onClick={() => onCopy(client.snippet)}>Copy settings</Button>
      <p className="connect-muted connect-wrap">{title} settings: <BreakablePath text={client.config_path} /></p>
      {client.hooks.length > 0 && <p className="connect-muted">Reminders added: {client.hooks.join(', ')}</p>}
    </>
  )
}

export interface ConnectClientsDialogProps {
  onClose: () => void
}

export function ConnectClientsDialog({ onClose }: ConnectClientsDialogProps) {
  const toast = useToast()
  const doneRef = useRef<HTMLButtonElement>(null)
  const [data, setData] = useState<ClientConnections | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [lastChange, setLastChange] = useState<ClientChange | null>(null)
  // The tool whose button was pressed last; a result shows under that tool only.
  const [pressed, setPressed] = useState<ClientTool | null>(null)
  const noteClients = useNoteClients()

  // Read every time the window opens and after each change.
  const load = useCallback(async () => {
    setLoading(true)
    try {
      const c = await api.clientConnections()
      setData(c)
      noteClients(c)
      setError(null)
    } catch (e) {
      setError(errorCopy(toApiError(e)).message)
    } finally {
      setLoading(false)
    }
  }, [noteClients])

  useEffect(() => {
    void load()
  }, [load])

  // Keep the top of the window in view when the state first arrives.
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
  // A backend that does not say whether a tool was found is an older one: its tools are offered as before.
  const known = data ? CLIENT_TOOLS.filter((t) => data[t.tool]) : []
  const found = known.filter((t) => data?.[t.tool]?.found !== false)
  const missing = known.filter((t) => data?.[t.tool]?.found === false)
  return (
    <Dialog
      title="Connect your AI"
      onClose={onClose}
      initialFocus={doneRef}
      className="connect-dialog"
      footer={
        <Button variant="primary" ref={doneRef} onClick={onClose}>
          Done
        </Button>
      }
    >
      <p className="dialog-text" ref={introRef}>
        Connect the AI tool you use, so it can save what it finds here. You do this once.
      </p>
      {error && <p className="connect-error" role="alert">{error}</p>}
      {loading && !data && <p className="connect-muted" role="status">Looking at your settings...</p>}
      {data && <>
        {!cliKnown && <p className="connect-note">
          <TriangleAlert className="connect-status-icon is-warn" size={16} strokeWidth={1.75} aria-hidden="true" />
          <span>{CLI_MISSING_COPY}</span>
        </p>}
        {found.map((t) => (
          <ToolSection
            key={t.tool}
            tool={t}
            client={data[t.tool]!}
            canConnect={cliKnown}
            current={pressed === t.tool}
            onPressed={setPressed}
            onChanged={(change) => {
              setLastChange(change)
              void load()
            }}
          />
        ))}
        {found.length === 0 && <p className="connect-muted">{NONE_FOUND_COPY}</p>}
        <section className="connect-section" aria-label="Another AI tool">
          <h3 className="connect-section-title">Another AI tool</h3>
          {missing.length > 0 && <p className="connect-muted">Not found on this computer: {missing.map((t) => t.title).join(', ')}.</p>}
          <p className="connect-muted">{OTHER_TOOL_COPY}</p>
        </section>
        <section className="connect-section" aria-label="Your first finding">
          <h3 className="connect-section-title">Ask for your first finding</h3>
          <p className="connect-muted">In your AI tool, start with this message. Change the question to your own.</p>
          <p className="connect-prompt">{FIRST_FINDING_PROMPT}</p>
          <Button variant="secondary" icon={Copy} onClick={() => void copy(FIRST_FINDING_PROMPT)}>Copy this message</Button>
          <p className="connect-muted">Then open the question here and choose Check the next source. Write the answer, then use Share the answer to send it to your team.</p>
        </section>
        <details className="connect-disclosure">
          <summary>Technical details</summary>
          <section className="connect-details-body" aria-label="Technical details">
            {known.map((t) => <ToolTechnical key={t.tool} title={t.title} client={data[t.tool]!} shell={data.shell} onCopy={(text) => void copy(text)} />)}
            <p className="connect-muted">{OTHER_MCP_COPY}</p>
            {lastChange && lastChange.files.length > 0 && <>
              <p className="connect-label">What the last change did</p>
              <ul className="connect-files">
                {lastChange.files.map((f) => (
                  <li key={f.file} className="connect-muted connect-wrap">
                    <BreakablePath text={f.file} />: {f.action}{f.backup && <>, backup at <BreakablePath text={f.backup} /></>}
                  </li>
                ))}
              </ul>
            </>}
            {cliKnown && <>
              <p className="connect-label">The program your AI uses to reach Casebook</p>
              <CodeLine text={data.cli_path} copyLabel="Copy where the program is" onCopy={(t) => void copy(t)} />
            </>}
            {data.cli_source === 'path' && <p className="connect-muted">{CLI_ON_PATH_COPY}</p>}
            {data.db_path && <p className="connect-muted connect-wrap">
              This window keeps your questions in <BreakablePath text={data.db_path} />. The lines include it after --db, so your AI opens the same questions.
            </p>}
          </section>
        </details>
      </>}

    </Dialog>
  )
}
