//go:build !unix

package main

// processAlive cannot check a pid portably here, so a lock is always treated as held; remove the
// lock file by hand after a crash.
func processAlive(int) bool { return true }
