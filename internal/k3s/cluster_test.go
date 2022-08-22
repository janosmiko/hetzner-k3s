package k3s

import (
	"os/user"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_resolvePath(t *testing.T) {
	t.Parallel()

	usr, _ := user.Current()

	type args struct {
		filepath string
	}

	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "resolving home directory",
			args: args{
				filepath: "~",
			},
			want: usr.HomeDir,
		},
		{
			name: "resolving a file in home directory",
			args: args{
				filepath: "~/.ssh/id_rsa",
			},
			want: path.Join(usr.HomeDir, ".ssh/id_rsa"),
		},
		{
			name: "resolving a regular file path (shouldn't change)",
			args: args{
				filepath: "./kubeconfig",
			},
			want: "./kubeconfig",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(
			tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, resolvePath(tt.args.filepath), tt.want)
			},
		)
	}
}
