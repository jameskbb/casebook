import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import { WENT_WRONG_COPY } from '../../api/errors'
import type { ClientChange, ClientConnection, ClientTool } from '../../api/types'
import { AppDataProvider } from '../../state/appData'
import { CLI_PATH, CLI_WSL_PATH, clientConnections } from '../../test/fixtures.clients'
import { mockApi } from '../../test/mockApi'
import { OpenAtLaunch } from '../../test/OpenAtLaunch'
import { DataOpsProvider } from '../sidebar/DataOps'
import { SETTINGS_LABEL, SidebarFooter } from '../sidebar/SidebarFooter'
import { ToastProvider } from '../ui/Toaster'
import {
  BY_HAND_COPY,
  CLI_MISSING_COPY,
  CLI_ON_PATH_COPY,
  clientStatus,
  CONNECT_ACTION,
  connectChanges,
  CONNECTED,
  ConnectClientsDialog,
  DISCONNECT_ACTION,
  FIRST_FINDING_PROMPT,
  NOT_CONNECTED,
  OTHER_MCP_COPY,
  SETTINGS_UNREADABLE,
} from './ConnectClientsDialog'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../../api/api', () => ({ api: mocks.api }))

function renderDialog(onClose = vi.fn()) {
  render(<ToastProvider><ConnectClientsDialog onClose={onClose} /></ToastProvider>)
  return onClose
}

const base: ClientConnection = clientConnections.codex
const PF = 'C:\\Program Files\\Casebook\\bin\\casebook.exe'

/** Claude Code with a Command Prompt line, as for an install path with a space. */
const withCmd = {
  ...clientConnections,
  claude: { ...clientConnections.claude, setup_command: `& '${PF}' setup claude --hooks --skill`, setup_command_cmd: `"${PF}" setup claude --hooks --skill` },
}

const rejectWith = (body: object) => vi.fn(async () => Promise.reject(new Error(JSON.stringify(body))))

describe('clientStatus', () => {
  it('names each state', () => {
    expect(clientStatus(base)).toEqual({ tone: 'off', label: NOT_CONNECTED })
    const on = { ...base, configured: true, command_exists: true, matches_cli: true }
    expect(clientStatus(on)).toEqual({ tone: 'ok', label: CONNECTED })
    // Settings that point at another copy of Casebook: pressing connect again fixes it.
    expect(clientStatus({ ...on, matches_cli: false })).toEqual({ tone: 'off', label: NOT_CONNECTED })
    expect(clientStatus({ ...on, config_problem: true })).toEqual({ tone: 'warn', label: SETTINGS_UNREADABLE })
  })
})

