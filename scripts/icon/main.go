// Regenerate the packaged app icon from the same shape as the menu-bar icon.
package main

import (
	"log"
	"os"
	"xiaoyuzhou-menubar/internal/appicon"
)

func main() {
	if err := os.WriteFile("resources/icon.png", appicon.AppPNG(), 0644); err != nil {
		log.Fatal(err)
	}
}
