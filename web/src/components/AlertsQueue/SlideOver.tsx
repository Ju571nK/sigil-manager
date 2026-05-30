import { formatDistanceToNowStrict } from 'date-fns';
import { Check, Loader2, Search } from 'lucide-react';
import { type FormEvent, useEffect, useRef, useState } from 'react';
import { type EventWithTriage, extractAiGuard } from '@/api/fleet';
import type { TriageStatus } from '@/api/triage';
import { ReasonList } from '@/components/ReasonList';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet';
import { useAppendNote, useTriageDetailOrNull, useUpsertTriage } from '@/hooks/useTriage';
import { humanKind, humanTool, scopeLabel } from '@/lib/labels';
import { cn } from '@/lib/utils';

interface Props {
  /** The event being inspected, or null when nothing is selected. */
  event: EventWithTriage | null;
  /** Closes the slide-over (e.g. by clearing the URL alert= param). */
  onClose: () => void;
  /** Imperative handle the parent can use to focus the assign input. */
  registerFocusAssign: (fn: () => void) => void;
  registerFocusNote: (fn: () => void) => void;
}

/**
 * Right-side slide-over per UI/UX §5.1: 40% viewport, opens via the URL
 * `?alert=:event_id` param. Surfaces alert body + triage state + action
 * buttons. Every action goes through useTriage, which invalidates the
 * Alerts queue cache so the row pill updates immediately.
 */
export function SlideOver({ event, onClose, registerFocusAssign, registerFocusNote }: Props) {
  const open = event !== null;
  const assignInputRef = useRef<HTMLInputElement | null>(null);
  const noteInputRef = useRef<HTMLTextAreaElement | null>(null);

  useEffect(() => {
    registerFocusAssign(() => assignInputRef.current?.focus());
    registerFocusNote(() => noteInputRef.current?.focus());
  }, [registerFocusAssign, registerFocusNote]);

  return (
    <Sheet
      open={open}
      onOpenChange={(next) => {
        if (!next) onClose();
      }}
    >
      <SheetContent
        side="right"
        className="w-[40vw] min-w-[420px] max-w-[640px] bg-bg-surface p-0 border-l border-border"
      >
        {event ? (
          <SlideOverBody
            event={event}
            assignInputRef={assignInputRef}
            noteInputRef={noteInputRef}
          />
        ) : (
          <SheetHeader className="p-4">
            <SheetTitle>No alert selected</SheetTitle>
            <SheetDescription>Open one from the queue.</SheetDescription>
          </SheetHeader>
        )}
      </SheetContent>
    </Sheet>
  );
}

