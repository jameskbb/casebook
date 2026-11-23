// The AI tools Casebook can connect itself to, and what their settings say.
import type { ClientConnection, ClientConnections, ClientTool } from '../api/types'

export interface ToolName {
  tool: ClientTool
  /** As a heading: "Claude desktop app". */
  title: string
  /** Inside a sentence: "the Claude desktop app". */
  name: string
}

export const CLIENT_TOOLS: ToolName[] = [
  { tool: 'claude_desktop', title: 'Claude desktop app', name: 'the Claude desktop app' },
  { tool: 'claude', title: 'Claude Code', name: 'Claude Code' },
  { tool: 'codex', title: 'Codex', name: 'Codex' },
]

/** The tool's settings point at this copy of Casebook. */
export function isConnected(c: ClientConnection | undefined): boolean {
  return Boolean(c && c.configured && c.matches_cli && !c.config_problem)
}

export function anyClientConnected(all: ClientConnections): boolean {
  return CLIENT_TOOLS.some(({ tool }) => isConnected(all[tool]))
}
