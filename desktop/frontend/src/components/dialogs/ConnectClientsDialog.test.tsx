import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import { WENT_WRONG_COPY } from '../../api/errors'
import type { ClientConnection } from '../../api/types'
import { AppDataProvider } from '../../state/appData'
import { CLI_PATH, CLI_WSL_PATH, clientConnections } from '../../test/fixtures.clients'
import { mockApi } from '../../test/mockApi'
import { OpenAtLaunch } from '../../test/OpenAtLaunch'
import { DataOpsProvider } from '../sidebar/DataOps'
import { SETTINGS_LABEL, SidebarFooter } from '../sidebar/SidebarFooter'
import { ToastProvider } from '../ui/Toaster'
import {
  CLAUDE_AFTER_COPY,
  CLI_MISSING_COPY,
  CLI_ON_PATH_COPY,
  clientStatus,
  CODEX_AFTER_COPY,
  ConnectClientsDialog,
  FIRST_FINDING_PROMPT,
  NEEDS_FIXING,
  OTHER_MCP_COPY,
  runLabel,
} from './ConnectClientsDialog'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../../api/api', () => ({ api: mocks.api }))

function renderDialog(onClose = vi.fn()) {
  render(<ToastProvider><ConnectClientsDialog onClose={onClose} /></ToastProvider>)
  return onClose
}

const base: ClientConnection = clientConnections.codex
const PF = 'C:\\Program Files\\Casebook\\bin\\casebook.exe'

/** Both tools with a Command Prompt line, as for an install path with a space. */
const withCmd = {
  ...clientConnections,
  claude: { ...clientConnections.claude, setup_command: `& '${PF}' setup claude --hooks --skill`, setup_command_cmd: `"${PF}" setup claude --hooks --skill` },
}

describe('clientStatus', () => {
  it('names each state', () => {
    expect(clientStatus(base, true)).toEqual({ tone: 'off', label: 'Not connected' })
    expect(clientStatus({ ...base, config_problem: true }, true)).toEqual({ tone: 'warn', label: NEEDS_FIXING })
    const on = { ...base, configured: true, command_exists: true, matches_cli: true }
    expect(clientStatus(on, true)).toEqual({ tone: 'ok', label: 'Connected' })
    expect(clientStatus({ ...on, config_problem: true }, true).label).toBe(NEEDS_FIXING)
    expect(clientStatus({ ...on, matches_cli: false }, true)).toEqual({ tone: 'warn', label: NEEDS_FIXING })
    expect(clientStatus({ ...on, matches_cli: false }, false).label).toBe('Connected')
    expect(clientStatus({ ...on, command_exists: false }, true)).toEqual({ tone: 'warn', label: NEEDS_FIXING })
    expect(clientStatus({ ...on, command_exists: false }, false).label).toBe(NEEDS_FIXING)
  })

  it('only ever says Connected, Not connected or Needs fixing', () => {
    const bools = [false, true]
    const labels = new Set<string>()
    for (const config_problem of bools)
      for (const configured of bools)
        for (const command_exists of bools)
          for (const matches_cli of bools)
            for (const cliKnown of bools)
              labels.add(clientStatus({ ...base, config_problem, configured, command_exists, matches_cli }, cliKnown).label)
    expect([...labels].sort()).toEqual(['Connected', NEEDS_FIXING, 'Not connected'])
  })
})

describe('runLabel', () => {
  it('gives the hand-off sentence for PowerShell', () => {
    expect(runLabel('PowerShell')).toBe(
      'Copy this line, paste it into PowerShell and press Enter. Never used PowerShell? Send this line to whoever set up your AI tool.',
    )
  })

  it('gives the hand-off sentence for the Terminal app', () => {
    const terminal =
      'Copy this line, paste it into the Terminal app and press Enter. Never used the Terminal app? Send this line to whoever set up your AI tool.'
    expect(runLabel('terminal')).toBe(terminal)
    expect(runLabel(undefined)).toBe(terminal)
  })
})

