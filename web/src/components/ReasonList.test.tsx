import '@testing-library/jest-dom/vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ReasonList } from './ReasonList';

afterEach(cleanup);
const reasons = [
  {
    kind: 'mcp_tool_instruction_override',
    server: 'server-a',
    tool: 'tool-a',
    pattern: 'ignore_previous',
  },
  { kind: 'mcp_tool_hidden_text', server: 'server-b', tool: 'tool-b', text_kind: 'zero_width' },
  { kind: 'mcp_tool_name_shadow', tool: 'shared', servers: ['server-a', 'server-b'] },
  {
    kind: 'mcp_tool_surface_drift',
    server: 'server-c',
    tool: 'tool-c',
    baseline_hash: 'oldhash',
    current_hash: 'newhash',
  },
  {
    kind: 'mcp_unapproved_new_tool',
    server: 'server-d',
    tool: 'tool-d',
    current_hash: 'addedhash',
  },
  { kind: 'mcp_schema_privilege_expansion', server: 'server-e', tool: 'tool-e' },
  { kind: 'mcp_read_only_hint_contradiction', server: 'server-f', tool: 'tool-f' },
  { kind: 'auto_mode_defaults_dropped', list: 'hard_deny' },
  { kind: 'hook_forwards_tool_calls', hook_event: 'PreToolUse', destination: 'external.example' },
  { kind: 'unattended_loop_prompt', source: 'project' },
  { kind: 'standing_command_approval', pattern: 'git push' },
];
describe('reason evidence', () => {
  it('renders the eleven additive reason payloads and advisory context', () => {
    render(<ReasonList reasons={reasons} />);
    for (const value of [
      'ignore_previous',
      'zero_width',
      'oldhash',
      'newhash',
      'addedhash',
      'tool-e',
      'tool-f',
      'hard_deny',
      'external.example',
      'PreToolUse',
      'project',
      'git push',
    ])
      expect(screen.getByText(value)).toBeInTheDocument();
    expect(screen.getByText('["server-a","server-b"]')).toBeInTheDocument();
    expect(screen.getByText(/not proof of a write/)).toBeInTheDocument();
    expect(screen.getByText(/does not establish an approval violation/)).toBeInTheDocument();
  });
  it('keeps distinct and identical repeated reasons without React key collisions', () => {
    const errors = vi.spyOn(console, 'error').mockImplementation(() => {});
    render(
      <ReasonList reasons={[reasons[1], { ...reasons[1], server: 'other-server' }, reasons[1]]} />,
    );
    expect(screen.getAllByRole('listitem')).toHaveLength(3);
    expect(errors).not.toHaveBeenCalled();
    errors.mockRestore();
  });
  it('shows script_path, future fields and full long evidence without interpreting markup', () => {
    const path = `/work/${'nested/'.repeat(25)}script.sh`;
    render(
      <ReasonList
        reasons={[
          {
            kind: 'external_script_unscanned',
            script_path: path,
            hook_event: 'Stop',
            future_value: false,
            source_chain: ['/entry.sh', path],
            snippet: '<script>alert(1)</script>',
          },
        ]}
      />,
    );
    fireEvent.click(screen.getAllByText(/full value/)[0]);
    expect(screen.getByText(path)).toBeVisible();
    expect(screen.getByText('false')).toBeInTheDocument();
    expect(screen.getByText('<script>alert(1)</script>')).toBeInTheDocument();
  });
});
