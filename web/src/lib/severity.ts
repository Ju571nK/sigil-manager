/**
 * Maps an AI Guard / severity bucket to its Tailwind text-color token
 * (`text-sev-*`). Shared by the alerts queue rows and the host AI Guard block
 * so the bucket→color mapping lives in one place. Unknown/future buckets fall
 * back to the info color. Callers that need a background derive it with
 * `.replace('text-', 'bg-')`.
 */
export function bucketTextColor(bucket: string): string {
  switch (bucket) {
    case 'critical':
      return 'text-sev-critical';
    case 'high':
      return 'text-sev-high';
    case 'medium':
      return 'text-sev-medium';
    case 'low':
      return 'text-sev-low';
    default:
      return 'text-sev-info';
  }
}
