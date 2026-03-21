package main

import (
	"os"

	"milvus-health/cmd"
)

func main() {
	os.Exit(cmd.Execute())
}
