package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print duck version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("duck version", Version)
		},
	}
	rootCmd.AddCommand(versionCmd)
}
