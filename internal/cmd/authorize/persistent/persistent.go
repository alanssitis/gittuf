// SPDX-License-Identifier: Apache-2.0

package persistent

import "github.com/spf13/cobra"

type Options struct {
	SigningKey string
}

func (o *Options) AddPersistentFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVarP(
		&o.SigningKey,
		"signing-key",
		"k",
		"",
		"signing key to use to sign authorization",
	)
	cmd.MarkPersistentFlagRequired("signing-key") //nolint:errcheck
}
