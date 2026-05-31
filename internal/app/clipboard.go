package app

import "github.com/atotto/clipboard"

// writeClipboard writes text to the system clipboard.
// Errors are silently ignored on headless / unsupported environments.
func writeClipboard(text string) error {
	return clipboard.WriteAll(text)
}
