package mtx

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	registerCmd.Flags().String("token", "", "registration token")
	rootCmd.AddCommand(registerCmd)
}

var registerCmd = &cobra.Command{
	Use:   "register <username> <password>",
	Short: "Register a Matrix user and print access token",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, _ := cmd.Flags().GetString("token")
		client := NewClient(Homeserver, "")
		return runRegister(args[0], args[1], token, client.Register)
	},
}

func runRegister(username, password, regToken string, register Registerer) error {
	reg, err := register(username, password, regToken)
	if err != nil {
		return err
	}
	fmt.Printf("%s %s\n", reg.UserID, reg.AccessToken)
	return nil
}
