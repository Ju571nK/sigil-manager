import { useQuery } from '@tanstack/react-query';
import { NotFoundError } from '@/api/client';
import { type EventWithTriage, fleetEventByID } from '@/api/fleet';
import { SlideOver } from '@/components/AlertsQueue/SlideOver';
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet';

const ignoreFocus = (_fn: () => void) => undefined;
interface Props {
  eventID: string | null;
  rows: EventWithTriage[];
  onClose: () => void;
  hostID?: string;
  registerFocusAssign?: (fn: () => void) => void;
  registerFocusNote?: (fn: () => void) => void;
}

/** Resolve a shared URL even when the event is outside the loaded pages. */
export function EventDetails({
  eventID,
  rows,
  onClose,
  hostID,
  registerFocusAssign = ignoreFocus,
  registerFocusNote = ignoreFocus,
}: Props) {
  const loaded = rows.find((row) => row.event_id === eventID);
  const q = useQuery({
    queryKey: ['fleet', 'event', eventID],
    queryFn: ({ signal }) => fleetEventByID(eventID as string, signal),
    enabled: !!eventID && !loaded,
    retry: false,
  });
  if (!eventID) return null;
  const event = loaded ?? q.data;
  if (event && (!hostID || hostID === event.host_id)) {
    return (
      <SlideOver
        event={event}
        onClose={onClose}
        registerFocusAssign={registerFocusAssign}
        registerFocusNote={registerFocusNote}
      />
    );
  }
  const mismatch = !!event && !!hostID && event.host_id !== hostID;
  return (
    <Sheet
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <SheetContent className="overflow-y-auto p-5">
        <SheetHeader>
          <SheetTitle>Event detail</SheetTitle>
          <SheetDescription>{eventID}</SheetDescription>
        </SheetHeader>
        {mismatch || q.error instanceof NotFoundError ? (
          <p className="mt-4">
            Event unavailable for this view. It may have expired from retention.
          </p>
        ) : q.error ? (
          <div className="mt-4">
            <p role="alert">Failed to load event: {q.error.message}</p>
            <button
              type="button"
              className="mt-2 text-accent"
              disabled={q.isFetching}
              onClick={() => q.refetch()}
            >
              Retry event
            </button>
          </div>
        ) : (
          <p className="mt-4" role="status">
            Loading event…
          </p>
        )}
      </SheetContent>
    </Sheet>
  );
}
