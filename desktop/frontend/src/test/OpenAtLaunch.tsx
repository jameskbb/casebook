// Nothing is open at launch (the main pane shows Home). A test of the
// question page puts this inside AppDataProvider to open one question as
// soon as the list has loaded: your AI's current question, else the first
// one that is not archived.
import { useEffect, useRef } from 'react'
import { useAppData } from '../state/appData'

export function OpenAtLaunch() {
  const { listStatus, investigations, select } = useAppData()
  const done = useRef(false)
  useEffect(() => {
    if (done.current || listStatus !== 'ready') return
    done.current = true
    const pick = investigations.find((i) => i.is_active) ?? investigations.find((i) => !i.archived)
    if (pick) select(pick.ref)
  }, [listStatus, investigations, select])
  return null
}