function SlideOverBody({
  event,
  assignInputRef,
  noteInputRef,
}: {
  event: EventWithTriage;
  assignInputRef: React.MutableRefObject<HTMLInputElement | null>;
  noteInputRef: React.MutableRefObject<HTMLTextAreaElement | null>;
}) {
  const ag = extractAiGuard(event);
  const triage = useTriageDetailOrNull(event.host_id, event.event_id);
  const upsert = useUpsertTriage();
  const appendNote = useAppendNote();

  const [assigneeDraft, setAssigneeDraft] = useState<string>('');
  const [noteDraft, setNoteDraft] = useState<string>('');

  // Keep the assignee input in sync with what's on the server, but only
  // when the user isn't actively editing it (we don't clobber their typing).
  useEffect(() => {
    if (document.activeElement !== assignInputRef.current) {
      setAssigneeDraft(triage.data?.row.assignee ?? '');
    }
  }, [triage.data?.row.assignee, assignInputRef]);

  const ensureSnapshot = (): unknown => {
    // Server only honors evidence_snapshot on insert; sending the event's
    // raw evidence payload makes the row survive sigil-server JSONL
    // retention pruning (contract §13).
    return event.evidence;
  };

  const setStatus = (status: TriageStatus) => {
    upsert.mutate({
      host_id: event.host_id,
      event_id: event.event_id,
      status,
      evidence_snapshot: triage.data ? undefined : ensureSnapshot(),
    });
  };

  const onAssign = (e: FormEvent) => {
    e.preventDefault();
    const trimmed = assigneeDraft.trim();
    upsert.mutate({
      host_id: event.host_id,
      event_id: event.event_id,
      assignee: trimmed === '' ? undefined : trimmed,
      clear_assignee: trimmed === '',
      evidence_snapshot: triage.data ? undefined : ensureSnapshot(),
    });
  };

  const onSubmitNote = (e: FormEvent) => {
    e.preventDefault();
    const body = noteDraft.trim();
    if (!body) return;
    // The note endpoint requires the triage row to exist; create it first
    // when this is an un-actioned alert so the FK is satisfied.
    if (!triage.data) {
      upsert.mutate(
        {
          host_id: event.host_id,
          event_id: event.event_id,
          status: 'open',
          evidence_snapshot: ensureSnapshot(),
        },
        {
          onSuccess: () => {
            appendNote.mutate(
              { host_id: event.host_id, event_id: event.event_id, body },
              { onSuccess: () => setNoteDraft('') },
            );
          },
        },
      );
      return;
    }
    appendNote.mutate(
      { host_id: event.host_id, event_id: event.event_id, body },
      { onSuccess: () => setNoteDraft('') },
    );
  };

  const status = triage.status ?? 'open';
  const bucket = ag?.bucket ?? severityLabel(event.severity);

  return (
    <div className="flex h-full flex-col">
      {/* Header — title + close baked into Sheet (top-right ✕). */}
      <div className="border-b border-border-subtle px-5 py-3">
        <SheetHeader className="p-0">
          <SheetTitle className="text-base font-semibold text-text-primary">
            {ag
              ? `AI Guard risk · ${humanTool(ag.tool)}`
              : humanKind(event.evidence?.kind ?? 'unknown')}
          </SheetTitle>
          <SheetDescription className="text-xs text-text-muted">
            event_id <code className="font-mono">{event.event_id}</code>
          </SheetDescription>
        </SheetHeader>
        <div className="mt-2 flex flex-wrap items-center gap-1.5 text-[11px] uppercase tracking-wide">
          <Badge tone="severity">{bucket}</Badge>
          <Badge tone="status">{status}</Badge>
          {ag?.scope?.kind && <Badge tone="muted">{scopeLabel(ag.scope)}</Badge>}
        </div>
      </div>

      {/* Body — scrollable. */}
      <div className="flex-1 overflow-y-auto px-5 py-4 space-y-5 text-sm">
        {triage.isPending && (
          <div aria-hidden className="space-y-2">
            <div className="h-2 w-2/3 animate-pulse rounded bg-bg-elevated" />
            <div className="h-2 w-1/2 animate-pulse rounded bg-bg-elevated" />
            <div className="h-2 w-3/4 animate-pulse rounded bg-bg-elevated" />
          </div>
        )}
        <FactGrid event={event} ag={ag} />

        {/* Actions */}
        <section>
          <h3 className="mb-2 text-xs uppercase tracking-wide text-text-subtle">Actions</h3>
          <div className="flex flex-wrap gap-2">
            <ActionButton
              active={status === 'acknowledged'}
              disabled={upsert.isPending}
              onClick={() => setStatus('acknowledged')}
            >
              Acknowledge
            </ActionButton>
            <ActionButton
              active={status === 'investigating'}
              disabled={upsert.isPending}
              onClick={() => setStatus('investigating')}
            >
              Investigating
            </ActionButton>
            <ActionButton
              active={status === 'resolved'}
              disabled={upsert.isPending}
              onClick={() => setStatus('resolved')}
            >
              Resolve
            </ActionButton>
            <ActionButton
              disabled={upsert.isPending || status === 'open'}
              onClick={() => setStatus('open')}
            >
              Reopen
            </ActionButton>
          </div>
          {/* Triage writes are easy to miss when they fail (a flaky upstream,
              a 409, etc.) — surface it inline so the action isn't silently
              dropped. The same `upsert` backs the status buttons and the
              assignee form below. */}
          {upsert.isError && (
            <InlineError>Could not save the change — {mutationErrorText(upsert.error)}</InlineError>
          )}
        </section>

        {/* Assignee */}
        <section>
          <h3 className="mb-2 text-xs uppercase tracking-wide text-text-subtle">Assignee</h3>
          <form className="flex items-center gap-2" onSubmit={onAssign}>
            <Input
              ref={assignInputRef}
              value={assigneeDraft}
              onChange={(e) => setAssigneeDraft(e.target.value)}
              placeholder="unassigned"
              className="h-8 flex-1 text-sm"
              aria-label="Assignee"
            />
            <Button type="submit" size="sm" disabled={upsert.isPending}>
              {upsert.isPending ? (
                <Loader2 className="h-3.5 w-3.5 animate-spin" />
              ) : (
                <Check className="h-3.5 w-3.5" />
              )}
            </Button>
          </form>
        </section>

        {/* Notes */}
        <section>
          <h3 className="mb-2 text-xs uppercase tracking-wide text-text-subtle">
            Notes
            {triage.data && (
              <span className="ml-2 text-text-subtle/80 normal-case tracking-normal">
                ({triage.data.notes.length})
              </span>
            )}
          </h3>
          {triage.data && triage.data.notes.length > 0 && (
            <ul className="mb-3 space-y-2">
              {triage.data.notes.map((n) => (
                <li
                  key={n.id}
                  className="rounded border border-border-subtle bg-bg-elevated/40 p-2"
                >
                  <div className="flex items-center justify-between text-[11px] text-text-muted">
                    <span>{n.author}</span>
                    <span>{relativeOrEmpty(n.created_at)} ago</span>
                  </div>
                  <p className="mt-1 text-sm whitespace-pre-wrap text-text-body">{n.body}</p>
                </li>
              ))}
            </ul>
          )}
          <form className="space-y-2" onSubmit={onSubmitNote}>
            <Label htmlFor="note-body" className="sr-only">
              Add note
            </Label>
            <textarea
              id="note-body"
              ref={noteInputRef}
              value={noteDraft}
              onChange={(e) => setNoteDraft(e.target.value)}
              placeholder="Add a note (Enter to send, Shift+Enter for new line)"
              rows={2}
              className="w-full resize-y rounded-md border border-border bg-bg-elevated px-2 py-1.5 text-sm text-text-body placeholder:text-text-subtle focus:outline-none focus:ring-2 focus:ring-accent"
              onKeyDown={(e) => {
                if (e.key === 'Enter' && !e.shiftKey) {
                  e.preventDefault();
                  onSubmitNote(e as unknown as FormEvent);
                }
              }}
            />
            <div className="flex justify-end">
              <Button type="submit" size="sm" disabled={appendNote.isPending || !noteDraft.trim()}>
                {appendNote.isPending ? 'Sending…' : 'Add note'}
              </Button>
            </div>
            {appendNote.isError && (
              <InlineError>
                Could not add the note — {mutationErrorText(appendNote.error)}
              </InlineError>
            )}
          </form>
        </section>

        {/* Raw payload — mono, scrollable inside its own container. */}
        <section>
          <h3 className="mb-2 text-xs uppercase tracking-wide text-text-subtle">Raw evidence</h3>
          <pre className="overflow-x-auto rounded border border-border-subtle bg-bg-elevated px-3 py-2 text-[11px] leading-relaxed font-mono">
            {JSON.stringify(event.evidence, null, 2)}
          </pre>
        </section>

        {/* Log — optional, only when there's transition history. */}
        {triage.data && triage.data.log.length > 0 && (
          <section>
            <h3 className="mb-2 text-xs uppercase tracking-wide text-text-subtle">History</h3>
            <ul className="space-y-1 text-xs">
              {triage.data.log.map((entry) => (
                <li
                  key={entry.id}
                  className="flex items-center justify-between gap-2 text-text-muted"
                >
                  <span>
                    <span className="text-text-primary">{entry.actor}</span>{' '}
                    {entry.from_status
                      ? `${entry.from_status} → ${entry.to_status}`
                      : `opened (${entry.to_status})`}
                  </span>
                  <span className="font-mono text-text-subtle">
                    {relativeOrEmpty(entry.at)} ago
                  </span>
                </li>
              ))}
            </ul>
          </section>
        )}
      </div>
    </div>
  );
}

