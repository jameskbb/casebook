// Connect AI clients panel fixtures (desktop/clients.go shapes).
import type { ClientConnections } from '../api/types'

export const CLI_PATH = 'C:\\Users\\Ada\\AppData\\Local\\Programs\\Casebook\\bin\\casebook.exe'
/** CLI_PATH as WSL runs it. */
export const CLI_WSL_PATH = '/mnt/c/Users/Ada/AppData/Local/Programs/Casebook/bin/casebook.exe'

export const clientConnections: ClientConnections = {
  cli_path: CLI_PATH,
  cli_source: 'bundled',
  cli_name: CLI_PATH,
  db_path: '',
  shell: 'PowerShell',
  claude: {
    configured: true,
    matches_cli: true,
    command: CLI_PATH,
    command_exists: true,
    config_path: 'C:\\Users\\Ada\\.claude.json',
    config_problem: false,
    hooks: ['SessionStart', 'Stop', 'PreCompact'],
    setup_command: `${CLI_PATH} setup claude --hooks --skill`,
    setup_command_cmd: '',
    setup_command_wsl: `claude mcp add --scope user casebook -- ${CLI_WSL_PATH} mcp`,
    snippet:
      'Claude Code MCP server (user scope): merge into the top-level "mcpServers" object of C:\\Users\\Ada\\.claude.json\n{\n  "mcpServers": {}\n}\n',
  },
  codex: {
    configured: false,
    matches_cli: false,
    command: '',
    command_exists: false,
    config_path: 'C:\\Users\\Ada\\.codex\\config.toml',
    config_problem: false,
    hooks: [],
    setup_command: `${CLI_PATH} setup codex --skill`,
    setup_command_cmd: '',
    setup_command_wsl: `codex mcp add casebook -- ${CLI_WSL_PATH} mcp`,
    snippet: '# Codex MCP server: add this block to C:\\Users\\Ada\\.codex\\config.toml\n[mcp_servers.casebook]\n',
  },
}
