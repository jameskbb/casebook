import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import type { ClientConnection } from '../../api/types'
import { AppDataProvider } from '../../state/appData'
import { CLI_PATH, CLI_WSL_PATH, clientConnections } from '../../test/fixtures.clients'
import { mockApi } from '../../test/mockApi'
import { DataOpsProvider } from '../sidebar/DataOps'
import { SidebarFooter } from '../sidebar/SidebarFooter'
import { ToastProvider } from '../ui/Toaster'
import {
  CLAUDE_AFTER_COPY,
  CLI_MISSING_COPY,
  CLI_ON_PATH_COPY,
  clientStatus,
  CODEX_AFTER_COPY,
  ConnectClientsDialog,
  FIRST_FINDING_PROMPT,
  OTHER_MCP_COPY,
} from './ConnectClientsDialog'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../../api/api', () => ({ api: mocks.api }))

function renderDialog(onClose = vi.fn()) {
  render(<ToastProvider><ConnectClientsDialog onClose={onClose} /></ToastProvider>)
  return onClose
}

const base: ClientConnection = clientConnections.codex

describe('clientStatus', () => {
  it('names each state', () => {
    expect(clientStatus(base, true)).toEqual({ tone: 'off', label: 'Not connected' })
    expect(clientStatus({ ...base, config_problem: true }, true).label).toBe("Settings file can't be read")
    const on = { ...base, configured: true, command_exists: true, matches_cli: true }
    expect(clientStatus(on, true)).toEqual({ tone: 'ok', label: 'Connected' })
    expect(clientStatus({ ...on, matches_cli: false }, true).label).toBe('Uses a different command-line tool')
    expect(clientStatus({ ...on, matches_cli: false }, false).label).toBe('Connected')
    expect(clientStatus({ ...on, command_exists: false }, true).tone).toBe('warn')
  })
})