// -----------------------------------------------------------------------------
// Helpers / small subcomponents
// -----------------------------------------------------------------------------

function FactGrid({
  event,
  ag,
}: {
  event: EventWithTriage;
  ag: ReturnType<typeof extractAiGuard>;
}) {
  return (
    <dl className="grid grid-cols-[110px_minmax(0,1fr)] gap-y-1.5 text-xs">
      <Fact label="Host">
        <code className="font-mono break-all">{event.host_id}</code>
      </Fact>
      {ag && (
        <>
          <Fact label="Tool">{humanTool(ag.tool)}</Fact>
          <Fact label="Score">
            <span className="font-mono">{ag.score.toFixed(1)}</span>{' '}
            <span className="text-text-subtle">/ 10</span>
          </Fact>
          <Fact label="Bucket">{ag.bucket}</Fact>
          <Fact label="Reattested">{ag.is_reattestation ? 'yes' : 'no'}</Fact>
        </>
      )}
      <Fact label="Time">
        <span title={event.ts}>{relativeOrEmpty(event.ts)} ago</span>
      </Fact>
      <Fact label="Severity">{event.severity}</Fact>
      <Fact label="Agent">v{event.agent_version}</Fact>
      {ag && ag.reasons.length > 0 && (
        <Fact label="Reasons">
          <ReasonList reasons={ag.reasons} />
        </Fact>
      )}
    </dl>
  );
}

