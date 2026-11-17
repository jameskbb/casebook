// A file path or source reference that wraps after its slashes
// (internal/checkout/ | totals_test.go:41) instead of in the middle of a
// name. Each / or \ is followed by a <wbr>; the text itself is unchanged,
// so copying it gives the original string.
import { Fragment } from 'react'

export function BreakablePath({ text }: { text: string }) {
  const parts = text.split(/(?<=[/\\])/)
  return (
    <>
      {parts.map((p, i) => (
        <Fragment key={i}>
          {i > 0 && <wbr />}
          {p}
        </Fragment>
      ))}
    </>
  )
}
