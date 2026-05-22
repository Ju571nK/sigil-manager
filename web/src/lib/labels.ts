/** Maps an AI-tool wire string (contract §14.5/§14.7) to a display name. */
export function humanTool(tool: string): string {
  switch (tool) {
    case 'claude_code':
      return 'Claude Code';
    case 'claude_desktop':
      return 'Claude Desktop';
    case 'continue_dev':
      return 'Continue.dev';
    case 'codex':
      return 'Codex';
    case 'gemini':
      return 'Gemini';
    case 'cursor':
      return 'Cursor';
    default:
      return tool;
  }
}