function Fact({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <>
      <dt className="text-text-subtle">{label}</dt>
      <dd className="text-text-body min-w-0">{children}</dd>
    </>
  );
}

/** Inline, assertive error line for a failed triage write. */
function InlineError({ children }: { children: React.ReactNode }) {
  return (
    <p role="alert" className="mt-2 text-xs text-sev-critical">
      {children}
    </p>
  );
}

/** Safe display message for a failed mutation. ApiError subclasses carry a
 *  consumer-safe `.message` (the server already strips upstream URLs); anything
 *  else falls back to a generic line. */
function mutationErrorText(error: unknown): string {
  return error instanceof Error && error.message ? error.message : 'please try again';
}

function Badge({
  tone,
  children,
}: {
  tone: 'severity' | 'status' | 'muted';
  children: React.ReactNode;
}) {
  const toneClass =
    tone === 'severity'
      ? 'text-sev-critical border-sev-critical/40 bg-sev-critical/10'
      : tone === 'status'
        ? 'text-accent border-accent/40 bg-accent/10'
        : 'text-text-muted border-border bg-bg-elevated';
  return (
    <span
      className={cn(
        'inline-block rounded border px-1.5 py-px text-[10px] font-medium uppercase tracking-wide',
        toneClass,
      )}
    >
      {children}
    </span>
  );
}

function ActionButton({
  active,
  disabled,
  onClick,
  children,
}: {
  active?: boolean;
  disabled?: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <Button
      size="sm"
      variant={active ? 'primary' : 'secondary'}
      disabled={disabled}
      onClick={onClick}
    >
      {children}
    </Button>
  );
}

function severityLabel(s: string): string {
  return s === 'warn' ? 'warn' : s === 'info' ? 'info' : s;
}

function relativeOrEmpty(ts: string): string {
  try {
    return formatDistanceToNowStrict(new Date(ts));
  } catch {
    return '';
  }
}

// Re-export the search icon so future callers can share the bare-bones
// reference; keeps the bundle from importing lucide elsewhere just for this.
export const SlideOverSearchIcon = Search;
