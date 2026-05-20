import { useEffect, useRef } from 'react';

/**
 * Global keyboard shortcut binder.
 *
 * Each handler is keyed by the binding string. Single-key bindings are
 * matched directly; the `'g'`-leader pattern (e.g. `g a`) buffers the
 * `g` press for up to 1s and matches the next non-modifier key.
 *
 * Bindings are ignored while focus is inside `<input>`, `<textarea>`,
 * `[contenteditable]`, or `[role="textbox"]` — except for `Escape`,
 * which always fires so the user can dismiss the slide-over without
 * leaving the input.
 */
export type ShortcutHandlers = Partial<{
  // Navigation between queue rows.
  ArrowDown: (e: KeyboardEvent) => void;
  ArrowUp: (e: KeyboardEvent) => void;
  j: (e: KeyboardEvent) => void;
  k: (e: KeyboardEvent) => void;
  Enter: (e: KeyboardEvent) => void;
  Escape: (e: KeyboardEvent) => void;

  // Triage actions on the focused alert.
  a: (e: KeyboardEvent) => void; // assign
  c: (e: KeyboardEvent) => void; // acknowledge
  r: (e: KeyboardEvent) => void; // resolve
  i: (e: KeyboardEvent) => void; // investigating
  n: (e: KeyboardEvent) => void; // new note

  // Focus search input.
  '/': (e: KeyboardEvent) => void;

  // Vim-style leader navigation: g a / g f / g s.
  'g a': (e: KeyboardEvent) => void;
  'g f': (e: KeyboardEvent) => void;
  'g s': (e: KeyboardEvent) => void;

  // Cheatsheet.
  '?': (e: KeyboardEvent) => void;
}>;

const LEADER_TIMEOUT_MS = 1_000;

export function useShortcuts(handlers: ShortcutHandlers) {
  // Keep the handlers in a ref so we don't reattach the global listener on
  // every render — the keydown handler reads `current` lazily.
  const ref = useRef(handlers);
  ref.current = handlers;

  useEffect(() => {
    let leaderActive = false;
    let leaderTimer: number | undefined;
    const clearLeader = () => {
      leaderActive = false;
      if (leaderTimer !== undefined) {
        window.clearTimeout(leaderTimer);
        leaderTimer = undefined;
      }
    };

    const handler = (e: KeyboardEvent) => {
      // Escape always fires.
      if (e.key === 'Escape') {
        const fn = ref.current.Escape;
        if (fn) {
          fn(e);
          e.preventDefault();
        }
        clearLeader();
        return;
      }
      if (isTypingTarget(e.target)) return;
      // Ignore modifier-augmented presses so the page's standard
      // browser shortcuts (⌘R reload, ⌘F find) keep working.
      if (e.ctrlKey || e.metaKey || e.altKey) return;

      // Leader handling.
      if (leaderActive) {
        const combo = `g ${e.key}` as keyof ShortcutHandlers;
        const fn = ref.current[combo];
        clearLeader();
        if (typeof fn === 'function') {
          (fn as (ev: KeyboardEvent) => void)(e);
          e.preventDefault();
          return;
        }
        // No match — drop the leader and let the key fall through to the
        // single-key branch below as a courtesy.
      }
      if (e.key === 'g') {
        leaderActive = true;
        leaderTimer = window.setTimeout(clearLeader, LEADER_TIMEOUT_MS);
        return;
      }

      const fn = ref.current[e.key as keyof ShortcutHandlers];
      if (typeof fn === 'function') {
        (fn as (ev: KeyboardEvent) => void)(e);
        e.preventDefault();
      }
    };

    window.addEventListener('keydown', handler);
    return () => {
      window.removeEventListener('keydown', handler);
      clearLeader();
    };
  }, []);
}

function isTypingTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  const tag = target.tagName;
  if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return true;
  if (target.isContentEditable) return true;
  if (target.getAttribute('role') === 'textbox') return true;
  return false;
}