describe('ConnectClientsDialog', () => {
  beforeEach(() => { Object.assign(mocks.api, mockApi()) })

  it('shows one line at a time, with its status and hand-off sentence, and switches tools', async () => {
    const user = userEvent.setup()
    renderDialog()
    const claude = await screen.findByRole('region', { name: 'Claude Code' })
    expect(within(claude).getByText('Connected')).toBeInTheDocument()
    expect(within(claude).getByText(runLabel('PowerShell'))).toBeInTheDocument()
    expect(within(claude).getByText(`${CLI_PATH} setup claude --hooks --skill`)).toBeInTheDocument()
    expect(within(claude).getByText(CLAUDE_AFTER_COPY)).toBeInTheDocument()
    expect(screen.queryByRole('region', { name: 'Codex' })).not.toBeInTheDocument()
    await user.click(screen.getByRole('radio', { name: 'Codex' }))
    const codex = screen.getByRole('region', { name: 'Codex' })
    expect(within(codex).getByText('Not connected')).toBeInTheDocument()
    expect(within(codex).getByText(CODEX_AFTER_COPY)).toBeInTheDocument()
    await user.click(within(codex).getByRole('button', { name: 'Copy the line for Codex' }))
    expect(mocks.api.copyText).toHaveBeenCalledWith(`${CLI_PATH} setup codex --skill`)
    expect(await screen.findByText('Copied.')).toBeInTheDocument()
  })

  it('says Needs fixing when the settings point at a different copy', async () => {
    Object.assign(mocks.api, mockApi({ clientConnections: async () => ({
      ...clientConnections, claude: { ...clientConnections.claude, matches_cli: false },
    }) }))
    renderDialog()
    const claude = await screen.findByRole('region', { name: 'Claude Code' })
    expect(within(claude).getByText(NEEDS_FIXING)).toBeInTheDocument()
  })

  it('leads with the Terminal app sentence off Windows and offers no WSL line', async () => {
    Object.assign(mocks.api, mockApi({ clientConnections: async () => ({
      ...clientConnections, shell: 'terminal',
      claude: { ...clientConnections.claude, setup_command_wsl: '' },
      codex: { ...clientConnections.codex, setup_command_wsl: '' },
    }) }))
    renderDialog()
    const claude = await screen.findByRole('region', { name: 'Claude Code' })
    expect(within(claude).getByText(runLabel('terminal'))).toBeInTheDocument()
    expect(screen.queryByText(/WSL/)).not.toBeInTheDocument()
  })

  it('keeps everything else under one Technical details disclosure', async () => {
    Object.assign(mocks.api, mockApi({ clientConnections: async () => withCmd }))
    const user = userEvent.setup()
    renderDialog()
    await screen.findByRole('region', { name: 'Claude Code' })
    const disclosures = document.querySelectorAll('details')
    expect(disclosures).toHaveLength(1)
    const details = disclosures[0]
    expect(details.querySelector('summary')).toHaveTextContent('Technical details')
    expect(details).not.toHaveAttribute('open')
    const inside = within(details)
    const hidden = [
      inside.getByText('The same line for Command Prompt'),
      inside.getByText('The same line for WSL (bash), for Claude Code installed inside WSL'),
      inside.getByText('Or add the settings by hand'),
      inside.getByText(/--hooks opens the question your AI is working on/),
      inside.getByText(/--skill adds the \/casebook command/),
      inside.getByText(/Reminders added: SessionStart, Stop, PreCompact/),
      inside.getByText(/Casebook reads the settings file/),
    ]
    for (const el of hidden) expect(el).not.toBeVisible()
    // Outside the disclosure the only technical words are the line itself.
    const outside = document.querySelector('.dialog-body')!.cloneNode(true) as HTMLElement
    outside.querySelector('details')!.remove()
    outside.querySelectorAll('code').forEach((el) => el.remove())
    expect(outside).not.toHaveTextContent(/Command Prompt|WSL|by hand|--hooks|--skill/)

    await user.click(inside.getByText('Technical details'))
    for (const el of hidden) expect(el).toBeVisible()
    await user.click(inside.getByRole('button', { name: 'Copy the line for Claude Code in Command Prompt' }))
    expect(mocks.api.copyText).toHaveBeenLastCalledWith(`"${PF}" setup claude --hooks --skill`)
    await user.click(inside.getByRole('button', { name: 'Copy the line for Claude Code in WSL' }))
    expect(mocks.api.copyText).toHaveBeenLastCalledWith(`claude mcp add --scope user casebook -- ${CLI_WSL_PATH} mcp`)
    await user.click(inside.getByRole('button', { name: 'Copy settings' }))
    expect(mocks.api.copyText).toHaveBeenLastCalledWith(clientConnections.claude.snippet)

    await user.click(screen.getByRole('radio', { name: 'Codex' }))
    expect(document.querySelectorAll('details')).toHaveLength(1)
    expect(inside.queryByText('The same line for Command Prompt')).not.toBeInTheDocument()
    expect(inside.queryByText(/--hooks/)).not.toBeInTheDocument()
    await user.click(inside.getByRole('button', { name: 'Copy the line for Codex in WSL' }))
    expect(mocks.api.copyText).toHaveBeenLastCalledWith(`codex mcp add casebook -- ${CLI_WSL_PATH} mcp`)
  })

  it('never mentions the source code or a build script', async () => {
    Object.assign(mocks.api, mockApi({ clientConnections: async () => ({ ...withCmd, cli_path: '', cli_source: 'missing', cli_name: 'casebook' }) }))
    const user = userEvent.setup()
    renderDialog()
    const dialog = await screen.findByRole('dialog', { name: 'Connect your AI' })
    await screen.findByRole('region', { name: 'Claude Code' })
    for (const tool of ['Claude Code', 'Codex', 'Another AI tool']) {
      await user.click(screen.getByRole('radio', { name: tool }))
      expect(dialog).not.toHaveTextContent(/source code/i)
      expect(dialog).not.toHaveTextContent('scripts/build-cli.sh')
    }
  })

  it('gives another AI tool its own instructions', async () => {
    const user = userEvent.setup()
    renderDialog()
    await screen.findByRole('region', { name: 'Claude Code' })
    await user.click(screen.getByRole('radio', { name: 'Another AI tool' }))
    expect(within(screen.getByRole('region', { name: 'Another AI tool' })).getByText(OTHER_MCP_COPY)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /^Copy the line for/ })).not.toBeInTheDocument()
  })

  it('copies a business question prompt and leads from the finding to checking and sharing', async () => {
    const user = userEvent.setup()
    renderDialog()
    const finding = await screen.findByRole('region', { name: 'Your first finding' })
    await user.click(within(finding).getByRole('button', { name: 'Copy this message' }))
    expect(mocks.api.copyText).toHaveBeenCalledWith(FIRST_FINDING_PROMPT)
    expect(FIRST_FINDING_PROMPT).toMatch(/why calls to our call center went up when sales stayed the same/)
    expect(FIRST_FINDING_PROMPT).toMatch(/First ask me which months and which reports to use/)
    expect(FIRST_FINDING_PROMPT).toMatch(/with the source it came from/)
    expect(within(finding).getByText(/Choose I checked this only after you have checked it/)).toHaveTextContent(/Share the answer/)
  })

  it('reports a clipboard failure', async () => {
    Object.assign(mocks.api, { copyText: vi.fn(async () => Promise.reject(new Error('no clipboard'))) })
    const user = userEvent.setup()
    renderDialog()
    await user.click(await screen.findByRole('button', { name: 'Copy the line for Claude Code' }))
    expect(await screen.findByRole('alert')).toBeInTheDocument()
  })

  it('explains a missing helper program before the line', async () => {
    Object.assign(mocks.api, { clientConnections: vi.fn(async () => ({ ...clientConnections, cli_path: '', cli_source: 'missing', cli_name: 'casebook' })) })
    renderDialog()
    expect(await screen.findByText(CLI_MISSING_COPY)).toBeVisible()
  })

  it('discloses where the program is, a test version and where the questions are kept', async () => {
    Object.assign(mocks.api, { clientConnections: vi.fn(async () => ({ ...clientConnections, cli_source: 'path', db_path: 'D:\\cases\\casebook.db' })) })
    const user = userEvent.setup()
    renderDialog()
    await screen.findByRole('region', { name: 'Claude Code' })
    await user.click(screen.getByText('Technical details'))
    const cli = screen.getByRole('region', { name: 'Technical details' })
    expect(within(cli).getByText(CLI_ON_PATH_COPY)).toBeVisible()
    expect(within(cli).getByText(/This window keeps your questions in D:\\cases\\casebook.db/)).toBeVisible()
    await user.click(within(cli).getByRole('button', { name: 'Copy where the program is' }))
    expect(mocks.api.copyText).toHaveBeenCalledWith(CLI_PATH)
  })

  it('never writes client settings: Look again only reads', async () => {
    const user = userEvent.setup()
    const onClose = renderDialog()
    await screen.findByRole('region', { name: 'Claude Code' })
    await user.click(screen.getByRole('button', { name: 'Look again' }))
    await waitFor(() => expect(mocks.api.clientConnections).toHaveBeenCalledTimes(2))
    await user.click(screen.getByRole('button', { name: 'Done' }))
    expect(onClose).toHaveBeenCalled()
    const called = Object.entries(mocks.api)
      .filter(([, fn]) => (fn as unknown as { mock: { calls: unknown[] } }).mock.calls.length > 0)
      .map(([name]) => name)
    expect(called.sort()).toEqual(['clientConnections'])
  })

  it('shows a plain error, never the engine text, and Look again loads the settings', async () => {
    Object.assign(mocks.api, { clientConnections: vi.fn()
      .mockRejectedValueOnce(new Error(JSON.stringify({ kind: 'other', message: 'Casebook cannot find your home folder' })))
      .mockResolvedValue(clientConnections) })
    const user = userEvent.setup()
    renderDialog()
    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent(WENT_WRONG_COPY)
    expect(alert).not.toHaveTextContent(/home folder/)
    await user.click(screen.getByRole('button', { name: 'Look again' }))
    expect(await screen.findByRole('region', { name: 'Claude Code' })).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })
})

describe('Settings menu entry', () => {
  beforeEach(() => { Object.assign(mocks.api, mockApi()) })

  it('opens the panel from the Settings menu', async () => {
    const user = userEvent.setup()
    render(<ToastProvider><AppDataProvider><OpenAtLaunch /><DataOpsProvider><SidebarFooter /></DataOpsProvider></AppDataProvider></ToastProvider>)
    await user.click(screen.getByRole('button', { name: SETTINGS_LABEL }))
    await user.click(await screen.findByRole('menuitem', { name: 'Connect your AI...' }))
    expect(await screen.findByRole('dialog', { name: 'Connect your AI' })).toBeInTheDocument()
    await screen.findByRole('region', { name: 'Claude Code' })
  })
})
