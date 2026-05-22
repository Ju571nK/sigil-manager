import { Link } from '@tanstack/react-router';

const TABS = [
  { to: '/fleet/risk', label: 'Risk' },
  { to: '/fleet/events', label: 'Events' },
  { to: '/fleet/compliance', label: 'Compliance' },
] as const;

/** Route-based tab bar for the Fleet section (UI/UX §5.2). */
export function FleetTabs() {
  return (
    <nav className="flex items-center gap-1 border-b border-border-subtle">
      {TABS.map((t) => (
        <Link
          key={t.to}
          to={t.to}
          activeProps={{ className: 'text-text-primary border-accent' }}
          inactiveProps={{
            className: 'text-text-muted border-transparent hover:text-text-primary',
          }}
          className="border-b-2 px-3 py-2 text-sm transition-colors -mb-px"
        >
          {t.label}
        </Link>
      ))}
    </nav>
  );
}
