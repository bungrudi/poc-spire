package ledger

// HoldForTest makes contention explicit in external acceptance-boundary tests;
// it is absent from the application build.
func HoldForTest(book *Ledger) func() { book.mu.Lock(); return book.mu.Unlock }
