package envdata

import (
	"os"
	"path/filepath"
)

func ExeDir() string {
	exe, err := os.Executable()
	if err != nil {
		panic(err)
	}
	return filepath.Dir(exe)
}

func EnvfilePath() string {
	return filepath.Join(ExeDir(), ".env")
}
