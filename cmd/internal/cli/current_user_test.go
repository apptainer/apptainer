// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies

package cli

import (
	"testing"

	"github.com/apptainer/apptainer/internal/pkg/util/user"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultHomeDir(t *testing.T) {
	saved := CurrentUser
	t.Cleanup(func() { CurrentUser = saved })

	CurrentUser = nil
	assert.Equal(t, "", defaultHomeDir())

	CurrentUser = &user.User{Dir: "/home/someone"}
	assert.Equal(t, "/home/someone", defaultHomeDir())
}

func TestCheckCurrentUser(t *testing.T) {
	me, err := user.Current()
	require.NoError(t, err)
	known := &user.User{Name: "known", Dir: "/home/known"}

	tests := []struct {
		name      string
		current   *user.User
		homeArgs  []string
		wantUser  *user.User
		wantHome  string
		startHome string
	}{
		{
			name:      "user found at setup is kept",
			current:   known,
			wantUser:  known,
			startHome: "/home/known",
			wantHome:  "/home/known",
		},
		{
			name:     "user looked up late fills in the home default",
			current:  nil,
			wantUser: me,
			wantHome: me.Dir,
		},
		{
			name:     "user looked up late keeps an explicit --home",
			current:  nil,
			homeArgs: []string{"--home", "/custom"},
			wantUser: me,
			wantHome: "/custom",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			savedUser, savedHome := CurrentUser, homePath
			t.Cleanup(func() { CurrentUser, homePath = savedUser, savedHome })

			CurrentUser = tt.current
			cmd := &cobra.Command{}
			cmd.Flags().StringVarP(&homePath, actionHomeFlag.Name, actionHomeFlag.ShortHand, tt.startHome, "")
			require.NoError(t, cmd.ParseFlags(tt.homeArgs))

			checkCurrentUser(cmd)
			assert.Equal(t, tt.wantUser, CurrentUser)
			assert.Equal(t, tt.wantHome, homePath)
		})
	}
}
