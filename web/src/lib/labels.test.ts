import { describe, expect, it } from 'vitest';
import { humanKind, humanTool, scopeLabel, shortPath } from './labels';

describe('humanTool', () => {
  it('maps the six known tool wire strings to display names', () => {
    expect(humanTool('claude_code')).toBe('Claude Code');
    expect(humanTool('claude_desktop')).toBe('Claude Desktop');
    expect(humanTool('continue_dev')).toBe('Continue.dev');
    expect(humanTool('codex')).toBe('Codex');
    expect(humanTool('gemini')).toBe('Gemini');
    expect(humanTool('cursor')).toBe('Cursor');
  });

  it('maps the post-0.5.0 tool wire strings (contract §14.9.1)', () => {
    expect(humanTool('antigravity')).toBe('Antigravity');
    expect(humanTool('grok')).toBe('Grok');
    expect(humanTool('other')).toBe('Other');
  });

  it('prefers tool_label over "Other" when tool is "other" and a label is present', () => {
    expect(humanTool('other', 'Acme Agent')).toBe('Acme Agent');
  });

  it('ignores tool_label for non-"other" tools (built-in mapping wins)', () => {
    expect(humanTool('claude_code', 'Acme Agent')).toBe('Claude Code');
  });

  it('falls back to "Other" when tool is "other" with an empty/absent label', () => {
    expect(humanTool('other', '')).toBe('Other');
    expect(humanTool('other', null)).toBe('Other');
    expect(humanTool('other', undefined)).toBe('Other');
  });

  it('falls back to the raw string for unknown tools', () => {
    expect(humanTool('future_tool')).toBe('future_tool');
  });
});

describe('humanKind', () => {
  it('title-cases multi-word snake_case strings', () => {
    expect(humanKind('policy_reloaded')).toBe('Policy Reloaded');
    expect(humanKind('tls_failure')).toBe('Tls Failure');
  });

  it('title-cases a single word', () => {
    expect(humanKind('heartbeat')).toBe('Heartbeat');
  });

  it('returns empty string unchanged', () => {
    expect(humanKind('')).toBe('');
  });

  it('preserves empty segments from leading/double underscores', () => {
    expect(humanKind('a__b')).toBe('A  B');
  });
});

describe('shortPath', () => {
  it('returns short paths (≤2 non-empty segments) unchanged', () => {
    expect(shortPath('etc/hosts')).toBe('etc/hosts');
    expect(shortPath('/etc/hosts')).toBe('/etc/hosts');
    expect(shortPath('foo')).toBe('foo');
  });

  it('truncates long paths to "…/" plus the last two segments', () => {
    expect(shortPath('/home/user/project/src/index.ts')).toBe('…/src/index.ts');
    expect(shortPath('a/b/c')).toBe('…/b/c');
  });
});

describe('scopeLabel', () => {
  it('humanizes user_global (single shared rendering across surfaces)', () => {
    expect(scopeLabel({ kind: 'user_global' })).toBe('user global');
  });
  it('shortens project paths', () => {
    expect(scopeLabel({ kind: 'project', path: '/home/u/proj/src' })).toBe('project · …/proj/src');
  });
  it('labels application scope by app name', () => {
    expect(scopeLabel({ kind: 'application', app: 'Slack' })).toBe('app · Slack');
  });
});
