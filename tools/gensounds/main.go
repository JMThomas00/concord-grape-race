// Command gensounds writes the Concord Arcade sound kit into client/sounds
// (run from the repo root: go generate). The files are committed, so
// releases carry them.
package main

import (
	"log"

	"github.com/JMThomas00/Concord/sdk/arcade"
)

func main() {
	if err := arcade.WriteSoundKit("client"); err != nil {
		log.Fatal(err)
	}
}
