// Middle truncation for the data file path in the sidebar footer (UI-SPEC
// Layout, sidebar footer): the first 12 and last 28 characters joined by
// "...", so both the drive and the file name stay visible. The full path
// goes in title and aria-label.

export function truncateMiddle(path: string, head = 12, tail = 28): string {
  if (path.length <= head + tail + 3) return path
  return `${path.slice(0, head)}...${path.slice(path.length - tail)}`
}
