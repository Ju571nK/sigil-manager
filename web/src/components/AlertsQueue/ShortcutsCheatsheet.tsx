import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

/** Single source of truth for what `?` shows — mirrored from useShortcuts. */
const SHORTCUTS: Array<[label: string, keys: string]> = [
  ['Move selection down', 'j  /  ↓'],
  ['Move selection up', 'k  /  ↑'],
  ['Open selected alert', 'Enter'],
  ['Close slide-over', 'Esc'],
  ['Acknowledge', 'c'],
  ['Resolve', 'r'],
  ['Set Investigating', 'i'],
  ['Assign (focus field)', 'a'],
  ['Add note', 'n'],
  ['Focus search', '/'],
  ['Go to Alerts', 'g a'],
  ['Go to Fleet  (Plan 03)', 'g f'],
  ['Go to Settings  (Plan 05)', 'g s'],
  ['Show shortcuts', '?'],
];

export function ShortcutsCheatsheet({ open, onOpenChange }: Props) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>Keyboard shortcuts</DialogTitle>
          <DialogDescription>
            UI/UX §7.1 bindings, always available except inside text inputs.
          </DialogDescription>
        </DialogHeader>
        <ul className="mt-2 divide-y divide-border-subtle text-sm">
          {SHORTCUTS.map(([label, keys]) => (
            <li key={label} className="flex items-center justify-between py-1.5">
              <span className="text-text-body">{label}</span>
              <kbd className="rounded border border-border bg-bg-elevated px-1.5 py-0.5 font-mono text-[11px] text-text-primary">
                {keys}
              </kbd>
            </li>
          ))}
        </ul>
      </DialogContent>
    </Dialog>
  );
}
