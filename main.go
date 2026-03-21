package main

import (
	"os"

	"github.com/weiqinzhou3/milvus-health/cmd"
)

func main() {
	os.Exit(cmd.Execute())
}
