// The sidebar footer, kept to one
// quiet row so the question list keeps the height, even at the 960 by
// 640 minimum window: a "Data" menu with Add a question from a file...,
// Back up all data..., Restore from backup... and the data file location
// (middle-truncated, the full location in title and the accessible name;
// choosing it copies the location), and the System, Light and Dark switch as
// three labelled icon segments. Below the data operations, "Connect your
// AI..." opens the read-only Connect your AI panel, the codes item shows or
// hides the short codes an AI uses for records (C-001), "Check
// for updates..." opens the dialog that asks GitHub for the latest version
// only when the person chooses Check now, and the last item shows
// the version (choosing it copies "Casebook <version>"), with a maker credit
// below the actions.
import { useEffect, useState } from 'react'
import { ArrowDownToLine, Cable, ChevronDown, Copy, Database, DatabaseBackup, Hash, Info, RotateCcw, Upload } from 'lucide-react'
import { CLIPBOARD_COPY } from '../../api/errors'
import { api } from '../../api/api'
import { getShowIds, setShowIds } from '../../lib/showIds'
import { truncateMiddle } from '../../lib/truncatePath'
import { useAppData } from '../../state/appData'
import { CheckUpdatesDialog } from '../dialogs/CheckUpdatesDialog'
import { ConnectClientsDialog } from '../dialogs/ConnectClientsDialog'
import { Menu, type MenuItem } from '../ui/Menu'
import { useToast } from '../ui/Toaster'
import { PATH_COPIED, useDataOps } from './DataOps'
import { ThemeSwitch } from './ThemeSwitch'
import './SidebarFooter.css'

export const SHOW_IDS_LABEL = 'Show the codes your AI uses'
export const HIDE_IDS_LABEL = 'Hide the codes your AI uses'

export function SidebarFooter() {
  const { dataLocation } = useAppData()
  const ops = useDataOps()
  const toast = useToast()
  const [connecting, setConnecting] = useState(false)
  const [checkingUpdates, setCheckingUpdates] = useState(false)
  const [version, setVersion] = useState('')
  const [showIds, setShowIdsState] = useState(getShowIds)

  useEffect(() => {
    let live = true
    api
      .appVersion()
      .then((v) => {
        if (live) setVersion(v)
      })
      .catch(() => {})
    return () => {
      live = false
    }
  }, [])

  const copyLocation = async (path: string) => {
    try {
      await api.copyText(path)
      toast.success(`${PATH_COPIED}.`)
    } catch {
      toast.error(CLIPBOARD_COPY)
    }
  }

  const copyVersion = async (v: string) => {
    try {
      await api.copyText(`Casebook ${v}`)
      toast.success('Version copied to the clipboard.')
    } catch {
      toast.error(CLIPBOARD_COPY)
    }
  }

  const items: MenuItem[] = [
    { label: 'Add a question from a file...', icon: Upload, onSelect: () => void ops.importInvestigation() },
    { label: 'Back up all data...', icon: DatabaseBackup, onSelect: () => void ops.backUp() },
    { label: 'Restore from backup...', icon: RotateCcw, onSelect: () => void ops.restore() },
  ]
  if (dataLocation) {
    items.push({
      label: 'Data file',
      detail: truncateMiddle(dataLocation),
      icon: Copy,
      title: dataLocation,
      ariaLabel: `Data file ${dataLocation}. Copy its location`,
      separatorBefore: true,
      onSelect: () => void copyLocation(dataLocation),
    })
  }
  items.push({
    label: 'Connect your AI...',
    icon: Cable,
    separatorBefore: true,
    onSelect: () => setConnecting(true),
  })
  items.push({
    label: showIds ? HIDE_IDS_LABEL : SHOW_IDS_LABEL,
    detail: 'Like C-001 and E-004',
    icon: Hash,
    onSelect: () => {
      setShowIds(!showIds)
      setShowIdsState(!showIds)
    },
  })
  items.push({
    label: 'Check for updates...',
    icon: ArrowDownToLine,
    onSelect: () => setCheckingUpdates(true),
  })
  if (version) {
    items.push({
      label: 'Version',
      detail: version,
      icon: Info,
      ariaLabel: `Casebook version ${version}. Copy it`,
      onSelect: () => void copyVersion(version),
    })
  }

  return (
    <div className="sidebar-footer">
      <Menu
        label="Data"
        items={items}
        footer={<>Made with <span role="img" aria-label="love">❤️</span> by jameskbb</>}
        trigger={(props) => (
          <button {...props} type="button" className="sidebar-footer-data">
            <Database size={16} strokeWidth={1.75} aria-hidden="true" />
            <span>Data</span>
            <ChevronDown className="sidebar-footer-chevron" size={16} strokeWidth={1.75} aria-hidden="true" />
          </button>
        )}
      />
      <ThemeSwitch />
      {connecting && <ConnectClientsDialog onClose={() => setConnecting(false)} />}
      {checkingUpdates && <CheckUpdatesDialog onClose={() => setCheckingUpdates(false)} />}
    </div>
  )
}
