/*
Package icon provides compiled icons for the application.
*/
package icon

import (
	"embed"
	_ "embed"
	"runtime"
)

//go:embed assets/*
var assetsFS embed.FS

var (
	LogoPassive []byte
	LogoActive  []byte
	AppLogo     []byte
)

func init() {
	AppLogo = ReadFile("assets/app.png")
	if runtime.GOOS == "darwin" {
		LogoPassive = ReadFile("assets/icon_default.svg")
		LogoActive = ReadFile("assets/icon_active.svg")
	} else if runtime.GOOS == "windows" {
		LogoPassive = ReadFile("assets/icon_default.ico")
		LogoActive = ReadFile("assets/icon_active.ico")
	} else {
		LogoPassive = ReadFile("assets/icon_default.png")
		LogoActive = ReadFile("assets/icon_active.png")
	}
}

func ReadFile(path string) []byte {
	b, err := assetsFS.ReadFile(path)
	if err != nil {
		panic(err)
	}

	return b
}
