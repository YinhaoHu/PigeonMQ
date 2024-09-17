package recovery

import (
	"porage/internal/journal"
	"porage/internal/ledger"
	"porage/internal/pkg"
)

type recoverLedgerInfo struct {
	ledger      *ledger.Ledger
	fromEntryID int
}

// Recover recovers the ledgers from the journal.
//
// Expected to be called after the local storage is initialized.
func Recover() ([]*ledger.Ledger, error) {
	pkg.Logger.Infof("Recovering ledgers from journal.")
	persistentLedgerIDList, err := ledger.GetPersistentLedgerIDList()
	if err != nil {
		return nil, err
	}

	ledgers := make([]*ledger.Ledger, 0)
	recoveryLedgerInfoMap := make(map[uint64]*recoverLedgerInfo)
	for _, ledgerID := range persistentLedgerIDList {
		thisLedger, err := ledger.NewLedger(ledgerID)
		if err != nil {
			return nil, err
		}
		fromEntryID, err := thisLedger.PrepareRecovery()
		if err != nil {
			return nil, err
		}
		ledgers = append(ledgers, thisLedger)
		recoveryLedgerInfoMap[ledgerID] = &recoverLedgerInfo{
			ledger:      thisLedger,
			fromEntryID: fromEntryID,
		}
	}

	totalRecovered := 0
	segmentIndex := 0
	for {
		journalEntries, nextSegmentIdx, err := journal.ReadJournal(segmentIndex)
		if err != nil {
			return nil, err
		}

		for _, journalEntry := range journalEntries {
			recoverLedgerInfo, ok := recoveryLedgerInfoMap[journalEntry.Entry.LedgerID]
			if !ok {
				continue
			}

			if journalEntry.Entry.EntryID < recoverLedgerInfo.fromEntryID {
				continue
			}

			pkg.Logger.Debugf("Recovering entry %d in ledger %d with payload len %v.", journalEntry.Entry.EntryID, journalEntry.Entry.LedgerID, len(journalEntry.Entry.Payload))

			if err := recoverLedgerInfo.ledger.PutEntryOnRecovery(journalEntry.Entry.Payload); err != nil {
				return nil, err
			}
		}
		totalRecovered += len(journalEntries)
		pkg.Logger.Debugf("Recovered %d entries.", len(journalEntries))

		if nextSegmentIdx < 0 {
			break
		}
		segmentIndex = nextSegmentIdx
	}

	pkg.Logger.Infof("Recovery completed. Total recovered entries: %d.", totalRecovered)
	return ledgers, nil
}