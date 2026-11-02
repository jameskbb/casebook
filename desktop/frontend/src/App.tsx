// The app shell: providers, the sidebar and the main pane. App owns the New
// investigation dialog so the sidebar, the empty state and the Ctrl+N
// shortcut all open the same one, and binds the global keyboard map.
import { useCallback, useState } from 'react'
import { AppDataProvider, useAppData } from './state/appData'
import { DirtyGuardProvider } from './state/dirtyGuard'
import { CopyContextProvider, useCopyContext } from './state/copyContext'
import { useGlobalShortcuts } from './hooks/useGlobalShortcuts'
import { ToastProvider } from './components/ui/Toaster'
import { Banner } from './components/ui/Banner'
import { Sidebar } from './components/sidebar/Sidebar'
import { DataOpsProvider, useDataOps } from './components/sidebar/DataOps'
import { DetailView } from './components/detail/DetailView'
import { NewInvestigationDialog } from './components/dialogs/NewInvestigationDialog'
import { EmptyMain } from './components/EmptyMain'
import { StartupError } from './components/StartupError'
import './App.css'

export default function App() {
  return (
    <ToastProvider>
      <DirtyGuardProvider>
        <AppDataProvider>
          <CopyContextProvider>
            <DataOpsProvider>
              <Shell />
            </DataOpsProvider>
          </CopyContextProvider>
        </AppDataProvider>
      </DirtyGuardProvider>
    </ToastProvider>
  )
}

function Shell() {
  const { startup, selectedRef, investigations, listStatus, pollTrouble } = useAppData()
  const dataOps = useDataOps()
  const { copy } = useCopyContext()
  const [newOpen, setNewOpen] = useState(false)
  const [focusTitleRef, setFocusTitleRef] = useState<string | null>(null)
  const openNew = useCallback(() => setNewOpen(true), [])
  const titleFocused = useCallback(() => setFocusTitleRef(null), [])
  const appReady = startup?.ok === true
  useGlobalShortcuts({
    onNew: () => {
      if (appReady) openNew()
    },
    onCopyContext: appReady && selectedRef ? () => void copy(selectedRef) : undefined,
  })

  if (!startup) return <div className="app app-booting" />
  if (!startup.ok) return <StartupError info={startup} />

  let content = null
  if (selectedRef) {
    content = <DetailView focusTitle={focusTitleRef === selectedRef} onTitleFocused={titleFocused} />
  } else if (listStatus === 'ready' && investigations.length === 0) {
    content = <EmptyMain onNew={openNew} onImport={() => void dataOps.importInvestigation()} />
  } else if (listStatus === 'ready') {
    content = <p className="main-quiet">Choose a case from the list.</p>
  }

  return (
    <div className="app">
      <Sidebar onNew={openNew} />
      <main className="main-pane">
        {pollTrouble && (
          <Banner tone="warning" layout="bar">
            Casebook can't check for changes right now. It will keep trying.
          </Banner>
        )}
        <div className="main-scroll">{content}</div>
      </main>
      {newOpen && (
        <NewInvestigationDialog onClose={() => setNewOpen(false)} onCreated={(ref) => setFocusTitleRef(ref)} />
      )}
    </div>
  )
}
