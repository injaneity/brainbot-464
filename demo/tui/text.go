package tui

// UI Text Constants
const (
	// Instructions
	TextStartInstruction    = "Press 'd' to fetch new articles (Incremental)"
	TextResetInstruction    = "Press 'r' to reset database & fetch (Clean Start)"
	TextDetachInstruction   = "Press 'q' to detach (orchestrator keeps running)"
	TextShutdownInstruction = "Press 'x' to shutdown orchestrator and quit"
	TextCronNote            = "Note: Orchestrator runs automatically on a schedule (cron)."

	// Footer
	TextFooterIdle    = "d: Fetch New | r: Reset & Fetch | q: Detach | x: Shutdown"
	TextFooterRunning = "q: Detach (workflow continues) | x: Shutdown Orchestrator"
)
