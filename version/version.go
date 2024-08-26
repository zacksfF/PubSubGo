package version

import "fmt"

var (
	//Version release version 
	Version = "0.1.1"

	//Commit will be overwritten automatically by the build system
	Commit = "HEAD"
)

//FullVersion returns the full the full version,build and commit hash
func FullVersion() string{
	return fmt.Sprintf("%s@%s", Version, Commit)
}