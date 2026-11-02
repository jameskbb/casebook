import { beforeEach, describe, expect, it, vi } from 'vitest'

const binding = vi.hoisted(() => {
  const names = [
    'StartupState', 'DataLocation', 'Quit', 'ChangeToken', 'CheckForUpdates', 'OpenReleasePage', 'ListInvestigations', 'GetInvestigationDetail',
    'CreateInvestigation', 'UpdateInvestigation', 'ArchiveInvestigation', 'UnarchiveInvestigation',
    'UpdateConclusion', 'SetActiveInvestigation', 'ClearActiveInvestigation', 'Search', 'AddEvidence',
    'GetEvidence', 'UpdateEvidence', 'SetEvidenceVerification', 'AddClaim', 'GetClaim', 'UpdateClaim',
    'SetClaimVerification', 'SupersedeClaim', 'LinkEvidenceToClaim', 'UnlinkEvidenceFromClaim',
    'LinkEvidenceToHypothesis', 'UnlinkEvidenceFromHypothesis', 'AddHypothesis', 'GetHypothesis',
    'UpdateHypothesis', 'SetHypothesisVerification', 'SupersedeHypothesis', 'AddQuestion', 'UpdateQuestion',
    'ResolveQuestion', 'DeferQuestion', 'ReopenQuestion', 'CopyContext', 'CopyContextPlain', 'CopyReport', 'ReportPreview', 'SaveReport', 'CopyText', 'SetWindowTheme',
    'ExportInvestigation', 'ImportInvestigation', 'BackUp', 'PickBackupToRestore', 'RestoreBackup',
    'ClientConnections', 'AppVersion', 'OpenDataFolder', 'CopyDataLocation', 'PickBackupToRecover',
    'RecoverFromBackup', 'DeleteInvestigation', 'DeleteEvidence', 'DeleteClaim', 'DeleteHypothesis',
    'DeleteQuestion', 'GetReviewFindings', 'ReviewStatement',
    'SharePreview', 'CopySharePreview', 'SaveSharePreview',
  ]
  return Object.fromEntries(names.map((n) => [n, vi.fn()])) as Record<string, ReturnType<typeof vi.fn>>
})
vi.mock('../../wailsjs/go/main/Desktop', () => binding)

import { api } from './api'
import { ApiError } from './errors'
import { mockApi } from '../test/mockApi'

describe('api', () => {
  beforeEach(() => {
    for (const fn of Object.values(binding)) {
      fn.mockReset()
      fn.mockResolvedValue(undefined)
    }
  })

  it('wraps every generated binding', () => {
    expect(Object.keys(api)).toHaveLength(Object.keys(binding).length)
  })

  it('has no note-block, notebook or ENEX methods, and neither does the test mock', () => {
    const removed = /evernote|noteblock|notebook|enex/i
    expect(Object.keys(api).filter((k) => removed.test(k))).toEqual([])
    expect(Object.keys(mockApi()).filter((k) => removed.test(k))).toEqual([])
  })

  it('passes arguments through unchanged', async () => {
    binding.ListInvestigations.mockResolvedValue([{ ref: 'INV-0001' }])
    await expect(api.listInvestigations({ include_archived: true })).resolves.toEqual([{ ref: 'INV-0001' }])
    expect(binding.ListInvestigations).toHaveBeenCalledWith({ include_archived: true })

    await api.getInvestigationDetail('INV-0001', { activity_limit: 200 })
    expect(binding.GetInvestigationDetail).toHaveBeenCalledWith('INV-0001', { activity_limit: 200 })

    await api.updateClaim('INV-0001', 'C-001', { confidence: 'high' })
    expect(binding.UpdateClaim).toHaveBeenCalledWith('INV-0001', 'C-001', { confidence: 'high' })

    await api.linkEvidenceToHypothesis('INV-0001', 'H-001', 'E-002', 'contradicts')
    expect(binding.LinkEvidenceToHypothesis).toHaveBeenCalledWith('INV-0001', 'H-001', 'E-002', 'contradicts')

    await api.resolveQuestion('INV-0001', 'Q-001', 'Because.')
    expect(binding.ResolveQuestion).toHaveBeenCalledWith('INV-0001', 'Q-001', 'Because.')

    await api.restoreBackup('C:\\b.db')
    expect(binding.RestoreBackup).toHaveBeenCalledWith('C:\\b.db')

    await api.copyContextPlain('INV-0001')
    expect(binding.CopyContextPlain).toHaveBeenCalledWith('INV-0001')

    await api.copyReport('INV-0002')
    expect(binding.CopyReport).toHaveBeenCalledWith('INV-0002')
    expect(binding.CopyContext).not.toHaveBeenCalled()

    await api.copySharePreview('reviewed-snapshot')
    expect(binding.CopySharePreview).toHaveBeenCalledWith('reviewed-snapshot')
    await api.saveSharePreview('reviewed-snapshot')
    expect(binding.SaveSharePreview).toHaveBeenCalledWith('reviewed-snapshot')

    await api.setWindowTheme('dark')
    expect(binding.SetWindowTheme).toHaveBeenCalledWith('dark')
  })

  it('turns a rejected binding into an ApiError of the decoded kind', async () => {
    binding.CreateInvestigation.mockRejectedValue('{"kind":"validation","field":"title","message":"is required"}')
    const err = await api.createInvestigation({ title: '' }).catch((e: unknown) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect(err).toMatchObject({ kind: 'validation', field: 'title', message: 'is required' })

    binding.ChangeToken.mockRejectedValue('{"kind":"busy","message":"Casebook database is busy; try again"}')
    await expect(api.changeToken()).rejects.toMatchObject({ kind: 'busy' })

    binding.CopyContext.mockRejectedValue(new Error('runtime is gone'))
    await expect(api.copyContext('INV-0001')).rejects.toMatchObject({ kind: 'other', message: 'runtime is gone' })
  })
})
