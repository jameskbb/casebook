// The sidebar footer, kept to one quiet row so the question list keeps the
// height, even at the 960 by 640 minimum window: a "Settings" menu and the
// System, Light and Dark switch as three labelled icon segments. The menu
// holds what a person does: Connect your AI..., Open a question someone sent
// you..., Save a backup..., Bring back a backup... and Look for updates...
// The last entry, Technical..., opens a small window with what only a
// technical helper needs: the codes switch, saving the open question as a
// file to move it to another computer, where the questions are kept and the
// version. Saving runs the native save window in Go; a cancelled one shows
// nothing.
import { useEffect, useRef, useState } from 'react'
import { ArrowDownToLine, Cable, ChevronDown, Copy, DatabaseBackup, Download, RotateCcw, Settings, Upload, Wrench } from 'lucide-react'
import { CLIPBOARD_COPY, errorCopy, toApiError } from '../../api/errors'
import { api } from '../../api/api'
import { getShowIds, setShowIds } from '../../lib/showIds'
import { useAppData } from '../../state/appData'
import { CheckUpdatesDialog } from '../dialogs/CheckUpdatesDialog'
import { ConnectClientsDialog } from '../dialogs/ConnectClientsDialog'
import { BreakablePath } from '../ui/BreakablePath'
import { Button } from '../ui/Button'
import { Dialog } from '../ui/Dialog'
import { Menu, type MenuItem } from '../ui/Menu'
import { Switch } from '../ui/Switch'
import { useToast } from '../ui/Toaster'
import { PATH_COPIED, useDataOps } from './DataOps'
import { ThemeSwitch } from './ThemeSwitch'
import './SidebarFooter.css'

export const SETTINGS_LABEL = 'Settings'
export const OPEN_SENT_QUESTION_LABEL = 'Open a question someone sent you...'
export const SHOW_IDS_LABEL = 'Show the codes your AI uses'
export const SHOW_IDS_HINT = 'Like C-001 and E-004.'
export const SAVE_QUESTION_LABEL = 'Save a question as a file to move it'
export const SAVE_QUESTION_NONE = 'Open the question first, then come back here.'

export function SidebarFooter() {
  const ops = useDataOps()
  const [open, setOpen] = useState<'connect' | 'updates' | 'technical' | null>(null)

  const items: MenuItem[] = [
    { label: 'Connect your AI...', icon: Cable, onSelect: () => setOpen('connect') },
    { label: OPEN_SENT_QUESTION_LABEL, icon: Upload, onSelect: () => void ops.importInvestigation() },
    { label: 'Save a backup...', icon: DatabaseBackup, onSelect: () => void ops.backUp() },
    { label: 'Bring back a backup...', icon: RotateCcw, onSelect: () => void ops.restore() },
    { label: 'Look for updates...', icon: ArrowDownToLine, onSelect: () => setOpen('updates') },
    { label: 'Technical...', icon: Wrench, separatorBefore: true, onSelect: () => setOpen('technical') },
  ]

  return (
    <div className="sidebar-footer">
      <Menu
        label={SETTINGS_LABEL}
        items={items}
        trigger={(props) => (
          <button {...props} type="button" className="sidebar-footer-data">
            <Settings size={16} strokeWidth={1.75} aria-hidden="true" />
            <span>{SETTINGS_LABEL}</span>
            <ChevronDown className="sidebar-footer-chevron" size={16} strokeWidth={1.75} aria-hidden="true" />
          </button>
        )}
      />
      <ThemeSwitch />
      {open === 'connect' && <ConnectClientsDialog onClose={() => setOpen(null)} />}
      {open === 'updates' && <CheckUpdatesDialog onClose={() => setOpen(null)} />}
      {open === 'technical' && <TechnicalDialog onClose={() => setOpen(null)} />}
    </div>
  )
}

/** What only a technical helper needs: the codes switch, where the questions are kept, the version. */
function TechnicalDialog({ onClose }: { onClose: () => void }) {
  const { dataLocation, selectedRef, detail } = useAppData()
  const open = selectedRef && detail?.investigation.ref === selectedRef ? detail.investigation : null
  const toast = useToast()
  const doneRef = useRef<HTMLButtonElement>(null)
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

  const copy = async (text: string, done: string) => {
    try {
      await api.copyText(text)
      toast.success(done)
    } catch {
      toast.error(CLIPBOARD_COPY)
    }
  }

  const exportQuestion = async (ref: string) => {
    try {
      const res = await api.exportInvestigation(ref)
      if (!res.cancelled) toast.success(`Saved a copy as ${res.file_name}.`)
    } catch (e) {
      toast.error(errorCopy(toApiError(e)).message)
    }
  }

  return (
    <Dialog title="Technical" size="confirm" onClose={onClose} initialFocus={doneRef} footer={<Button variant="primary" ref={doneRef} onClick={onClose}>Done</Button>}>
      <div className="technical-rows">
        <div>
          <Switch
            label={SHOW_IDS_LABEL}
            checked={showIds}
            onChange={(on) => {
              setShowIds(on)
              setShowIdsState(on)
            }}
          />
          <p className="technical-hint">{SHOW_IDS_HINT}</p>
        </div>
        <div>
          <p className="technical-label">{SAVE_QUESTION_LABEL}</p>
          <p className="technical-hint">{open ? `The question open now: ${open.title}` : SAVE_QUESTION_NONE}</p>
          <Button variant="secondary" icon={Download} disabled={!open} onClick={() => open && void exportQuestion(open.ref)}>Save as a file...</Button>
        </div>
        {dataLocation && (
          <div>
            <p className="technical-label">Where your questions are kept</p>
            <p className="technical-value"><BreakablePath text={dataLocation} /></p>
            <Button variant="secondary" icon={Copy} onClick={() => void copy(dataLocation, `${PATH_COPIED}.`)}>Copy location</Button>
          </div>
        )}
        {version && (
          <div>
            <p className="technical-label">Version</p>
            <p className="technical-value tabular">{version.replace(/-.*$/, '')}</p>
            <Button variant="secondary" icon={Copy} onClick={() => void copy(`Casebook ${version}`, 'Version copied.')}>Copy version</Button>
          </div>
        )}
      </div>
    </Dialog>
  )
}
