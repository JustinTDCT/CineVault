//go:build !unix

package transcode

import "os"

// Pausing isn't supported here; FFmpeg runs ahead unthrottled.
func suspend(*os.Process) bool { return false }

func resumeProcess(*os.Process) {}
