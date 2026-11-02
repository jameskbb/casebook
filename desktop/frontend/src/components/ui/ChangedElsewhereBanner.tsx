// "This changed in another app." with "Show latest" (UI-SPEC edit-in-place
// safety): shown by an editor that holds unsaved input when a reload changed
// the stored values it started from. The input stays until the user chooses
// Show latest.
import { Banner } from './Banner'
import { Button } from './Button'

export const CHANGED_ELSEWHERE_COPY = 'This changed in another app.'

export function ChangedElsewhereBanner({ onShowLatest }: { onShowLatest: () => void }) {
  return (
    <Banner
      className="changed-elsewhere"
      action={
        <Button variant="ghost" onClick={onShowLatest}>
          Show latest
        </Button>
      }
    >
      {CHANGED_ELSEWHERE_COPY}
    </Banner>
  )
}
