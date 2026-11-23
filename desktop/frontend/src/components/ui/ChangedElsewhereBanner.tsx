// Shown by an editor that holds unsaved input when a reload changed the
// stored values it started from. The input stays until the person chooses
// Show the new version.
import { Banner } from './Banner'
import { Button } from './Button'

export const CHANGED_ELSEWHERE_COPY = 'This changed while you had it open. Your AI may have updated it.'
export const SHOW_NEW_VERSION = 'Show the new version'

export function ChangedElsewhereBanner({ onShowLatest }: { onShowLatest: () => void }) {
  return (
    <Banner
      className="changed-elsewhere"
      action={
        <Button variant="ghost" onClick={onShowLatest}>
          {SHOW_NEW_VERSION}
        </Button>
      }
    >
      {CHANGED_ELSEWHERE_COPY}
    </Banner>
  )
}