describe('ConnectClientsDialog', () => {
  beforeEach(() => { Object.assign(mocks.api, mockApi()) })

  it('offers Connect it for me per tool found, says what it changes, and names a tool not found under Another AI tool', async () => {
    renderDialog()
    const claude = await screen.findByRole('region', { name: 'Claude Code' })
    expect(within(claude).getByText(CONNECTED)).toBeInTheDocument()
    expect(within(claude).getByRole('button', { name: `${DISCONNECT_ACTION}: Claude Code` })).toBeInTheDocument()
    expect(within(claude).queryByRole('button', { name: /Connect it for me/ })).not.toBeInTheDocument()
    const codex = screen.getByRole('region', { name: 'Codex' })
    expect(within(codex).getByText(NOT_CONNECTED)).toBeInTheDocument()
    expect(within(codex).getByText(connectChanges('Codex'))).toBeInTheDocument()
    expect(within(codex).getByRole('button', { name: `${CONNECT_ACTION}: Codex` })).toBeEnabled()
    // Not found here: no button of its own, named under Another AI tool.
    expect(screen.queryByRole('region', { name: 'Claude desktop app' })).not.toBeInTheDocument()
    expect(within(screen.getByRole('region', { name: 'Another AI tool' })).getByText('Not found on this computer: Claude desktop app.')).toBeInTheDocument()
    // The line to run by hand stays hidden until Technical details opens.
    expect(screen.getByText(`${CLI_PATH} setup codex --skill`)).not.toBeVisible()
  })

  it('Connect it for me connects the tool, shows what Casebook said, reads the settings again and then says Connected', async () => {
    const after = { ...clientConnections, codex: { ...clientConnections.codex, configured: true, matches_cli: true, command: CLI_PATH, command_exists: true } }
    const message = "Casebook added itself to Codex's settings. Close Codex and open it again to start."
    Object.assign(mocks.api, mockApi({
      clientConnections: vi.fn().mockResolvedValueOnce(clientConnections).mockResolvedValue(after),
      connectClient: vi.fn(async (tool: ClientTool): Promise<ClientChange> => ({ tool, connected: true, changed: true, message, files: [{ file: 'C:\\Users\\Ada\\.codex\\config.toml', action: 'updated', backup: 'C:\\Users\\Ada\\.codex\\config.toml.bak' }] })),
    }))
    const user = userEvent.setup()
    renderDialog()
    await user.click(await screen.findByRole('button', { name: `${CONNECT_ACTION}: Codex` }))
    expect(mocks.api.connectClient).toHaveBeenCalledWith('codex')
    const codex = screen.getByRole('region', { name: 'Codex' })
    expect(await within(codex).findByText(CONNECTED)).toBeInTheDocument()
    expect(within(codex).getByRole('status')).toHaveTextContent(message)
    expect(mocks.api.clientConnections).toHaveBeenCalledTimes(2)
    expect(within(codex).getByRole('button', { name: `${DISCONNECT_ACTION}: Codex` })).toBeInTheDocument()
    // What was written, with its backup, sits under Technical details.
    await user.click(screen.getByText('Technical details'))
    expect(within(screen.getByRole('region', { name: 'Technical details' })).getByText(/backup at/)).toBeVisible()
  })

  it('Disconnect takes Casebook out of the tool', async () => {
    const user = userEvent.setup()
    renderDialog()
    await user.click(await screen.findByRole('button', { name: `${DISCONNECT_ACTION}: Claude Code` }))
    expect(mocks.api.disconnectClient).toHaveBeenCalledWith('claude')
    expect(await within(screen.getByRole('region', { name: 'Claude Code' })).findByRole('status')).toHaveTextContent(/took itself out of Claude Code's settings/)
  })

  it('a tool Casebook cannot connect by itself points to the steps under Technical details', async () => {
    Object.assign(mocks.api, mockApi({
      clientConnections: async () => ({ ...clientConnections, claude_desktop: { ...clientConnections.claude_desktop!, found: true } }),
      connectClient: rejectWith({ kind: 'validation', field: 'tool', message: 'Casebook does not know this AI tool.' }),
    }))
    const user = userEvent.setup()
    renderDialog()
    await user.click(await screen.findByRole('button', { name: `${CONNECT_ACTION}: Claude desktop app` }))
    const desktop = screen.getByRole('region', { name: 'Claude desktop app' })
    expect(await within(desktop).findByRole('alert')).toHaveTextContent(BY_HAND_COPY)
    await user.click(screen.getByText('Technical details'))
    await user.click(screen.getByRole('button', { name: 'Copy settings for Claude desktop app' }))
    expect(mocks.api.copyText).toHaveBeenLastCalledWith(clientConnections.claude_desktop!.snippet)
  })

  it('shows a refused connect in the words Casebook sent', async () => {
    const message = "Casebook could not read Codex's settings safely, so it changed nothing. Use the steps under Technical details instead."
    Object.assign(mocks.api, mockApi({ connectClient: rejectWith({ kind: 'conflict', field: 'settings', message }) }))
    const user = userEvent.setup()
    renderDialog()
    await user.click(await screen.findByRole('button', { name: `${CONNECT_ACTION}: Codex` }))
    expect(await within(screen.getByRole('region', { name: 'Codex' })).findByRole('alert')).toHaveTextContent(message)
  })

  it('keeps the lines to run by hand under one Technical details disclosure', async () => {
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
      inside.getByText(/Reminders added: SessionStart, Stop, PreCompact/),
      inside.getByText(OTHER_MCP_COPY),
    ]
    for (const el of hidden) expect(el).not.toBeVisible()
    // Outside the disclosure there is no line to run and no technical word.
    const outside = document.querySelector('.dialog-body')!.cloneNode(true) as HTMLElement
    outside.querySelector('details')!.remove()
    expect(outside.querySelector('code')).toBeNull()
    expect(outside).not.toHaveTextContent(/PowerShell|Command Prompt|WSL|--hooks|--skill|MCP/)

    await user.click(inside.getByText('Technical details'))
    for (const el of hidden) expect(el).toBeVisible()
    await user.click(inside.getByRole('button', { name: 'Copy the line for Claude Code' }))
    expect(mocks.api.copyText).toHaveBeenLastCalledWith(`& '${PF}' setup claude --hooks --skill`)
    await user.click(inside.getByRole('button', { name: 'Copy the line for Claude Code in Command Prompt' }))
    expect(mocks.api.copyText).toHaveBeenLastCalledWith(`"${PF}" setup claude --hooks --skill`)
    await user.click(inside.getByRole('button', { name: 'Copy the line for Codex in WSL' }))
    expect(mocks.api.copyText).toHaveBeenLastCalledWith(`codex mcp add casebook -- ${CLI_WSL_PATH} mcp`)
    await user.click(inside.getByRole('button', { name: 'Copy settings for Claude Code' }))
    expect(mocks.api.copyText).toHaveBeenLastCalledWith(clientConnections.claude.snippet)
  })

  it('says the Terminal app off Windows and offers no WSL line', async () => {
    Object.assign(mocks.api, mockApi({ clientConnections: async () => ({
      ...clientConnections, shell: 'terminal',
      claude: { ...clientConnections.claude, setup_command_wsl: '' },
      codex: { ...clientConnections.codex, setup_command_wsl: '' },
    }) }))
    renderDialog()
    await screen.findByRole('region', { name: 'Claude Code' })
    expect(screen.getAllByText(/Paste this line into the Terminal app/).length).toBeGreaterThan(0)
    expect(screen.queryByText(/WSL/)).not.toBeInTheDocument()
  })

  it('never mentions the source code or a build script, and cannot connect without the helper program', async () => {
    Object.assign(mocks.api, mockApi({ clientConnections: async () => ({ ...withCmd, cli_path: '', cli_source: 'missing', cli_name: 'casebook' }) }))
    renderDialog()
    const dialog = await screen.findByRole('dialog', { name: 'Connect your AI' })
    await screen.findByRole('region', { name: 'Claude Code' })
    expect(dialog).not.toHaveTextContent(/source code/i)
    expect(dialog).not.toHaveTextContent('scripts/build-cli.sh')
    expect(screen.getByRole('button', { name: `${CONNECT_ACTION}: Codex` })).toBeDisabled()
  })

  it('copies a business question prompt and leads from the finding to checking and sharing', async () => {
    const user = userEvent.setup()
    renderDialog()
    const finding = await screen.findByRole('region', { name: 'Your first finding' })
    await user.click(within(finding).getByRole('button', { name: 'Copy this message' }))
    expect(mocks.api.copyText).toHaveBeenCalledWith(FIRST_FINDING_PROMPT)
    expect(FIRST_FINDING_PROMPT).toMatch(/why calls to our call center went up when sales stayed the same/)
    expect(FIRST_FINDING_PROMPT).toMatch(/with the source it came from/)
    expect(within(finding).getByText(/Check the next source/)).toHaveTextContent(/Share the answer/)
  })

  it('reports a clipboard failure', async () => {
    Object.assign(mocks.api, { copyText: vi.fn(async () => Promise.reject(new Error('no clipboard'))) })
    const user = userEvent.setup()
    renderDialog()
    const finding = await screen.findByRole('region', { name: 'Your first finding' })
    await user.click(within(finding).getByRole('button', { name: 'Copy this message' }))
    expect(await screen.findByRole('alert')).toBeInTheDocument()
  })

  it('explains a missing helper program', async () => {
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

  it('changes no settings until a button asks: Look again only reads', async () => {
    const user = userEvent.setup()
    const onClose = renderDialog()
    await screen.findByRole('region', { name: 'Claude Code' })
    await user.click(screen.getByRole('button', { name: 'Look again' }))
    await waitFor(() => expect(mocks.api.clientConnections).toHaveBeenCalledTimes(2))
    await user.click(screen.getByRole('button', { name: 'Done' }))
    expect(onClose).toHaveBeenCalled()
    const called = Object.entries(mocks.api)
      .filter(([, fn]) => (fn as unknown as { mock?: { calls: unknown[] } }).mock?.calls.length)
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
