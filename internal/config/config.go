package config

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
)

type Config struct {
	HetznerToken                      string             `mapstructure:"hetzner_token" validate:"required"`
	GithubToken                       string             `mapstructure:"github_token" validate:"-"`
	ClusterName                       string             `mapstructure:"cluster_name" validate:"required,dns_rfc1035_label"`
	KubeconfigPath                    string             `mapstructure:"kubeconfig_path" validate:"required"`
	K3sVersion                        string             `mapstructure:"k3s_version" validate:"required,semver"`
	PublicSSHKeyPath                  string             `mapstructure:"public_ssh_key_path" validate:"required"`
	PrivateSSHKeyPath                 string             `mapstructure:"private_ssh_key_path" validate:"required"`
	SSHAllowedNetworks                []string           `mapstructure:"ssh_allowed_networks" validate:"omitempty,dive,required,cidr"`
	APIAllowedNetworks                []string           `mapstructure:"api_allowed_networks" validate:"omitempty,dive,cidr"`
	VerifyHostKey                     bool               `mapstructure:"verify_host_key" validate:"-"`
	Location                          string             `mapstructure:"location" validate:"required,oneof=nbg1 fsn1 hel1 ash hil"`
	ScheduleWorkloadsOnMasters        bool               `mapstructure:"schedule_workloads_on_masters" validate:"-"`
	Masters                           MasterConfig       `mapstructure:"masters" validate:"required,dive"`
	WorkerNodePools                   []WorkerConfig     `mapstructure:"worker_node_pools" validate:"dive"`
	AutoscalingNodePools              []AutoscalerConfig `mapstructure:"autoscaling_node_pools" validate:"dive"`
	ClusterAutoscalerArgs             []string           `mapstructure:"cluster_autoscaler_args" validate:"-"`
	ClusterAutoscalerVersion          string             `mapstructure:"cluster_autoscaler_version" validate:"-"`
	AdditionalPackages                []string           `mapstructure:"additional_packages" validate:"-"`
	PostCreateCommands                []string           `mapstructure:"post_create_commands" validate:"-"`
	DefaultNameservers                []string           `mapstructure:"default_nameservers" validate:"-"`
	EnableEncryption                  bool               `mapstructure:"enable_encryption" validate:"-"`
	KubeAPIServerArgs                 []string           `mapstructure:"kube_api_server_args" validate:"-"`
	KubeSchedulerArgs                 []string           `mapstructure:"kube_scheduler_args" validate:"-"`
	KubeControllerManagerArgs         []string           `mapstructure:"kube_controller_manager_args" validate:"-"`
	KubeCloudControllerManagerArgs    []string           `mapstructure:"kube_cloud_controller_manager_args" validate:"-"`
	KubeletArgs                       []string           `mapstructure:"kubelet_args" validate:"-"`
	KubeProxyArgs                     []string           `mapstructure:"kube_proxy_args" validate:"-"`
	ExistingNetwork                   string             `mapstructure:"existing_network" validate:"-"`
	NetworkIPRange                    string             `mapstructure:"network_ip_range" validate:"omitempty,cidr"`
	HCloudVolumeIsDefaultStorageClass bool               `mapstructure:"hcloud_volume_is_default_storage_class" validate:"-"`
	FixMultipath                      bool               `mapstructure:"fix_multipath" validate:"-"`
	ScheduleCSIControllerOnMaster     bool               `mapstructure:"schedule_csi_controller_on_master" validate:"-"`
	Image                             string             `mapstructure:"image" validate:"-"`
}

type MasterConfig struct {
	InstanceType  string `mapstructure:"instance_type" validate:"required"`
	InstanceCount int    `mapstructure:"instance_count" validate:"gt=0,required"`
}

type WorkerConfig struct {
	Name          string `mapstructure:"name" validate:"required"`
	InstanceType  string `mapstructure:"instance_type" validate:"required"`
	InstanceCount int    `mapstructure:"instance_count" validate:"gt=0,required"`
	Location      string `mapstructure:"location" validate:"omitempty,oneof=nbg1 fsn1 hel1 ash hil"`
}

type AutoscalerConfig struct {
	Name         string `mapstructure:"name" validate:"required"`
	InstanceType string `mapstructure:"instance_type" validate:"required"`
	InstanceMin  int    `mapstructure:"instance_min" validate:"omitempty,gte=0,required"`
	InstanceMax  int    `mapstructure:"instance_max" validate:"gt=0,gtefield=InstanceMin,required"`
	Location     string `mapstructure:"location" validate:"omitempty,oneof=nbg1 fsn1 hel1 ash hil"`
}

func InitViper() {
	if viper.GetString("config_file") == "" {
		viper.AddConfigPath("/etc/hetzner-k3s")
		viper.AddConfigPath(".")
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	} else {
		viper.AddConfigPath(filepath.Dir(viper.GetString("config_file")))
		viper.SetConfigName(filepath.Base(viper.GetString("config_file")))
		viper.SetConfigType("yaml")
	}

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()
	_ = viper.ReadInConfig()
	viper.SetTypeByDefaultValue(true)

	viper.SetConfigName(".env")
	viper.SetConfigType("dotenv")
	_ = viper.MergeInConfig()

	// Set Defaults
	viper.SetDefault("debug", false)
	viper.SetDefault("hetzner_token", "")
	viper.SetDefault("public_ssh_key_path", "~/.ssh/id_rsa.pub")
	viper.SetDefault("private_ssh_key_path", "~/.ssh/id_rsa")
	viper.SetDefault("image", "ubuntu-20.04")
	viper.SetDefault("default_nameservers", []string{"1.1.1.1", "0.0.0.0"})
	viper.SetDefault("hcloud_volume_is_default_storage_class", true)
	viper.SetDefault("fix_multipath", false)
	viper.SetDefault("network_ip_range", "10.0.0.0/16")
	viper.SetDefault("api_allowed_networks", []string{"0.0.0.0/0"})
	viper.SetDefault("ssh_allowed_networks", []string{"0.0.0.0/0"})
	viper.SetDefault("schedule_csi_controller_on_master", false)

	// Bind on Environment Variables
	_ = viper.BindEnv("hetzner_token")
}

func InitConfig() *Config {
	var cfg Config

	err := viper.Unmarshal(&cfg)
	if err != nil {
		fmt.Printf("cannot unmarshal config: %s", err)
		os.Exit(1)
	}

	err = cfg.Validate()
	if err != nil {
		fmt.Printf("invalid configuration: %s", err)
		os.Exit(1)
	}

	return &cfg
}

func (c *Config) Validate() error {
	v := validator.New()

	_ = v.RegisterValidation("semver", validateVersion)
	_ = v.RegisterValidation("cidr", validateCIDR)

	err := v.Struct(c)
	if err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	return nil
}

// validateVersion overrides the original semver validation to support v1.x.x format instead of 1.x.x format.
func validateVersion(fl validator.FieldLevel) bool {
	semverRegexString := `^v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`
	semverRegex := regexp.MustCompile(semverRegexString)
	semverString := fl.Field().String()

	return semverRegex.MatchString(semverString)
}

// isCIDR is the validation function for validating if the field's value is a valid v4 or v6 CIDR address.
func validateCIDR(fl validator.FieldLevel) bool {
	val := fl.Field().String()
	if val == "0.0.0.0/0" {
		return true
	}

	_, _, err := net.ParseCIDR(fl.Field().String())

	return err == nil
}