describe('ConnectClientsDialog', () => {
  beforeEach(() => { Object.assign(mocks.api, mockApi()) })

  it('shows one client command at a time and switches client instructions', async () => {
    const user = userEvent.setup()
    renderDialog()
    const claude = await screen.findByRole('region', { name: 'Claude Code' })
    expect(within(claude).getByText('Connected')).toBeInTheDocument()
    expect(within(claude).getByText(`${CLI_PATH} setup claude --hooks --skill`)).toBeInTheDocument()
    expect(within(claude).getByText(CLAUDE_AFTER_COPY)).toBeInTheDocument()
    expect(screen.queryByRole('region', { name: 'Codex' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: /for WSL/ })).not.toBeVisible()
    await user.click(screen.getByRole('radio', { name: 'Codex' }))
    const codex = screen.getByRole('region', { name: 'Codex' })
    expect(within(codex).getByText('Not connected')).toBeInTheDocument()
    expect(within(codex).getByText(CODEX_AFTER_COPY)).toBeInTheDocument()
    await user.click(within(codex).getByRole('button', { name: 'Copy the Codex setup command' }))
    expect(mocks.api.copyText).toHaveBeenCalledWith(`${CLI_PATH} setup codex --skill`)
    expect(await screen.findByText('Copied to the clipboard.')).toBeInTheDocument()
  })

  it('names PowerShell and discloses the Command Prompt form for a quoted path (AR-12)', async () => {
    const pf = 'C:\\Program Files\\Casebook\\bin\\casebook.exe'
    Object.assign(mocks.api, mockApi({ clientConnections: async () => ({
      ...clientConnections,
      claude: { ...clientConnections.claude, setup_command: `& '${pf}' setup claude --hooks --skill`, setup_command_cmd: `"${pf}" setup claude --hooks --skill` },
    }) }))
    const user = userEvent.setup()
    renderDialog()
    const claude = await screen.findByRole('region', { name: 'Claude Code' })
    expect(within(claude).getByText('Run in PowerShell')).toBeInTheDocument()
    expect(within(claude).getByText(`& '${pf}' setup claude --hooks --skill`)).toBeInTheDocument()
    expect(within(claude).getByRole('button', { name: /for Command Prompt/ })).not.toBeVisible()
    await user.click(within(claude).getByText('Other shells and manual configuration'))
    await user.click(within(claude).getByRole('button', { name: 'Copy the Claude Code setup command for Command Prompt' }))
    expect(mocks.api.copyText).toHaveBeenCalledWith(`"${pf}" setup claude --hooks --skill`)
    await user.click(screen.getByRole('radio', { name: 'Codex' }))
    expect(screen.queryByText('Or in Command Prompt')).not.toBeInTheDocument()
  })

  it('discloses the WSL (bash) form for each client on Windows', async () => {
    const spaced = "'/mnt/c/Users/Jo Doe/AppData/Local/Programs/Casebook/bin/casebook.exe'"
    Object.assign(mocks.api, mockApi({ clientConnections: async () => ({
      ...clientConnections,
      claude: { ...clientConnections.claude, setup_command_wsl: `claude mcp add --scope user casebook -- ${spaced} mcp` },
    }) }))
    const user = userEvent.setup()
    renderDialog()
    const claude = await screen.findByRole('region', { name: 'Claude Code' })
    await user.click(within(claude).getByText('Other shells and manual configuration'))
    expect(within(claude).getByText('Or in WSL (bash), for Claude Code installed inside WSL')).toBeInTheDocument()
    await user.click(within(claude).getByRole('button', { name: 'Copy the Claude Code setup command for WSL' }))
    expect(mocks.api.copyText).toHaveBeenCalledWith(`claude mcp add --scope user casebook -- ${spaced} mcp`)
    await user.click(screen.getByRole('radio', { name: 'Codex' }))
    const codex = screen.getByRole('region', { name: 'Codex' })
    await user.click(within(codex).getByText('Other shells and manual configuration'))
    await user.click(within(codex).getByRole('button', { name: 'Copy the Codex setup command for WSL' }))
    expect(mocks.api.copyText).toHaveBeenLastCalledWith(`codex mcp add casebook -- ${CLI_WSL_PATH} mcp`)
  })

  it('says Run in a terminal and omits WSL options off Windows', async () => {
    Object.assign(mocks.api, mockApi({ clientConnections: async () => ({
      ...clientConnections, shell: 'terminal',
      claude: { ...clientConnections.claude, setup_command_wsl: '' },
      codex: { ...clientConnections.codex, setup_command_wsl: '' },
    }) }))
    const user = userEvent.setup()
    renderDialog()
    const claude = await screen.findByRole('region', { name: 'Claude Code' })
    expect(within(claude).getByText('Run in a terminal')).toBeInTheDocument()
    await user.click(within(claude).getByText('Other shells and manual configuration'))
    expect(screen.queryByText(/Or in WSL/)).not.toBeInTheDocument()
  })

  it('copies the manual configuration and explains the hooks in diagnostics', async () => {
    const user = userEvent.setup()
    renderDialog()
    const claude = await screen.findByRole('region', { name: 'Claude Code' })
    await user.click(within(claude).getByText('Other shells and manual configuration'))
    await user.click(within(claude).getByRole('button', { name: 'Copy configuration' }))
    expect(mocks.api.copyText).toHaveBeenCalledWith(clientConnections.claude.snippet)
    await user.click(within(claude).getByText('Claude Code settings and diagnostics'))
    expect(within(claude).getByText(/Session hooks: SessionStart, Stop, PreCompact/)).toBeVisible()
    expect(within(claude).getByText(/--skill adds the \/casebook skill/)).toBeVisible()
    expect(within(claude).getByText(/This checks local settings/)).toBeVisible()
  })

  it('gives other MCP clients their own instructions', async () => {
    const user = userEvent.setup()
    renderDialog()
    await screen.findByRole('region', { name: 'Claude Code' })
    await user.click(screen.getByRole('radio', { name: 'Other MCP client' }))
    expect(within(screen.getByRole('region', { name: 'Other MCP clients' })).getByText(OTHER_MCP_COPY)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Copy the .* setup command$/ })).not.toBeInTheDocument()
  })

  it('copies a business question prompt and leads from the finding to review and sharing', async () => {
    const user = userEvent.setup()
    renderDialog()
    const finding = await screen.findByRole('region', { name: 'Your first finding' })
    await user.click(within(finding).getByRole('button', { name: 'Copy first finding prompt' }))
    expect(mocks.api.copyText).toHaveBeenCalledWith(FIRST_FINDING_PROMPT)
    expect(FIRST_FINDING_PROMPT).toMatch(/First ask me which period, region and source data to use/)
    expect(FIRST_FINDING_PROMPT).toMatch(/supporting evidence and source details/)
    expect(within(finding).getByText(/Choose Verify myself only after you have checked it/)).toHaveTextContent(/Share summary/)
  })

  it('reports a clipboard failure', async () => {
    Object.assign(mocks.api, { copyText: vi.fn(async () => Promise.reject(new Error('no clipboard'))) })
    const user = userEvent.setup()
    renderDialog()
    await user.click(await screen.findByRole('button', { name: 'Copy the Claude Code setup command' }))
    expect(await screen.findByRole('alert')).toBeInTheDocument()
  })

  it('explains a missing CLI before setup', async () => {
    Object.assign(mocks.api, { clientConnections: vi.fn(async () => ({ ...clientConnections, cli_path: '', cli_source: 'missing', cli_name: 'casebook' })) })
    renderDialog()
    expect(await screen.findByText(CLI_MISSING_COPY)).toBeVisible()
  })

  it('discloses the CLI path, a development CLI from PATH and a non-default database', async () => {
    Object.assign(mocks.api, { clientConnections: vi.fn(async () => ({ ...clientConnections, cli_source: 'path', db_path: 'D:\\cases\\casebook.db' })) })
    const user = userEvent.setup()
    renderDialog()
    await screen.findByRole('region', { name: 'Claude Code' })
    expect(screen.getByRole('region', { name: 'Command-line tool' })).not.toBeVisible()
    await user.click(screen.getByText('Command-line tool and data file'))
    const cli = screen.getByRole('region', { name: 'Command-line tool' })
    expect(within(cli).getByText(CLI_ON_PATH_COPY)).toBeVisible()
    expect(within(cli).getByText(/This window uses D:\\cases\\casebook.db/)).toBeVisible()
    await user.click(within(cli).getByRole('button', { name: 'Copy the command-line tool path' }))
    expect(mocks.api.copyText).toHaveBeenCalledWith(CLI_PATH)
  })

  it('never writes client settings: it only reads and copies', async () => {
    const user = userEvent.setup()
    const onClose = renderDialog()
    await screen.findByRole('region', { name: 'Claude Code' })
    await user.click(screen.getByRole('button', { name: 'Check again' }))
    await waitFor(() => expect(mocks.api.clientConnections).toHaveBeenCalledTimes(2))
    await user.click(screen.getByRole('button', { name: 'Done' }))
    expect(onClose).toHaveBeenCalled()
    const called = Object.entries(mocks.api)
      .filter(([, fn]) => (fn as unknown as { mock: { calls: unknown[] } }).mock.calls.length > 0)
      .map(([name]) => name)
    expect(called.sort()).toEqual(['clientConnections'])
  })

  it('shows a readable error and retries when local status cannot be read', async () => {
    Object.assign(mocks.api, { clientConnections: vi.fn()
      .mockRejectedValueOnce(new Error(JSON.stringify({ kind: 'other', message: 'Casebook cannot find your home folder' })))
      .mockResolvedValue(clientConnections) })
    const user = userEvent.setup()
    renderDialog()
    expect(await screen.findByRole('alert')).toHaveTextContent(/home folder/)
    await user.click(screen.getByRole('button', { name: 'Check again' }))
    expect(await screen.findByRole('region', { name: 'Claude Code' })).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })
})

describe('Data menu entry', () => {
  beforeEach(() => { Object.assign(mocks.api, mockApi()) })

  it('opens the panel from the Data menu', async () => {
    const user = userEvent.setup()
    render(<ToastProvider><AppDataProvider><DataOpsProvider><SidebarFooter /></DataOpsProvider></AppDataProvider></ToastProvider>)
    await user.click(screen.getByRole('button', { name: 'Data' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Connect AI clients...' }))
    expect(await screen.findByRole('dialog', { name: 'Connect AI clients' })).toBeInTheDocument()
    await screen.findByRole('region', { name: 'Claude Code' })
  })
})
