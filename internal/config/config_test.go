package config

import "testing"

// nolint: funlen
func TestConfig_Validate(t *testing.T) {
	t.Parallel()

	type fields struct {
		HetznerToken               string
		ClusterName                string
		KubeconfigPath             string
		K3sVersion                 string
		PublicSSHKeyPath           string
		PrivateSSHKeyPath          string
		SSHAllowedNetworks         []string
		VerifyHostKey              bool
		Location                   string
		ScheduleWorkloadsOnMasters bool
		Masters                    MasterConfig
		WorkerNodePools            []WorkerConfig
		AdditionalPackages         []string
		PostCreateCommands         []string
		EnableEncryption           bool
		KubeAPIServerArgs          []string
		KubeSchedulerArgs          []string
		KubeControllerManagerArgs  []string
		KubeletArgs                []string
		KubeProxyArgs              []string
		ExistingNetwork            string
	}

	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name:    "invalid",
			fields:  fields{},
			wantErr: true,
		},
		{
			name: "invalid location",
			fields: fields{
				HetznerToken:       "test",
				ClusterName:        "test",
				KubeconfigPath:     "test",
				K3sVersion:         "v1.24.3+k3s1",
				PublicSSHKeyPath:   "test",
				PrivateSSHKeyPath:  "test",
				SSHAllowedNetworks: []string{"192.168.0.0/24"},
				Location:           "test1",
				Masters: MasterConfig{
					InstanceType:  "test",
					InstanceCount: 1,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid version",
			fields: fields{
				HetznerToken:       "test",
				ClusterName:        "test",
				KubeconfigPath:     "test",
				K3sVersion:         "v1.24+k3s1",
				PublicSSHKeyPath:   "test",
				PrivateSSHKeyPath:  "test",
				SSHAllowedNetworks: []string{"192.168.0.0/24"},
				Location:           "nbg1",
				Masters: MasterConfig{
					InstanceType:  "test",
					InstanceCount: 1,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid cidr",
			fields: fields{
				HetznerToken:       "test",
				ClusterName:        "test",
				KubeconfigPath:     "test",
				K3sVersion:         "v1.24.3+k3s1",
				PublicSSHKeyPath:   "test",
				PrivateSSHKeyPath:  "test",
				SSHAllowedNetworks: []string{"300.0.0.0/24"},
				Location:           "nbg1",
				Masters: MasterConfig{
					InstanceType:  "test",
					InstanceCount: 1,
				},
			},
			wantErr: true,
		},
		{
			name: "valid minimal",
			fields: fields{
				HetznerToken:       "test",
				ClusterName:        "test",
				KubeconfigPath:     "test",
				K3sVersion:         "v1.24.3+k3s1",
				PublicSSHKeyPath:   "test",
				PrivateSSHKeyPath:  "test",
				SSHAllowedNetworks: []string{"192.168.0.0/24"},
				Location:           "nbg1",
				Masters: MasterConfig{
					InstanceType:  "test",
					InstanceCount: 1,
				},
			},
			wantErr: false,
		},
		{
			name: "valid",
			fields: fields{
				HetznerToken:               "test",
				ClusterName:                "test",
				KubeconfigPath:             "test",
				K3sVersion:                 "v1.24.3+k3s1",
				PublicSSHKeyPath:           "test",
				PrivateSSHKeyPath:          "test",
				SSHAllowedNetworks:         []string{"0.0.0.0/0"},
				VerifyHostKey:              false,
				Location:                   "nbg1",
				ScheduleWorkloadsOnMasters: false,
				Masters: MasterConfig{
					InstanceType:  "test",
					InstanceCount: 1,
				},
				WorkerNodePools:           nil,
				AdditionalPackages:        nil,
				PostCreateCommands:        nil,
				EnableEncryption:          false,
				KubeAPIServerArgs:         nil,
				KubeSchedulerArgs:         nil,
				KubeControllerManagerArgs: nil,
				KubeletArgs:               nil,
				KubeProxyArgs:             nil,
				ExistingNetwork:           "",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		tt := tt // nolint: varnamelen
		t.Run(
			tt.name, func(t *testing.T) {
				t.Parallel()

				conf := &Config{
					HetznerToken:               tt.fields.HetznerToken,
					ClusterName:                tt.fields.ClusterName,
					KubeconfigPath:             tt.fields.KubeconfigPath,
					K3sVersion:                 tt.fields.K3sVersion,
					PublicSSHKeyPath:           tt.fields.PublicSSHKeyPath,
					PrivateSSHKeyPath:          tt.fields.PrivateSSHKeyPath,
					SSHAllowedNetworks:         tt.fields.SSHAllowedNetworks,
					VerifyHostKey:              tt.fields.VerifyHostKey,
					Location:                   tt.fields.Location,
					ScheduleWorkloadsOnMasters: tt.fields.ScheduleWorkloadsOnMasters,
					Masters:                    tt.fields.Masters,
					WorkerNodePools:            tt.fields.WorkerNodePools,
					AdditionalPackages:         tt.fields.AdditionalPackages,
					PostCreateCommands:         tt.fields.PostCreateCommands,
					EnableEncryption:           tt.fields.EnableEncryption,
					KubeAPIServerArgs:          tt.fields.KubeAPIServerArgs,
					KubeSchedulerArgs:          tt.fields.KubeSchedulerArgs,
					KubeControllerManagerArgs:  tt.fields.KubeControllerManagerArgs,
					KubeletArgs:                tt.fields.KubeletArgs,
					KubeProxyArgs:              tt.fields.KubeProxyArgs,
					ExistingNetwork:            tt.fields.ExistingNetwork,
				}
				if err := conf.Validate(); (err != nil) != tt.wantErr {
					t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				}
			},
		)
	}
}
